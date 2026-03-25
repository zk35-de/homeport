package api_test

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"git.zk35.de/secalpha/homeport/internal/db"
	"git.zk35.de/secalpha/homeport/internal/api"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// Minimal embedded file system for templates
//go:embed testdata/*
var testTemplateFS embed.FS

// To allow templates to find partials, we use this structure:
// testdata/base.html
// testdata/index.html
// testdata/manage.html
// testdata/partials/category_list.html
// testdata/partials/discovery_inbox.html
// testdata/partials/widget_ical.html (optional if not directly testing widgets)

// content for testdata/base.html
// {{define "base.html"}}<p>Base: {{template "content" .}}</p>{{end}}

// content for testdata/index.html
// {{define "content"}}Index: {{.Profile}} {{range .Categories}}{{.Name}} {{end}}{{end}}

// content for testdata/manage.html
// {{define "content"}}Manage: {{range .Categories}}{{.Name}} {{end}}{{template "category_list" .}}{{template "discovery_inbox" .}}{{end}}

// content for testdata/partials/category_list.html
// {{define "category_list"}}<div id="category-list">{{range .Categories}}{{.Name}}:{{len .Services}}@{{.Color}}|{{end}}</div>{{end}}

// content for testdata/partials/discovery_inbox.html
// {{define "discovery_inbox"}}<div id="discovery-inbox">{{range .Items}}{{.Suggested.Name}}|{{end}}</div>{{end}}

