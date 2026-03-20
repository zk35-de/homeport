package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

type Category struct {
	ID        int
	Name      string
	Layout    string // 'tiles', 'list', 'icons'
	Color     string // e.g., 'indigo'
	SortOrder int
	Services  []Service
}

type Service struct {
	ID          int
	CategoryID  int
	Name        string
	URL         string
	Icon        string
	Description string
	StatusCheck string
	SortOrder   int
	Alive       bool   // joined from service_status
	LastCheck   string // joined from service_status (sqlite datetime string)
	VisibleTo   []string  // profiles
}

type Widget struct {
	ID        int
	Type      string
	Name      string
	Config    string // json string
	Profile   string
	SortOrder int
	Events    []ICalEvent `json:"-"` // populated from cache
}

type ICalEvent struct {
	Title      string
	Start      string
	End        string
	IsToday    bool
	IsTomorrow bool
}

type WidgetCacheEntry struct {
	Events []ICalEvent
}

func InitDB(dbPath string) error {
	if dbPath == "" {
		dbPath = "./data/homeport.db"
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	// Encode foreign_keys pragma directly in DSN so it applies to every connection.
	dsn := dbPath + "?_pragma=foreign_keys(on)"
	if dbPath == ":memory:" {
		dsn = ":memory:?_pragma=foreign_keys(on)"
	}

	var err error
	DB, err = sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping db: %w", err)
	}

	// Create tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			layout TEXT NOT NULL DEFAULT 'tiles',
			color TEXT NOT NULL DEFAULT 'indigo',
			sort_order INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS services (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			icon TEXT DEFAULT '',
			description TEXT DEFAULT '',
			status_check TEXT DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS visibility (
			service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
			profile TEXT NOT NULL,
			PRIMARY KEY (service_id, profile)
		);`,
		`CREATE TABLE IF NOT EXISTS service_status (
			service_id INTEGER PRIMARY KEY REFERENCES services(id) ON DELETE CASCADE,
			alive INTEGER,
			last_check DATETIME
		);`,
		`CREATE TABLE IF NOT EXISTS widgets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL DEFAULT 'ical',
			name TEXT NOT NULL,
			config TEXT NOT NULL,
			profile TEXT NOT NULL DEFAULT 'all',
			sort_order INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS widget_cache (
			widget_id INTEGER PRIMARY KEY REFERENCES widgets(id) ON DELETE CASCADE,
			data TEXT NOT NULL,
			fetched_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS discovery_inbox (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			container_id TEXT NOT NULL UNIQUE,
			suggested    TEXT NOT NULL,
			seen_at      DATETIME NOT NULL DEFAULT (datetime('now')),
			ignored      INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS user_settings (
			profile        TEXT PRIMARY KEY,
			search_engine  TEXT NOT NULL DEFAULT 'https://duckduckgo.com/'
		);`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

type DiscoveryItem struct {
	ID          int
	ContainerID string
	Suggested   SuggestedService // JSON-decoded aus suggested
	SeenAt      string
}

type SuggestedService struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Profile     string `json:"profile"`
	StatusCheck string `json:"status_check"`
}

// GetDiscoveryInbox returns all non-ignored discovery items.
func GetDiscoveryInbox() ([]DiscoveryItem, error) {
	rows, err := DB.Query(`SELECT id, container_id, suggested, seen_at FROM discovery_inbox WHERE ignored = 0 ORDER BY seen_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DiscoveryItem
	for rows.Next() {
		var item DiscoveryItem
		var suggestedJSON string
		if err := rows.Scan(&item.ID, &item.ContainerID, &suggestedJSON, &item.SeenAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(suggestedJSON), &item.Suggested); err != nil {
			return nil, fmt.Errorf("failed to unmarshal suggested service for item %d: %w", item.ID, err)
		}
		items = append(items, item)
	}
	return items, nil
}

// AddDiscoveryItem adds a new discovery item if it doesn't already exist (based on container_id).
func AddDiscoveryItem(containerID, suggested string) error {
	_, err := DB.Exec(`INSERT OR IGNORE INTO discovery_inbox (container_id, suggested) VALUES (?, ?)`, containerID, suggested)
	return err
}

// IgnoreDiscoveryItem sets the ignored flag for a discovery item.
func IgnoreDiscoveryItem(id int) error {
	_, err := DB.Exec(`UPDATE discovery_inbox SET ignored = 1 WHERE id = ?`, id)
	return err
}

// AcceptDiscoveryItem reads an item, calls AddService(), then deletes the item.
func AcceptDiscoveryItem(id int) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	var containerID string
	var suggestedJSON string
	err = tx.QueryRow(`SELECT container_id, suggested FROM discovery_inbox WHERE id = ?`, id).Scan(&containerID, &suggestedJSON)
	if err != nil {
		return fmt.Errorf("failed to get discovery item %d: %w", id, err)
	}

	var suggestedService SuggestedService
	if err := json.Unmarshal([]byte(suggestedJSON), &suggestedService); err != nil {
		return fmt.Errorf("failed to unmarshal suggested service for item %d: %w", id, err)
	}

	// For now, add to category ID 1 and profiles markus, andrea.
	// In a real application, this might be configurable or selected by the user.
	// We need to ensure a category with ID 1 exists. If not, the AddService will fail or we need to create one.
	// For simplicity, assuming category 1 exists or handling the error.
	categoryID := 1 // Default category
	profiles := []string{"markus", "andrea"} // Default profiles

	// Check if the suggested category exists, if not, create it.
	var catID int
	err = tx.QueryRow(`SELECT id FROM categories WHERE name = ?`, suggestedService.Category).Scan(&catID)
	if err == sql.ErrNoRows {
		// Category doesn't exist, create it
		res, err := tx.Exec(`INSERT INTO categories (name, layout, color) VALUES (?, ?, ?)`, suggestedService.Category, "tiles", "indigo") // Default layout and color
		if err != nil {
			return fmt.Errorf("failed to create category %s: %w", suggestedService.Category, err)
		}
		lastID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get last insert ID for category: %w", err)
		}
		categoryID = int(lastID)
	} else if err != nil {
		return fmt.Errorf("failed to query category %s: %w", suggestedService.Category, err)
	} else {
		categoryID = catID
	}


	// Insert service within the transaction (not via AddService which uses DB directly).
	res, err := tx.Exec(`INSERT INTO services (category_id, name, url, icon, description, status_check, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM services WHERE category_id = ?))`,
		categoryID, suggestedService.Name, suggestedService.URL, suggestedService.Icon,
		suggestedService.Description, suggestedService.StatusCheck, categoryID)
	if err != nil {
		return fmt.Errorf("failed to add service from discovery item %d: %w", id, err)
	}
	svcID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get service ID for discovery item %d: %w", id, err)
	}
	for _, p := range profiles {
		if _, err := tx.Exec(`INSERT INTO visibility (service_id, profile) VALUES (?, ?)`, svcID, p); err != nil {
			return fmt.Errorf("failed to set visibility for service from discovery item %d: %w", id, err)
		}
	}

	_, err = tx.Exec(`DELETE FROM discovery_inbox WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete discovery item %d: %w", id, err)
	}

	return nil
}

func GetCategoriesWithServices(profile string) ([]Category, error) {
	rows, err := DB.Query(`SELECT id, name, layout, color, sort_order FROM categories ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Layout, &c.Color, &c.SortOrder); err != nil {
			return nil, err
		}

		// Get services for this category visible to profile
		// Or if profile is empty (manage mode), get all services
		var sRows *sql.Rows
		var sErr error

		query := `
			SELECT s.id, s.category_id, s.name, s.url, s.icon, s.description, s.status_check, s.sort_order, 
			       COALESCE(ss.alive, 0), COALESCE(ss.last_check, '0001-01-01 00:00:00')
			FROM services s
			LEFT JOIN service_status ss ON s.id = ss.service_id
			WHERE s.category_id = ?
		`

		if profile != "" {
			query += ` AND s.id IN (SELECT service_id FROM visibility WHERE profile = ?)`
			query += ` ORDER BY s.sort_order ASC`
			sRows, sErr = DB.Query(query, c.ID, profile)
		} else {
			query += ` ORDER BY s.sort_order ASC`
			sRows, sErr = DB.Query(query, c.ID)
		}

		if sErr != nil {
			return nil, sErr
		}
		defer sRows.Close()

		for sRows.Next() {
			var s Service
			var alive int
			if err := sRows.Scan(&s.ID, &s.CategoryID, &s.Name, &s.URL, &s.Icon, &s.Description, &s.StatusCheck, &s.SortOrder, &alive, &s.LastCheck); err != nil {
				return nil, err
			}
			s.Alive = alive == 1

			// If managing (profile == ""), fetch visibility
			if profile == "" {
				vRows, vErr := DB.Query(`SELECT profile FROM visibility WHERE service_id = ?`, s.ID)
				if vErr == nil {
					for vRows.Next() {
						var p string
						vRows.Scan(&p)
						s.VisibleTo = append(s.VisibleTo, p)
					}
					vRows.Close()
				}
			}

			c.Services = append(c.Services, s)
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func AddCategory(name, layout, color string) error {
	_, err := DB.Exec(`INSERT INTO categories (name, layout, color, sort_order) VALUES (?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM categories))`, name, layout, color)
	return err
}

func AddService(categoryID int, name, url, icon, desc, statusCheck string, profiles []string) error {
	res, err := DB.Exec(`INSERT INTO services (category_id, name, url, icon, description, status_check, sort_order) VALUES (?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM services WHERE category_id = ?))`, categoryID, name, url, icon, desc, statusCheck, categoryID)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}

	for _, p := range profiles {
		if _, err := DB.Exec(`INSERT INTO visibility (service_id, profile) VALUES (?, ?)`, id, p); err != nil {
			return err
		}
	}
	return nil
}

func DeleteCategory(id int) error {
	_, err := DB.Exec(`DELETE FROM categories WHERE id = ?`, id)
	return err
}

func DeleteService(id int) error {
	_, err := DB.Exec(`DELETE FROM services WHERE id = ?`, id)
	return err
}

func UpdateServiceStatus(id int, alive bool) error {
	val := 0
	if alive {
		val = 1
	}
	_, err := DB.Exec(`INSERT INTO service_status (service_id, alive, last_check) VALUES (?, ?, datetime('now')) ON CONFLICT(service_id) DO UPDATE SET alive = ?, last_check = datetime('now')`, id, val, val)
	return err
}

func GetAllServicesWithStatusCheck() ([]Service, error) {
	rows, err := DB.Query(`SELECT id, status_check FROM services WHERE status_check != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.StatusCheck); err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	return services, nil
}

func UpdateCategorySort(id, newOrder int) error {
	_, err := DB.Exec(`UPDATE categories SET sort_order = ? WHERE id = ?`, newOrder, id)
	return err
}

func UpdateServiceSort(id, newOrder int) error {
	_, err := DB.Exec(`UPDATE services SET sort_order = ? WHERE id = ?`, newOrder, id)
	return err
}

// Widgets

func AddWidget(name, icalURL, profile string) error {
	config := fmt.Sprintf(`{"url": "%s"}`, icalURL)
	_, err := DB.Exec(`INSERT INTO widgets (name, config, profile, sort_order) VALUES (?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM widgets))`, name, config, profile)
	return err
}

func DeleteWidget(id int) error {
	_, err := DB.Exec(`DELETE FROM widgets WHERE id = ?`, id)
	return err
}

func GetWidgets(profile string) ([]Widget, error) {
	rows, err := DB.Query(`SELECT id, type, name, config, profile, sort_order FROM widgets WHERE profile = ? OR profile = 'all' ORDER BY sort_order ASC`, profile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var widgets []Widget
	for rows.Next() {
		var w Widget
		if err := rows.Scan(&w.ID, &w.Type, &w.Name, &w.Config, &w.Profile, &w.SortOrder); err != nil {
			return nil, err
		}
		widgets = append(widgets, w)
	}
	return widgets, nil
}

func GetAllWidgets() ([]Widget, error) {
	rows, err := DB.Query(`SELECT id, type, name, config, profile, sort_order FROM widgets ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var widgets []Widget
	for rows.Next() {
		var w Widget
		if err := rows.Scan(&w.ID, &w.Type, &w.Name, &w.Config, &w.Profile, &w.SortOrder); err != nil {
			return nil, err
		}
		widgets = append(widgets, w)
	}
	return widgets, nil
}

// CloneToAndrea kopiert alle Services die 'markus' in visibility haben auch zu 'andrea',
// überspringt dabei Services die zur Kategorie mit color='cyan' gehören (IT-Kategorien)
// oder die visibility='andrea' bereits haben.
func CloneToAndrea() (added int, skipped int, err error) {
	tx, err := DB.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r) // re-throw panic
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	rows, err := tx.Query(`
		SELECT
			s.id
		FROM
			services s
		JOIN
			visibility vm ON s.id = vm.service_id
		JOIN
			categories c ON s.category_id = c.id
		WHERE
			vm.profile = 'markus'
			AND c.color != 'cyan'
	`)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to query services for markus: %w", err)
	}
	defer rows.Close()

	var serviceIDsToClone []int
	for rows.Next() {
		var serviceID int
		if err := rows.Scan(&serviceID); err != nil {
			return 0, 0, fmt.Errorf("failed to scan service ID: %w", err)
		}
		serviceIDsToClone = append(serviceIDsToClone, serviceID)
	}

	for _, serviceID := range serviceIDsToClone {
		// Check if service is already visible to 'andrea'
		var exists int
		err := tx.QueryRow(`SELECT COUNT(*) FROM visibility WHERE service_id = ? AND profile = 'andrea'`, serviceID).Scan(&exists)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to check existing visibility for service %d: %w", serviceID, err)
		}

		if exists > 0 {
			skipped++
			continue
		}

		// Insert new visibility for 'andrea'
		_, err = tx.Exec(`INSERT INTO visibility (service_id, profile) VALUES (?, 'andrea')`, serviceID)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to insert visibility for service %d and andrea: %w", serviceID, err)
		}
		added++
	}

	return added, skipped, nil
}

