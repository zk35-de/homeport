package db_test

import (
	"encoding/json"
	"log"
	"os"
	"sort"
	"testing"

	"git.zk35.de/secalpha/homeport/db"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// setupTestDB initializes a temporary file-based SQLite database for testing.
// Each call creates an isolated DB — critical because InitDB sets the global db.DB.
func setupTestDB(t *testing.T) func() {
	t.Helper()
	log.SetOutput(os.Stderr)

	tmpFile, err := os.CreateTemp("", "homeport-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()

	if err := db.InitDB(dbPath); err != nil {
		os.Remove(dbPath)
		t.Fatalf("Failed to initialize database schema: %v", err)
	}

	return func() {
		if db.DB != nil {
			db.DB.Close()
			db.DB = nil
		}
		os.Remove(dbPath)
	}
}

func TestInitDB(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Verify that a table exists
	row := db.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='categories'")
	var tableName string
	err := row.Scan(&tableName)
	if err != nil {
		t.Errorf("Failed to verify categories table: %v", err)
	}
	if tableName != "categories" {
		t.Errorf("Expected table 'categories' not found, got '%s'", tableName)
	}
}

func TestAddCategoryAndGetCategoriesWithServices(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Add categories
	err := db.AddCategory("Work", "tiles", "blue")
	if err != nil {
		t.Fatalf("Failed to add category Work: %v", err)
	}
	err = db.AddCategory("Social", "list", "green")
	if err != nil {
		t.Fatalf("Failed to add category Social: %v", err)
	}

	// Get categories with services (all profiles)
	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		t.Fatalf("Failed to get categories: %v", err)
	}
	if len(categories) != 2 {
		t.Fatalf("Expected 2 categories, got %d", len(categories))
	}

	if categories[0].Name != "Work" || categories[1].Name != "Social" {
		t.Errorf("Categories not in expected order or names incorrect")
	}

	// Add service to "Work" category
	err = db.AddService(categories[0].ID, "Jira", "http://jira.local", "", "Project management", "", []string{"markus"})
	if err != nil {
		t.Fatalf("Failed to add service Jira: %v", err)
	}
	err = db.AddService(categories[0].ID, "Confluence", "http://confluence.local", "", "Docs", "", []string{"markus", "andrea"})
	if err != nil {
		t.Fatalf("Failed to add service Confluence: %v", err)
	}

	// Add service to "Social" category
	err = db.AddService(categories[1].ID, "Mastodon", "http://mastodon.social", "", "Microblogging", "", []string{"andrea"})
	if err != nil {
		t.Fatalf("Failed to add service Mastodon: %v", err)
	}

	// Get for markus
	markusCategories, err := db.GetCategoriesWithServices("markus")
	if err != nil {
		t.Fatalf("Failed to get categories for markus: %v", err)
	}
	if len(markusCategories) != 2 { // Both categories should exist, even if empty
		t.Fatalf("Expected 2 categories for markus, got %d", len(markusCategories))
	}
	if len(markusCategories[0].Services) != 2 {
		t.Fatalf("Expected 2 services for Work category for markus, got %d", len(markusCategories[0].Services))
	}
	if markusCategories[0].Services[0].Name != "Jira" || markusCategories[0].Services[1].Name != "Confluence" {
		t.Errorf("Markus' Work services not as expected: %v", markusCategories[0].Services)
	}
	if len(markusCategories[1].Services) != 0 {
		t.Fatalf("Expected 0 services for Social category for markus, got %d", len(markusCategories[1].Services))
	}

	// Get for andrea
	andreaCategories, err := db.GetCategoriesWithServices("andrea")
	if err != nil {
		t.Fatalf("Failed to get categories for andrea: %v", err)
	}
	if len(andreaCategories) != 2 {
		t.Fatalf("Expected 2 categories for andrea, got %d", len(andreaCategories))
	}
	if len(andreaCategories[0].Services) != 1 {
		t.Fatalf("Expected 1 service for Work category for andrea, got %d", len(andreaCategories[0].Services))
	}
	if andreaCategories[0].Services[0].Name != "Confluence" {
		t.Errorf("Andrea's Work services not as expected: %v", andreaCategories[0].Services)
	}
	if len(andreaCategories[1].Services) != 1 {
		t.Fatalf("Expected 1 service for Social category for andrea, got %d", len(andreaCategories[1].Services))
	}
	if andreaCategories[1].Services[0].Name != "Mastodon" {
		t.Errorf("Andrea's Social services not as expected: %v", andreaCategories[1].Services)
	}

	// Test GetCategoriesWithServices with empty profile (manage mode) to check VisibleTo
	allCategories, err := db.GetCategoriesWithServices("")
	if err != nil {
		t.Fatalf("Failed to get all categories for manage mode: %v", err)
	}
	// Assuming Jira is the first service added, and Confluence second
	var jiraService db.Service
	var confluenceService db.Service
	for _, c := range allCategories {
		for _, s := range c.Services {
			if s.Name == "Jira" {
				jiraService = s
			}
			if s.Name == "Confluence" {
				confluenceService = s
			}
		}
	}

	sort.Strings(jiraService.VisibleTo)
	if len(jiraService.VisibleTo) != 1 || jiraService.VisibleTo[0] != "markus" {
		t.Errorf("Expected Jira visible to 'markus', got %v", jiraService.VisibleTo)
	}

	sort.Strings(confluenceService.VisibleTo)
	if len(confluenceService.VisibleTo) != 2 || confluenceService.VisibleTo[0] != "andrea" || confluenceService.VisibleTo[1] != "markus" {
		t.Errorf("Expected Confluence visible to 'markus' and 'andrea', got %v", confluenceService.VisibleTo)
	}
}

func TestDeleteCategory(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	db.AddCategory("TestCat", "tiles", "red")
	categories, _ := db.GetCategoriesWithServices("")
	catID := categories[0].ID

	db.AddService(catID, "TestService", "http://test.local", "", "", "", []string{"markus"})
	servicesBefore, _ := db.GetCategoriesWithServices("")
	if len(servicesBefore[0].Services) != 1 {
		t.Fatalf("Expected 1 service before delete, got %d", len(servicesBefore[0].Services))
	}

	err := db.DeleteCategory(catID)
	if err != nil {
		t.Fatalf("Failed to delete category: %v", err)
	}

	categoriesAfter, _ := db.GetCategoriesWithServices("")
	if len(categoriesAfter) != 0 {
		t.Errorf("Expected 0 categories after delete, got %d", len(categoriesAfter))
	}

	// Verify cascade delete
	rows := db.DB.QueryRow("SELECT COUNT(*) FROM services WHERE category_id = ?", catID)
	var count int
	rows.Scan(&count)
	if count != 0 {
		t.Errorf("Expected services to be cascade deleted, but found %d", count)
	}
}

func TestDeleteService(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	db.AddCategory("TestCat", "tiles", "red")
	categories, _ := db.GetCategoriesWithServices("")
	catID := categories[0].ID

	err := db.AddService(catID, "Service1", "http://s1.local", "", "", "", []string{"markus"})
	if err != nil {
		t.Fatalf("Failed to add service: %v", err)
	}
	err = db.AddService(catID, "Service2", "http://s2.local", "", "", "", []string{"andrea"})
	if err != nil {
		t.Fatalf("Failed to add service: %v", err)
	}

	allCategories, _ := db.GetCategoriesWithServices("")
	if len(allCategories[0].Services) != 2 {
		t.Fatalf("Expected 2 services initially, got %d", len(allCategories[0].Services))
	}

	serviceIDToDelete := allCategories[0].Services[0].ID
	err = db.DeleteService(serviceIDToDelete)
	if err != nil {
		t.Fatalf("Failed to delete service: %v", err)
	}

	allCategoriesAfter, _ := db.GetCategoriesWithServices("")
	if len(allCategoriesAfter[0].Services) != 1 {
		t.Errorf("Expected 1 service after delete, got %d", len(allCategoriesAfter[0].Services))
	}
	if allCategoriesAfter[0].Services[0].Name != "Service2" {
		t.Errorf("Wrong service remaining after delete, expected Service2, got %s", allCategoriesAfter[0].Services[0].Name)
	}
}

func TestAddWidgetAndGetWidgets(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	err := db.AddWidget("Work Calendar", "http://ical.work/cal.ics", "markus")
	if err != nil {
		t.Fatalf("Failed to add widget: %v", err)
	}
	err = db.AddWidget("Personal Events", "http://ical.personal/cal.ics", "andrea")
	if err != nil {
		t.Fatalf("Failed to add widget: %v", err)
	}
	err = db.AddWidget("Global Announcements", "http://ical.global/cal.ics", "all")
	if err != nil {
		t.Fatalf("Failed to add widget: %v", err)
	}

	// Get widgets for markus
	markusWidgets, err := db.GetWidgets("markus")
	if err != nil {
		t.Fatalf("Failed to get widgets for markus: %v", err)
	}
	if len(markusWidgets) != 2 {
		t.Fatalf("Expected 2 widgets for markus, got %d", len(markusWidgets))
	}
	if markusWidgets[0].Name != "Work Calendar" && markusWidgets[1].Name != "Work Calendar" {
		t.Errorf("Markus widgets missing 'Work Calendar'")
	}
	if markusWidgets[0].Name != "Global Announcements" && markusWidgets[1].Name != "Global Announcements" {
		t.Errorf("Markus widgets missing 'Global Announcements'")
	}

	// Get widgets for andrea
	andreaWidgets, err := db.GetWidgets("andrea")
	if err != nil {
		t.Fatalf("Failed to get widgets for andrea: %v", err)
	}
	if len(andreaWidgets) != 2 {
		t.Fatalf("Expected 2 widgets for andrea, got %d", len(andreaWidgets))
	}
	if andreaWidgets[0].Name != "Personal Events" && andreaWidgets[1].Name != "Personal Events" {
		t.Errorf("Andrea widgets missing 'Personal Events'")
	}
	if andreaWidgets[0].Name != "Global Announcements" && andreaWidgets[1].Name != "Global Announcements" {
		t.Errorf("Andrea widgets missing 'Global Announcements'")
	}

	// Get all widgets
	allWidgets, err := db.GetAllWidgets()
	if err != nil {
		t.Fatalf("Failed to get all widgets: %v", err)
	}
	if len(allWidgets) != 3 {
		t.Fatalf("Expected 3 widgets in total, got %d", len(allWidgets))
	}
}

func TestDeleteWidget(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	err := db.AddWidget("Test Widget", "http://ical.test", "all")
	if err != nil {
		t.Fatalf("Failed to add widget: %v", err)
	}
	widgets, _ := db.GetAllWidgets()
	if len(widgets) != 1 {
		t.Fatalf("Expected 1 widget initially, got %d", len(widgets))
	}

	err = db.DeleteWidget(widgets[0].ID)
	if err != nil {
		t.Fatalf("Failed to delete widget: %v", err)
	}

	widgetsAfter, _ := db.GetAllWidgets()
	if len(widgetsAfter) != 0 {
		t.Errorf("Expected 0 widgets after delete, got %d", len(widgetsAfter))
	}
}

func TestSearchEngineFunctions(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Test default GetSearchEngine
	if db.GetSearchEngine("markus") != "https://duckduckgo.com/" {
		t.Errorf("Expected default search engine for markus to be DuckDuckGo")
	}

	// Set custom search engine
	err := db.SetSearchEngine("markus", "https://google.com/search?q=")
	if err != nil {
		t.Fatalf("Failed to set search engine for markus: %v", err)
	}
	err = db.SetSearchEngine("andrea", "https://bing.com/search?q=")
	if err != nil {
		t.Fatalf("Failed to set search engine for andrea: %v", err)
	}

	// Test GetSearchEngine for custom values
	if db.GetSearchEngine("markus") != "https://google.com/search?q=" {
		t.Errorf("Expected search engine for markus to be Google, got %s", db.GetSearchEngine("markus"))
	}
	if db.GetSearchEngine("andrea") != "https://bing.com/search?q=" {
		t.Errorf("Expected search engine for andrea to be Bing, got %s", db.GetSearchEngine("andrea"))
	}

	// Test GetAllSearchEngines
	allEngines := db.GetAllSearchEngines()
	if len(allEngines) != 2 {
		t.Fatalf("Expected 2 search engines, got %d", len(allEngines))
	}
	if allEngines["markus"] != "https://google.com/search?q=" {
		t.Errorf("Expected markus search engine to be Google, got %s", allEngines["markus"])
	}
	if allEngines["andrea"] != "https://bing.com/search?q=" {
		t.Errorf("Expected andrea search engine to be Bing, got %s", allEngines["andrea"])
	}

	// Test updating an existing search engine
	err = db.SetSearchEngine("markus", "https://newgoogle.com/search?q=")
	if err != nil {
		t.Fatalf("Failed to update search engine for markus: %v", err)
	}
	if db.GetSearchEngine("markus") != "https://newgoogle.com/search?q=" {
		t.Errorf("Expected updated search engine for markus, got %s", db.GetSearchEngine("markus"))
	}
}

func TestCloneToAndrea(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Setup:
	// Category 1: Work (blue) - service1 (markus, andrea), service2 (markus)
	// Category 2: IT (cyan) - service3 (markus) - should be skipped due to color
	// Category 3: Personal (green) - service4 (markus)

	// Add categories
	db.AddCategory("Work", "tiles", "blue")
	db.AddCategory("IT", "tiles", "cyan")
	db.AddCategory("Personal", "tiles", "green")

	cats, _ := db.GetCategoriesWithServices("")
	workCatID := cats[0].ID
	itCatID := cats[1].ID
	personalCatID := cats[2].ID

	// Add services
	db.AddService(workCatID, "WorkService1", "url1", "", "", "", []string{"markus", "andrea"}) // Already visible to Andrea
	db.AddService(workCatID, "WorkService2", "url2", "", "", "", []string{"markus"})
	db.AddService(itCatID, "ITService1", "url3", "", "", "", []string{"markus"}) // Cyan category
	db.AddService(personalCatID, "PersonalService1", "url4", "", "", "", []string{"markus"})

	// Check initial state for Andrea
	andreaInitialCats, _ := db.GetCategoriesWithServices("andrea")
	initialAndreaServices := 0
	for _, c := range andreaInitialCats {
		initialAndreaServices += len(c.Services)
	}
	if initialAndreaServices != 1 { // Only WorkService1
		t.Fatalf("Expected Andrea to have 1 service initially, got %d", initialAndreaServices)
	}

	added, skipped, err := db.CloneToAndrea()
	if err != nil {
		t.Fatalf("Error cloning to Andrea: %v", err)
	}

	if added != 2 { // WorkService2, PersonalService1
		t.Errorf("Expected 2 services added, got %d", added)
	}
	if skipped != 1 { // WorkService1 (already exists)
		t.Errorf("Expected 1 service skipped, got %d", skipped)
	}

	// Check final state for Andrea
	andreaFinalCats, _ := db.GetCategoriesWithServices("andrea")
	finalAndreaServices := 0
	var finalServiceNames []string
	for _, c := range andreaFinalCats {
		for _, s := range c.Services {
			finalAndreaServices++
			finalServiceNames = append(finalServiceNames, s.Name)
		}
	}

	if finalAndreaServices != 3 { // WorkService1, WorkService2, PersonalService1
		t.Fatalf("Expected Andrea to have 3 services finally, got %d", finalAndreaServices)
	}
	sort.Strings(finalServiceNames)
	expectedServiceNames := []string{"PersonalService1", "WorkService1", "WorkService2"}
	sort.Strings(expectedServiceNames)
	for i, name := range finalServiceNames {
		if name != expectedServiceNames[i] {
			t.Errorf("Andrea's final services mismatch. Expected %v, got %v", expectedServiceNames, finalServiceNames)
			break
		}
	}
}

func TestDiscoveryInbox(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Add items
	suggested1 := db.SuggestedService{Name: "Discovered1", URL: "http://d1.local", Category: "Discovered"}
	suggested1JSON, _ := json.Marshal(suggested1)
	db.AddDiscoveryItem("container1", string(suggested1JSON))

	suggested2 := db.SuggestedService{Name: "Discovered2", URL: "http://d2.local", Category: "Discovered"}
	suggested2JSON, _ := json.Marshal(suggested2)
	db.AddDiscoveryItem("container2", string(suggested2JSON))

	items, err := db.GetDiscoveryInbox()
	if err != nil {
		t.Fatalf("Failed to get discovery inbox: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("Expected 2 discovery items, got %d", len(items))
	}
	// Collect names (order may vary when timestamps are identical)
	names := map[string]bool{}
	for _, item := range items {
		names[item.Suggested.Name] = true
	}
	if !names["Discovered1"] || !names["Discovered2"] {
		t.Errorf("Discovery items missing expected entries: %v", items)
	}

	// Ignore the first item
	ignoreID := items[0].ID
	err = db.IgnoreDiscoveryItem(ignoreID)
	if err != nil {
		t.Fatalf("Failed to ignore discovery item: %v", err)
	}
	itemsAfterIgnore, err := db.GetDiscoveryInbox()
	if err != nil {
		t.Fatalf("Failed to get discovery inbox after ignore: %v", err)
	}
	if len(itemsAfterIgnore) != 1 {
		t.Fatalf("Expected 1 discovery item after ignore, got %d", len(itemsAfterIgnore))
	}
	if itemsAfterIgnore[0].ID == ignoreID {
		t.Errorf("Ignored item still in inbox")
	}

	// Accept an item
	// Need to ensure category exists for AcceptDiscoveryItem to work, or it will create it
	db.AddCategory("Discovered", "tiles", "orange")
	categories, _ := db.GetCategoriesWithServices("")
	if len(categories) != 1 || categories[0].Name != "Discovered" {
		t.Fatalf("Expected 'Discovered' category to exist for testing AcceptDiscoveryItem")
	}

	itemToAccept := itemsAfterIgnore[0]
	err = db.AcceptDiscoveryItem(itemToAccept.ID)
	if err != nil {
		t.Fatalf("Failed to accept discovery item: %v", err)
	}

	itemsAfterAccept, err := db.GetDiscoveryInbox()
	if err != nil {
		t.Fatalf("Failed to get discovery inbox after accept: %v", err)
	}
	if len(itemsAfterAccept) != 0 {
		t.Fatalf("Expected 0 discovery items after accept, got %d", len(itemsAfterAccept))
	}

	// Verify service was added
	catsAfterAccept, _ := db.GetCategoriesWithServices("markus") // Services added to markus and andrea by default
	foundService := false
	for _, c := range catsAfterAccept {
		for _, s := range c.Services {
			if s.Name == itemToAccept.Suggested.Name {
				foundService = true
				break
			}
		}
		if foundService {
			break
		}
	}
	if !foundService {
		t.Errorf("Accepted discovery item did not result in a new service being added")
	}
}

func TestUpdateCategorySort(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	db.AddCategory("CatA", "tiles", "red") // ID 1, SortOrder 0
	db.AddCategory("CatB", "tiles", "blue") // ID 2, SortOrder 1
	db.AddCategory("CatC", "tiles", "green") // ID 3, SortOrder 2

	categories, _ := db.GetCategoriesWithServices("")
	if len(categories) != 3 {
		t.Fatalf("Expected 3 categories, got %d", len(categories))
	}
	if categories[0].Name != "CatA" || categories[1].Name != "CatB" || categories[2].Name != "CatC" {
		t.Errorf("Initial category order incorrect")
	}

	// Swap CatB (ID 2, SO 1) with CatA (ID 1, SO 0) -> CatB SO 0, CatA SO 1
	// Update CatB to new order 0
	catBID := categories[1].ID
	catAID := categories[0].ID
	err := db.UpdateCategorySort(catBID, categories[0].SortOrder)
	if err != nil {
		t.Fatalf("Failed to update sort for CatB: %v", err)
	}
	// Update CatA to new order 1
	err = db.UpdateCategorySort(catAID, categories[1].SortOrder)
	if err != nil {
		t.Fatalf("Failed to update sort for CatA: %v", err)
	}

	categoriesAfterSort, _ := db.GetCategoriesWithServices("")
	if categoriesAfterSort[0].Name != "CatB" || categoriesAfterSort[1].Name != "CatA" || categoriesAfterSort[2].Name != "CatC" {
		t.Errorf("Category order after sort incorrect: %v", categoriesAfterSort)
	}
}

func TestUpdateServiceSort(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	db.AddCategory("MainCat", "tiles", "red")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	db.AddService(catID, "SvcX", "urlX", "", "", "", []string{"markus"}) // ID 1, SortOrder 0
	db.AddService(catID, "SvcY", "urlY", "", "", "", []string{"markus"}) // ID 2, SortOrder 1
	db.AddService(catID, "SvcZ", "urlZ", "", "", "", []string{"markus"}) // ID 3, SortOrder 2

	categories, _ := db.GetCategoriesWithServices("")
	if len(categories[0].Services) != 3 {
		t.Fatalf("Expected 3 services, got %d", len(categories[0].Services))
	}
	if categories[0].Services[0].Name != "SvcX" || categories[0].Services[1].Name != "SvcY" || categories[0].Services[2].Name != "SvcZ" {
		t.Errorf("Initial service order incorrect")
	}

	// Swap SvcY (ID 2, SO 1) with SvcX (ID 1, SO 0) -> SvcY SO 0, SvcX SO 1
	svcYID := categories[0].Services[1].ID
	svcXID := categories[0].Services[0].ID
	err := db.UpdateServiceSort(svcYID, categories[0].Services[0].SortOrder)
	if err != nil {
		t.Fatalf("Failed to update sort for SvcY: %v", err)
	}
	err = db.UpdateServiceSort(svcXID, categories[0].Services[1].SortOrder)
	if err != nil {
		t.Fatalf("Failed to update sort for SvcX: %v", err)
	}

	categoriesAfterSort, _ := db.GetCategoriesWithServices("")
	if categoriesAfterSort[0].Services[0].Name != "SvcY" || categoriesAfterSort[0].Services[1].Name != "SvcX" || categoriesAfterSort[0].Services[2].Name != "SvcZ" {
		t.Errorf("Service order after sort incorrect: %v", categoriesAfterSort[0].Services)
	}
}

func TestUpdateServiceStatus(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	db.AddCategory("TestCat", "tiles", "red")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID
	db.AddService(catID, "MonitorMe", "http://monitor.me", "", "", "http://status.me", []string{"markus"})

	services, _ := db.GetCategoriesWithServices("markus")
	svcID := services[0].Services[0].ID

	// Initial status should be 0/false (alive, last_check)
	if services[0].Services[0].Alive || services[0].Services[0].LastCheck != "0001-01-01 00:00:00" {
		t.Errorf("Initial service status incorrect: Alive=%t, LastCheck=%s", services[0].Services[0].Alive, services[0].Services[0].LastCheck)
	}

	// Update to alive=true
	err := db.UpdateServiceStatus(svcID, true)
	if err != nil {
		t.Fatalf("Failed to update service status to true: %v", err)
	}

	servicesAfterUpdate1, _ := db.GetCategoriesWithServices("markus")
	if !servicesAfterUpdate1[0].Services[0].Alive {
		t.Errorf("Service status not updated to true")
	}
	if servicesAfterUpdate1[0].Services[0].LastCheck == "0001-01-01 00:00:00" || servicesAfterUpdate1[0].Services[0].LastCheck == "" {
		t.Errorf("LastCheck was not updated after first status update: %s", servicesAfterUpdate1[0].Services[0].LastCheck)
	}

	// Update to alive=false
	err = db.UpdateServiceStatus(svcID, false)
	if err != nil {
		t.Fatalf("Failed to update service status to false: %v", err)
	}

	servicesAfterUpdate2, _ := db.GetCategoriesWithServices("markus")
	if servicesAfterUpdate2[0].Services[0].Alive {
		t.Errorf("Service status not updated to false")
	}
	// Note: we don't compare LastCheck timestamps between the two updates because
	// SQLite datetime('now') has second precision and both updates may land in the same second.

	// Test GetAllServicesWithStatusCheck
	servicesWithChecks, err := db.GetAllServicesWithStatusCheck()
	if err != nil {
		t.Fatalf("Failed to get services with status check: %v", err)
	}
	if len(servicesWithChecks) != 1 {
		t.Fatalf("Expected 1 service with status check, got %d", len(servicesWithChecks))
	}
	if servicesWithChecks[0].ID != svcID || servicesWithChecks[0].StatusCheck != "http://status.me" {
		t.Errorf("GetAllServicesWithStatusCheck returned incorrect service: %v", servicesWithChecks[0])
	}
}

func TestWidgetCache(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	db.AddWidget("TestCal", "http://ical.test/cal", "markus")
	widgets, _ := db.GetWidgets("markus")
	widgetID := widgets[0].ID

	events := []db.ICalEvent{
		{Title: "Event1", Start: "2026-01-01", End: "2026-01-01", IsToday: true},
		{Title: "Event2", Start: "2026-01-02", End: "2026-01-02", IsTomorrow: true},
	}
	cacheEntry := db.WidgetCacheEntry{Events: events}
	cacheData, _ := json.Marshal(cacheEntry)

	// Update cache
	err := db.UpdateWidgetCache(widgetID, string(cacheData))
	if err != nil {
		t.Fatalf("Failed to update widget cache: %v", err)
	}

	// Get cache
	retrievedCache, err := db.GetWidgetCache(widgetID)
	if err != nil {
		t.Fatalf("Failed to get widget cache: %v", err)
	}
	if retrievedCache == nil {
		t.Fatal("Retrieved cache is nil")
	}
	if len(retrievedCache.Events) != 2 {
		t.Fatalf("Expected 2 events in cache, got %d", len(retrievedCache.Events))
	}
	if retrievedCache.Events[0].Title != "Event1" || retrievedCache.Events[1].Title != "Event2" {
		t.Errorf("Events in cache mismatch: %v", retrievedCache.Events)
	}

	// Test non-existent widget cache
	nonExistentCache, err := db.GetWidgetCache(999)
	if err != nil {
		t.Fatalf("Error getting non-existent widget cache: %v", err)
	}
	if nonExistentCache != nil {
		t.Errorf("Expected nil for non-existent widget cache, got %v", nonExistentCache)
	}
}