func setupTest(t *testing.T) func() {
	t.Helper()
	log.SetOutput(io.Discard)

	// Use temp file DB — InitDB opens a file by path and sets the global db.DB.
	tmpFile, err := os.CreateTemp("", "homeport-handler-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()

	if err := db.InitDB(dbPath); err != nil {
		os.Remove(dbPath)
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Parse test templates directly — InitTemplates hardcodes "templates/*" paths
	// which don't match our testdata/ structure.
	indexTmpl, err := template.ParseFS(testTemplateFS,
		"testdata/base.html",
		"testdata/index.html",
		"testdata/partials/*.html",
	)
	if err != nil {
		os.Remove(dbPath)
		t.Fatalf("Failed to parse index test templates: %v", err)
	}
	manageTmpl, err := template.ParseFS(testTemplateFS,
		"testdata/base.html",
		"testdata/manage.html",
		"testdata/partials/*.html",
	)
	if err != nil {
		os.Remove(dbPath)
		t.Fatalf("Failed to parse manage test templates: %v", err)
	}
	api.IndexTmpl = indexTmpl
	api.ManageTmpl = manageTmpl

	return func() {
		if db.DB != nil {
			db.DB.Close()
			db.DB = nil
		}
		os.Remove(dbPath)
		api.IndexTmpl = nil
		api.ManageTmpl = nil
		log.SetOutput(os.Stderr)
	}
}

func TestHandleIndex(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Add some data to the in-memory DB
	db.AddCategory("Personal", "blue")
	db.AddCategory("Work", "indigo")
	cats, _ := db.GetCategoriesWithServices("")
	db.AddService(cats[0].ID, "MyBlog", "http://blog.me", "", "", "", []string{"markus"})
	db.AddService(cats[1].ID, "Jira", "http://jira.work", "", "", "", []string{"markus", "andrea"})
	db.AddWidget("MyCal", "http://ical.me", "markus")
	widgets, _ := db.GetWidgets("markus")
	cacheData, _ := json.Marshal(db.WidgetCacheEntry{Events: []db.ICalEvent{{Title: "Test Event"}}})
	db.UpdateWidgetCache(widgets[0].ID, string(cacheData))

	db.SetUserPreferences("markus", db.UserPreferences{Theme: "dark", AccentColor: "#6366f1", SearchEngine: "https://customsearch.com/q="})

	t.Run("markus profile", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		api.HandleIndex(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
		expectedBodyContains := []string{
			"Index: markus",
			"Personal", "Work", // category names
			"Test Event",              // widget cache events rendered as struct
			"https://customsearch.com/q=", // search engine
		}
		body := rr.Body.String()
		for _, s := range expectedBodyContains {
			if !strings.Contains(body, s) {
				t.Errorf("Response body does not contain '%s', got: %s", s, body)
			}
		}
	})

	t.Run("andrea profile", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/andrea", nil)
		rr := httptest.NewRecorder()
		api.HandleIndex(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
		expectedBodyContains := []string{
			"Index: andrea",
			"Work",                  // Jira is in Work category, visible to andrea
			"https://duckduckgo.com/", // Andrea's search engine should be default
		}
		expectedBodyNotContains := []string{
			"https://customsearch.com/q=", // markus-specific search engine
		}
		body := rr.Body.String()
		for _, s := range expectedBodyContains {
			if !strings.Contains(body, s) {
				t.Errorf("Response body does not contain '%s', got: %s", s, body)
			}
		}
		for _, s := range expectedBodyNotContains {
			if strings.Contains(body, s) {
				t.Errorf("Response body unexpectedly contains '%s', got: %s", s, body)
			}
		}
	})
}

func TestHandleManage(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("ManageMe", "red")

	req := httptest.NewRequest("GET", "/manage", nil)
	rr := httptest.NewRecorder()
	api.HandleManage(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	expectedBodyContains := []string{
		"Manage: ManageMe",
		`<div id="category-list">ManageMe:0@red|</div>`,
	}
	body := rr.Body.String()
	for _, s := range expectedBodyContains {
		if !strings.Contains(body, s) {
			t.Errorf("Response body does not contain '%s', got: %s", s, body)
		}
	}
}


func TestHandleAddCategory(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	formData := url.Values{}
	formData.Set("name", "New Category")
	formData.Set("layout", "list")
	formData.Set("color", "purple")

	req := httptest.NewRequest("POST", "/add-category", strings.NewReader(formData.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	api.HandleAddCategory(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	categories, _ := db.GetCategoriesWithServices("")
	if len(categories) != 1 || categories[0].Name != "New Category" {
		t.Errorf("Category not added correctly: %v", categories)
	}

	if !strings.Contains(rr.Body.String(), `<div id="category-list">New Category:0@purple|</div>`) {
		t.Errorf("Expected category list to be rendered with new category, got %s", rr.Body.String())
	}
}

func TestHandleAddService(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("TestCat", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	formData := url.Values{}
	formData.Set("category_id", fmt.Sprintf("%d", catID))
	formData.Set("name", "New Service")
	formData.Set("url", "http://new.service")
	formData.Set("icon", "icon.png")
	formData.Set("description", "A new service")
	formData.Set("status_check", "http://status.new.service")
	formData.Add("visibility", "markus")
	formData.Add("visibility", "andrea")

	req := httptest.NewRequest("POST", "/add-service", strings.NewReader(formData.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	api.HandleAddService(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	categories, _ := db.GetCategoriesWithServices("")
	if len(categories[0].Services) != 1 || categories[0].Services[0].Name != "New Service" {
		t.Errorf("Service not added correctly: %v", categories[0].Services)
	}

	if !strings.Contains(rr.Body.String(), `<div id="category-list">TestCat:1@blue|</div>`) {
		t.Errorf("Expected category list to be rendered with new service, got %s", rr.Body.String())
	}
}

func TestHandleDeleteCategory(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("ToDelete", "red")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	r := chi.NewRouter()
	r.Delete("/delete-category/{id}", api.HandleDeleteCategory)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/delete-category/%d", catID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req) // Use the router to handle URL parameters

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	categories, _ := db.GetCategoriesWithServices("")
	if len(categories) != 0 {
		t.Errorf("Category not deleted correctly: %v", categories)
	}
	if !strings.Contains(rr.Body.String(), `<div id="category-list"></div>`) {
		t.Errorf("Expected empty category list to be rendered, got %s", rr.Body.String())
	}
}

func TestHandleDeleteService(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("TestCat", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID
	db.AddService(catID, "SvcToDelete", "http://todelete.me", "", "", "", []string{"markus"})
	// Re-fetch so Services slice is populated
	catsWithSvc, _ := db.GetCategoriesWithServices("")
	svcID := catsWithSvc[0].Services[0].ID

	r := chi.NewRouter()
	r.Delete("/delete-service/{id}", api.HandleDeleteService)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/delete-service/%d", svcID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	categories, _ := db.GetCategoriesWithServices("")
	if len(categories[0].Services) != 0 {
		t.Errorf("Service not deleted correctly: %v", categories[0].Services)
	}
	if !strings.Contains(rr.Body.String(), `<div id="category-list">TestCat:0@blue|</div>`) {
		t.Errorf("Expected category list with no services to be rendered, got %s", rr.Body.String())
	}
}

func TestHandleSortCategory(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("CatA", "red")
	db.AddCategory("CatB", "blue")
	db.AddCategory("CatC", "green")
	cats, _ := db.GetCategoriesWithServices("")
	catBID := cats[1].ID // SO 1

	r := chi.NewRouter()
	r.Post("/sort-category/{id}/{direction}", api.HandleSortCategory)

	// Move CatB up (swap with CatA)
	req := httptest.NewRequest("POST", fmt.Sprintf("/sort-category/%d/up", catBID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	categories, _ := db.GetCategoriesWithServices("")
	if len(categories) != 3 {
		t.Fatalf("Expected 3 categories, got %d", len(categories))
	}
	if categories[0].Name != "CatB" || categories[1].Name != "CatA" || categories[2].Name != "CatC" {
		t.Errorf("Category order after sort up incorrect: %v", categories)
	}
}

func TestHandleSortService(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("TestCat", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID
	db.AddService(catID, "Svc1", "url1", "", "", "", []string{"markus"}) // SO 0
	db.AddService(catID, "Svc2", "url2", "", "", "", []string{"markus"}) // SO 1
	db.AddService(catID, "Svc3", "url3", "", "", "", []string{"markus"}) // SO 2
	catsWithSvc, _ := db.GetCategoriesWithServices("")
	svc2ID := catsWithSvc[0].Services[1].ID

	r := chi.NewRouter()
	r.Post("/sort-service/{id}/{direction}", api.HandleSortService)

	// Move Svc2 up (swap with Svc1)
	req := httptest.NewRequest("POST", fmt.Sprintf("/sort-service/%d/up", svc2ID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	categories, _ := db.GetCategoriesWithServices("")
	if len(categories[0].Services) != 3 {
		t.Fatalf("Expected 3 services, got %d", len(categories[0].Services))
	}
	if categories[0].Services[0].Name != "Svc2" || categories[0].Services[1].Name != "Svc1" || categories[0].Services[2].Name != "Svc3" {
		t.Errorf("Service order after sort up incorrect: %v", categories[0].Services)
	}
}

func TestHandleAddWidget(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	formData := url.Values{}
	formData.Set("name", "New Widget")
	formData.Set("url", "http://widget.url")
	formData.Set("profile", "markus")

	req := httptest.NewRequest("POST", "/add-widget", strings.NewReader(formData.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	api.HandleAddWidget(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	widgets, _ := db.GetWidgets("markus")
	if len(widgets) != 1 || widgets[0].Name != "New Widget" {
		t.Errorf("Widget not added correctly: %v", widgets)
	}
}

func TestHandleDeleteWidget(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	db.AddWidget("WidgetToDelete", "http://delete.me", "all")
	widgets, _ := db.GetAllWidgets()
	widgetID := widgets[0].ID

	r := chi.NewRouter()
	r.Delete("/delete-widget/{id}", api.HandleDeleteWidget)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/delete-widget/%d", widgetID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	widgetsAfter, _ := db.GetAllWidgets()
	if len(widgetsAfter) != 0 {
		t.Errorf("Widget not deleted correctly: %v", widgetsAfter)
	}
}

func TestHandleCloneProfile(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Initial setup with a service for 'markus'
	db.AddCategory("Work", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	workCatID := cats[0].ID
	db.AddService(workCatID, "MarkusOnly", "url1", "", "", "", []string{"markus"})

	// Create a request to clone from 'markus' to 'andrea'
	formData := url.Values{}
	formData.Set("name", "andrea") // Name of the target profile

	r := chi.NewRouter()
	r.Post("/manage/profile/{slug}/clone", api.HandleCloneProfile)
	r.Get("/manage", api.HandleManage) // HandleManage is called after cloning

	req := httptest.NewRequest("POST", "/manage/profile/markus/clone", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Verify that the 'MarkusOnly' service is now visible to 'andrea'
	andreaCats, _ := db.GetCategoriesWithServices("andrea")
	if len(andreaCats[0].Services) != 1 || andreaCats[0].Services[0].Name != "MarkusOnly" {
		t.Errorf("Service 'MarkusOnly' not cloned to Andrea: %v", andreaCats)
	}

	// Check the response body (from HandleManage) for expected content
	if !strings.Contains(rr.Body.String(), "Manage: Work") { // from testdata/manage.html
		t.Errorf("Response body from HandleManage missing expected content, got: %s", rr.Body.String())
	}
}

func TestHandleDiscoveryInbox(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	suggested := db.SuggestedService{Name: "DiscoveredService", URL: "http://d.local", Category: "Discovered"}
	suggestedJSON, _ := json.Marshal(suggested)
	db.AddDiscoveryItem("containerID1", string(suggestedJSON))

	req := httptest.NewRequest("GET", "/discovery-inbox", nil)
	rr := httptest.NewRecorder()
	api.HandleDiscoveryInbox(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), `<div id="discovery-inbox">DiscoveredService|</div>`) {
		t.Errorf("Expected discovery inbox partial with item, got %s", rr.Body.String())
	}
}

func TestHandleAcceptDiscovery(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	suggested := db.SuggestedService{Name: "AcceptMe", URL: "http://accept.me", Category: "Discovered"}
	suggestedJSON, _ := json.Marshal(suggested)
	db.AddDiscoveryItem("containerID_accept", string(suggestedJSON))
	items, _ := db.GetDiscoveryInbox()
	itemID := items[0].ID

	// Need "Discovered" category to exist for db.AcceptDiscoveryItem
	db.AddCategory("Discovered", "orange")

	r := chi.NewRouter()
	r.Post("/accept-discovery/{id}", api.HandleAcceptDiscovery)

	req := httptest.NewRequest("POST", fmt.Sprintf("/accept-discovery/%d", itemID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	inboxItems, _ := db.GetDiscoveryInbox()
	if len(inboxItems) != 0 {
		t.Errorf("Discovery item not removed from inbox after accept: %v", inboxItems)
	}

	// Verify service was added
	categories, _ := db.GetCategoriesWithServices("markus")
	found := false
	for _, cat := range categories {
		for _, svc := range cat.Services {
			if svc.Name == "AcceptMe" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Errorf("Accepted service was not added to regular services")
	}

	if !strings.Contains(rr.Body.String(), `<div id="discovery-inbox"></div>`) {
		t.Errorf("Expected empty discovery inbox partial after accept, got %s", rr.Body.String())
	}
}

func TestHandleIgnoreDiscovery(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	suggested := db.SuggestedService{Name: "IgnoreMe", URL: "http://ignore.me", Category: "Discovered"}
	suggestedJSON, _ := json.Marshal(suggested)
	db.AddDiscoveryItem("containerID_ignore", string(suggestedJSON))
	items, _ := db.GetDiscoveryInbox()
	itemID := items[0].ID

	r := chi.NewRouter()
	r.Post("/ignore-discovery/{id}", api.HandleIgnoreDiscovery)

	req := httptest.NewRequest("POST", fmt.Sprintf("/ignore-discovery/%d", itemID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	inboxItems, _ := db.GetDiscoveryInbox()
	if len(inboxItems) != 0 {
		t.Errorf("Discovery item not removed from inbox after ignore: %v", inboxItems)
	}

	if !strings.Contains(rr.Body.String(), `<div id="discovery-inbox"></div>`) {
		t.Errorf("Expected empty discovery inbox partial after ignore, got %s", rr.Body.String())
	}
}

// Ensure testdata directory and files exist for embedding
// This will be created in the current working directory which is /home/debian/new_pro/homeport
// before running the test command.
//
//