func GetWidgetCache(widgetID int) (*WidgetCacheEntry, error) {
	row := DB.QueryRow(`SELECT data FROM widget_cache WHERE widget_id = ?`, widgetID)
	var data string
	if err := row.Scan(&data); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found is not an error
		}
		return nil, err
	}

	var entry WidgetCacheEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func UpdateWidgetCache(widgetID int, data string) error {
	_, err := DB.Exec(`INSERT INTO widget_cache (widget_id, data, fetched_at) VALUES (?, ?, datetime('now')) ON CONFLICT(widget_id) DO UPDATE SET data = ?, fetched_at = datetime('now')`, widgetID, data, data)
	return err
}

// GetSearchEngine returns the configured search engine action URL for a profile.
// Falls back to DuckDuckGo if not set.
func GetSearchEngine(profile string) string {
	var url string
	err := DB.QueryRow(`SELECT search_engine FROM user_settings WHERE profile = ?`, profile).Scan(&url)
	if err != nil || url == "" {
		return "https://duckduckgo.com/"
	}
	return url
}

// SetSearchEngine stores the search engine action URL for a profile.
func SetSearchEngine(profile, engineURL string) error {
	_, err := DB.Exec(`INSERT INTO user_settings (profile, search_engine) VALUES (?, ?)
		ON CONFLICT(profile) DO UPDATE SET search_engine = ?`, profile, engineURL, engineURL)
	return err
}

// GetAllSearchEngines returns search engine settings for all known profiles.
func GetAllSearchEngines() map[string]string {
	result := map[string]string{
		"markus": "https://duckduckgo.com/",
		"andrea": "https://duckduckgo.com/",
	}
	rows, err := DB.Query(`SELECT profile, search_engine FROM user_settings`)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var p, u string
		if rows.Scan(&p, &u) == nil {
			result[p] = u
		}
	}
	return result
}
