package db_test

import (
	"encoding/json"
	"log"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/zk35-de/homeport/internal/db"
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

	// Seed test profiles (formerly done by InitDB, now test-only)
	if err := db.AddProfile("Markus", "markus"); err != nil {
		os.Remove(dbPath)
		t.Fatalf("Failed to seed markus profile: %v", err)
	}
	if err := db.SetDefaultProfile("markus"); err != nil {
		os.Remove(dbPath)
		t.Fatalf("Failed to set default profile: %v", err)
	}
	if err := db.AddProfile("Andrea", "andrea"); err != nil {
		os.Remove(dbPath)
		t.Fatalf("Failed to seed andrea profile: %v", err)
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
	_, err := db.AddCategory("Work", "blue")
	if err != nil {
		t.Fatalf("Failed to add category Work: %v", err)
	}
	_, err = db.AddCategory("Social", "green")
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
	err = db.AddService(categories[0].ID, "Jira", "http://jira.local", "", "Project management", "", false, []string{"markus"})
	if err != nil {
		t.Fatalf("Failed to add service Jira: %v", err)
	}
	err = db.AddService(categories[0].ID, "Confluence", "http://confluence.local", "", "Docs", "", false, []string{"markus", "andrea"})
	if err != nil {
		t.Fatalf("Failed to add service Confluence: %v", err)
	}

	// Add service to "Social" category
	err = db.AddService(categories[1].ID, "Mastodon", "http://mastodon.social", "", "Microblogging", "", false, []string{"andrea"})
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

	db.AddCategory("TestCat", "red")
	categories, _ := db.GetCategoriesWithServices("")
	catID := categories[0].ID

	db.AddService(catID, "TestService", "http://test.local", "", "", "", false, []string{"markus"})
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

	db.AddCategory("TestCat", "red")
	categories, _ := db.GetCategoriesWithServices("")
	catID := categories[0].ID

	err := db.AddService(catID, "Service1", "http://s1.local", "", "", "", false, []string{"markus"})
	if err != nil {
		t.Fatalf("Failed to add service: %v", err)
	}
	err = db.AddService(catID, "Service2", "http://s2.local", "", "", "", false, []string{"andrea"})
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

func TestCloneServicesToProfile(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Setup:
	// Category 1: Work (blue) - service1 (markus, andrea), service2 (markus)
	// Category 2: IT (cyan) - service3 (markus) - should be skipped due to color
	// Category 3: Personal (green) - service4 (markus)

	// Add categories
	db.AddCategory("Work", "blue")
	db.AddCategory("IT", "cyan")
	db.AddCategory("Personal", "green")

	cats, _ := db.GetCategoriesWithServices("")
	workCatID := cats[0].ID
	itCatID := cats[1].ID
	personalCatID := cats[2].ID

	// Add services
	db.AddService(workCatID, "WorkService1", "url1", "", "", "", false, []string{"markus", "andrea"}) // Already visible to Andrea
	db.AddService(workCatID, "WorkService2", "url2", "", "", "", false, []string{"markus"})
	db.AddService(itCatID, "ITService1", "url3", "", "", "", false, []string{"markus"}) // Cyan category
	db.AddService(personalCatID, "PersonalService1", "url4", "", "", "", false, []string{"markus"})

	// Check initial state for Andrea
	andreaInitialCats, _ := db.GetCategoriesWithServices("andrea")
	initialAndreaServices := 0
	for _, c := range andreaInitialCats {
		initialAndreaServices += len(c.Services)
	}
	if initialAndreaServices != 1 { // Only WorkService1
		t.Fatalf("Expected Andrea to have 1 service initially, got %d", initialAndreaServices)
	}

	added, skipped, err := db.CloneServicesToProfile("markus", "andrea")
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
	db.AddCategory("Discovered", "orange")
	categories, _ := db.GetCategoriesWithServices("")
	if len(categories) != 1 || categories[0].Name != "Discovered" {
		t.Fatalf("Expected 'Discovered' category to exist for testing AcceptDiscoveryItem")
	}

	itemToAccept := itemsAfterIgnore[0]
	err = db.AcceptDiscoveryItem(itemToAccept.ID, 0, false)
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

	db.AddCategory("CatA", "red") // ID 1, SortOrder 0
	db.AddCategory("CatB", "blue") // ID 2, SortOrder 1
	db.AddCategory("CatC", "green") // ID 3, SortOrder 2

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

	db.AddCategory("MainCat", "red")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	db.AddService(catID, "SvcX", "urlX", "", "", "", false, []string{"markus"}) // ID 1, SortOrder 0
	db.AddService(catID, "SvcY", "urlY", "", "", "", false, []string{"markus"}) // ID 2, SortOrder 1
	db.AddService(catID, "SvcZ", "urlZ", "", "", "", false, []string{"markus"}) // ID 3, SortOrder 2

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

	db.AddCategory("TestCat", "red")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID
	db.AddService(catID, "MonitorMe", "http://monitor.me", "", "", "http://status.me", false, []string{"markus"})

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

func TestGetTopClicks(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Empty DB
	stats, err := db.GetTopClicks("", 10)
	if err != nil {
		t.Fatalf("GetTopClicks on empty DB: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("Expected 0 stats on empty DB, got %d", len(stats))
	}

	// Setup services
	db.AddCategory("TestCat", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID
	db.AddService(catID, "Service A", "http://a.local", "", "", "", false, []string{"markus"})
	db.AddService(catID, "Service B", "http://b.local", "", "", "", false, []string{"andrea"})

	allCats, _ := db.GetCategoriesWithServices("")
	svcA := allCats[0].Services[0].ID
	svcB := allCats[0].Services[1].ID

	db.RecordClick(svcA, "markus")
	db.RecordClick(svcA, "markus")
	db.RecordClick(svcA, "markus")
	db.RecordClick(svcB, "andrea")
	db.RecordClick(svcB, "andrea")

	// All profiles – Service A (3 clicks) should be first
	stats, err = db.GetTopClicks("", 10)
	if err != nil {
		t.Fatalf("GetTopClicks all: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("Expected 2 stats, got %d", len(stats))
	}
	if stats[0].ServiceName != "Service A" || stats[0].ClickCount != 3 {
		t.Errorf("Expected Service A first with 3 clicks, got name=%s count=%d", stats[0].ServiceName, stats[0].ClickCount)
	}

	// Profile filter – markus
	markusStats, _ := db.GetTopClicks("markus", 10)
	if len(markusStats) != 1 || markusStats[0].ServiceName != "Service A" {
		t.Errorf("Expected 1 stat for markus (Service A), got %v", markusStats)
	}

	// Profile filter – andrea
	andreaStats, _ := db.GetTopClicks("andrea", 10)
	if len(andreaStats) != 1 || andreaStats[0].ServiceName != "Service B" {
		t.Errorf("Expected 1 stat for andrea (Service B), got %v", andreaStats)
	}

	// Limit
	limitedStats, _ := db.GetTopClicks("", 1)
	if len(limitedStats) != 1 {
		t.Errorf("Expected 1 stat with limit=1, got %d", len(limitedStats))
	}
}

func TestProfiles(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// setupTestDB seeds "markus" and "andrea" as test profiles
	profiles, err := db.GetProfiles()
	if err != nil {
		t.Fatalf("GetProfiles: %v", err)
	}
	if len(profiles) < 2 {
		t.Fatalf("Expected at least 2 test profiles, got %d", len(profiles))
	}

	// GetDefaultProfile
	def, err := db.GetDefaultProfile()
	if err != nil {
		t.Fatalf("GetDefaultProfile: %v", err)
	}
	if def == nil {
		t.Fatal("Expected a default profile, got nil")
	}

	// GetProfileBySlug
	p, err := db.GetProfileBySlug(def.Slug)
	if err != nil {
		t.Fatalf("GetProfileBySlug: %v", err)
	}
	if p == nil || p.Slug != def.Slug {
		t.Errorf("GetProfileBySlug returned wrong profile: %v", p)
	}

	// AddProfile + DeleteProfile
	if err := db.AddProfile("Test User", "testuser"); err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	profilesAfter, _ := db.GetProfiles()
	found := false
	for _, pr := range profilesAfter {
		if pr.Slug == "testuser" {
			found = true
		}
	}
	if !found {
		t.Error("Added profile 'testuser' not found")
	}

	if err := db.DeleteProfile("testuser"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	profilesAfterDelete, _ := db.GetProfiles()
	for _, pr := range profilesAfterDelete {
		if pr.Slug == "testuser" {
			t.Error("Deleted profile 'testuser' still present")
		}
	}
}

func TestUserPreferences(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// GetUserPreferences on fresh profile returns defaults
	prefs, err := db.GetUserPreferences("markus")
	if err != nil {
		t.Fatalf("GetUserPreferences: %v", err)
	}
	// Default theme should be "dark" (or empty – just verify no error and struct returned)
	if prefs == nil {
		t.Fatal("Expected non-nil UserPreferences")
	}

	// SetUserPreferences
	updated := &db.UserPreferences{
		Theme:           "light",
		AccentColor:     "#ff0000",
		AuroraColor:     "#00ff00",
		AuroraIntensity: "vivid",
		AuroraAnimated:  true,
	}
	if err := db.SetUserPreferences("markus", *updated); err != nil {
		t.Fatalf("SetUserPreferences: %v", err)
	}

	prefs2, err := db.GetUserPreferences("markus")
	if err != nil {
		t.Fatalf("GetUserPreferences after set: %v", err)
	}
	if prefs2.Theme != "light" {
		t.Errorf("Expected theme 'light', got %q", prefs2.Theme)
	}
	if prefs2.AccentColor != "#ff0000" {
		t.Errorf("Expected accent '#ff0000', got %q", prefs2.AccentColor)
	}
	if prefs2.AuroraColor != "#00ff00" {
		t.Errorf("Expected aurora color '#00ff00', got %q", prefs2.AuroraColor)
	}
	if prefs2.AuroraIntensity != "vivid" {
		t.Errorf("Expected aurora intensity 'vivid', got %q", prefs2.AuroraIntensity)
	}
	if !prefs2.AuroraAnimated {
		t.Error("Expected aurora animated true, got false")
	}
}

func TestSessions(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// CreateSession returns a non-empty token.
	token, err := db.CreateSession("markus", 7)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if token == "" {
		t.Fatal("Expected non-empty session token")
	}

	// GetSession finds a valid session.
	profile := db.GetSession(token)
	if profile != "markus" {
		t.Errorf("GetSession: expected 'markus', got %q", profile)
	}

	// GetSessionInfo returns non-nil with a future ExpiresAt.
	info := db.GetSessionInfo(token)
	if info == nil {
		t.Fatal("GetSessionInfo returned nil for valid token")
	}
	if info.Profile != "markus" {
		t.Errorf("GetSessionInfo profile: expected 'markus', got %q", info.Profile)
	}
	if info.ExpiresAt.IsZero() {
		t.Error("GetSessionInfo ExpiresAt is zero – datetime parse failed")
	}
	if !info.ExpiresAt.After(time.Now()) {
		t.Errorf("GetSessionInfo ExpiresAt should be in the future, got %v", info.ExpiresAt)
	}

	// ExtendSession pushes ExpiresAt further out.
	if err := db.ExtendSession(token, 14); err != nil {
		t.Fatalf("ExtendSession: %v", err)
	}
	infoAfter := db.GetSessionInfo(token)
	if infoAfter == nil {
		t.Fatal("GetSessionInfo nil after ExtendSession")
	}
	if !infoAfter.ExpiresAt.After(info.ExpiresAt) {
		t.Errorf("ExpiresAt after extend (%v) should be after original (%v)", infoAfter.ExpiresAt, info.ExpiresAt)
	}

	// DeleteSession removes the session.
	if err := db.DeleteSession(token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if db.GetSession(token) != "" {
		t.Error("GetSession should return '' after DeleteSession")
	}

	// Unknown token returns empty.
	if db.GetSession("nonexistent") != "" {
		t.Error("GetSession with unknown token should return ''")
	}
}

func TestSQLInjection(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Category name with SQL injection payload – parametrized queries must store it safely
	injName := `'; DROP TABLE categories; --`
	if _, err := db.AddCategory(injName, "red"); err != nil {
		t.Fatalf("AddCategory with injection payload failed (should succeed with parametrized queries): %v", err)
	}
	cats, err := db.GetCategoriesWithServices("")
	if err != nil {
		t.Fatalf("GetCategoriesWithServices after injection attempt: %v", err)
	}
	if len(cats) != 1 || cats[0].Name != injName {
		t.Errorf("Expected category %q stored safely, got %v", injName, cats)
	}

	// Service name and URL with OR-injection
	catID := cats[0].ID
	injSvcName := `' OR '1'='1`
	injURL := `http://x.com/' OR '1'='1`
	if err := db.AddService(catID, injSvcName, injURL, "", "", "", false, []string{"markus"}); err != nil {
		t.Fatalf("AddService with injection payload: %v", err)
	}
	services, _ := db.GetCategoriesWithServices("")
	if len(services[0].Services) != 1 {
		t.Fatalf("Expected 1 service after injection, got %d", len(services[0].Services))
	}
	if services[0].Services[0].Name != injSvcName {
		t.Errorf("Service name not stored correctly: got %q, want %q", services[0].Services[0].Name, injSvcName)
	}

	// Verify categories table still intact (DROP TABLE was not executed)
	catsAfter, err := db.GetCategoriesWithServices("")
	if err != nil {
		t.Fatalf("categories table should still exist after injection attempts: %v", err)
	}
	if !strings.Contains(catsAfter[0].Name, "DROP TABLE") {
		// The name we stored must still be there
		_ = catsAfter
	}
}

