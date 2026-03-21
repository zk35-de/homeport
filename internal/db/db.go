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

type Profile struct {
	ID        int
	Slug      string // URL-Pfad z.B. "markus"
	Name      string // Anzeigename z.B. "Markus"
	IsDefault bool
	SortOrder int
}

type Page struct {
	ID        int
	Profile   string
	Name      string
	Icon      string
	SortOrder int
}

type Category struct {
	ID        int
	Name      string
	Layout    string // 'tiles', 'list', 'icons'
	Color     string // e.g., 'indigo'
	SortOrder int
	ColSpan   int    // 1=full, 2=half, 3=third
	SortMode  string // 'manual' or 'usage'
	PageID    int    // 0 = unassigned (shown on all page tabs)
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
	PageID    int // 0 = unassigned (shown on all page tabs)
	Events    []ICalEvent   `json:"-"` // populated from cache
	Weather   *WeatherCache `json:"-"` // populated from cache for weather widgets
	RSSItems  []RSSItem     `json:"-"` // populated from cache for rss widgets
	Todos         []TodoItem     `json:"-"` // populated from DB for todo widgets
	BookmarkLinks []BookmarkLink `json:"-"` // populated for type=bookmarks
	NoteContent   string         `json:"-"` // populated for type=notes
	// Clock widget fields (populated for type=clock)
	ClockMode        string `json:"-"`
	ClockTimezone    string `json:"-"`
	ClockShowSeconds bool   `json:"-"`
	ClockShowDate    bool   `json:"-"`
	ClockCountdown   string `json:"-"`
}

// RSSItem is one entry from an RSS/Atom feed.
type RSSItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	PubDate string `json:"pub_date"`
}

// BookmarkLink is a single link stored in a bookmarks widget config.
type BookmarkLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Icon string `json:"icon"`
}

// ClickStat holds analytics data for a service.
type ClickStat struct {
	ServiceID   int
	ServiceName string
	ServiceURL  string
	ServiceIcon string
	ClickCount  int
	LastClicked string
	Profile     string
}

// TodoItem is a single to-do entry linked to a widget.
type TodoItem struct {
	ID        int    `json:"id"`
	WidgetID  int    `json:"widget_id"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
	DueDate   string `json:"due_date"`
	SortOrder int    `json:"sort_order"`
}

type ICalEvent struct {
	Title      string
	Start      string
	End        string
	IsToday    bool
	IsTomorrow bool
}

type WidgetCacheEntry struct {
	Events   []ICalEvent `json:"Events,omitempty"`
	RSSItems []RSSItem   `json:"RSSItems,omitempty"`
}

// WeatherCache holds cached weather data for weather widgets.
type WeatherCache struct {
	Temperature float64
	WeatherCode int
	Description string
	WindSpeed   float64
	Humidity    int
	IsDay       bool
	CityName    string
	Forecast    []WeatherForecastDay
}

type WeatherForecastDay struct {
	Date    string
	TempMax float64
	TempMin float64
	Code    int
	Desc    string
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
		`CREATE TABLE IF NOT EXISTS user_preferences (
			profile       TEXT PRIMARY KEY,
			theme         TEXT NOT NULL DEFAULT 'dark',
			accent_color  TEXT NOT NULL DEFAULT '#6366f1',
			search_engine TEXT NOT NULL DEFAULT 'https://duckduckgo.com/',
			background    TEXT NOT NULL DEFAULT 'aurora',
			language      TEXT NOT NULL DEFAULT 'de',
			layout        TEXT NOT NULL DEFAULT 'grid'
		);`,
		`CREATE TABLE IF NOT EXISTS short_urls (
			code       TEXT PRIMARY KEY,
			url        TEXT NOT NULL,
			clicks     INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		);`,
		`CREATE TABLE IF NOT EXISTS profiles (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			slug       TEXT NOT NULL UNIQUE,
			name       TEXT NOT NULL,
			is_default INTEGER NOT NULL DEFAULT 0,
			sort_order INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS todos (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			widget_id  INTEGER NOT NULL REFERENCES widgets(id) ON DELETE CASCADE,
			text       TEXT NOT NULL,
			done       INTEGER NOT NULL DEFAULT 0,
			due_date   TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS service_clicks (
			service_id  INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
			profile     TEXT NOT NULL,
			click_count INTEGER NOT NULL DEFAULT 1,
			last_clicked DATETIME NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (service_id, profile)
		);`,
		`CREATE TABLE IF NOT EXISTS notes (
			widget_id  INTEGER PRIMARY KEY REFERENCES widgets(id) ON DELETE CASCADE,
			content    TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		);`,
		`CREATE TABLE IF NOT EXISTS pages (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			profile    TEXT NOT NULL,
			name       TEXT NOT NULL,
			icon       TEXT NOT NULL DEFAULT '📄',
			sort_order INTEGER NOT NULL DEFAULT 0
		);`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	// Idempotent migrations: ignore errors (column may already exist)
	_, _ = DB.Exec(`ALTER TABLE widgets ADD COLUMN visible INTEGER NOT NULL DEFAULT 1`)
	_, _ = DB.Exec(`ALTER TABLE user_preferences ADD COLUMN custom_css TEXT NOT NULL DEFAULT ''`)
	_, _ = DB.Exec(`ALTER TABLE categories ADD COLUMN col_span INTEGER NOT NULL DEFAULT 1`)
	_, _ = DB.Exec(`ALTER TABLE categories ADD COLUMN sort_mode TEXT NOT NULL DEFAULT 'manual'`)
	_, _ = DB.Exec(`ALTER TABLE user_preferences ADD COLUMN background_mode TEXT NOT NULL DEFAULT 'aurora'`)
	_, _ = DB.Exec(`ALTER TABLE categories ADD COLUMN page_id INTEGER REFERENCES pages(id) ON DELETE SET NULL`)
	_, _ = DB.Exec(`ALTER TABLE widgets ADD COLUMN page_id INTEGER REFERENCES pages(id) ON DELETE SET NULL`)

	// Seed default profiles if table is empty
	var count int
	DB.QueryRow(`SELECT COUNT(*) FROM profiles`).Scan(&count)
	if count == 0 {
		DB.Exec(`INSERT INTO profiles (slug, name, is_default, sort_order) VALUES ('markus', 'Markus', 1, 0)`)
		DB.Exec(`INSERT INTO profiles (slug, name, is_default, sort_order) VALUES ('andrea', 'Andrea', 0, 1)`)
	}

	return nil
}

func ReinitDB(dbPath string) error {
	if DB != nil {
		DB.Close()
	}
	return InitDB(dbPath)
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

	// For now, add to category ID 1 and profiles from the database.
	categoryID := 1 // Default category
    profs, _ := GetProfiles()
    profiles := make([]string, len(profs))
    for i, p := range profs { profiles[i] = p.Slug }

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
	rows, err := DB.Query(`SELECT id, name, layout, color, sort_order, COALESCE(col_span,1), COALESCE(sort_mode,'manual'), COALESCE(page_id,0) FROM categories ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Layout, &c.Color, &c.SortOrder, &c.ColSpan, &c.SortMode, &c.PageID); err != nil {
			return nil, err
		}
		if c.ColSpan < 1 || c.ColSpan > 3 {
			c.ColSpan = 1
		}

		// Get services for this category visible to profile
		// Or if profile is empty (manage mode), get all services
		var sRows *sql.Rows
		var sErr error

		base := `
			SELECT s.id, s.category_id, s.name, s.url, s.icon, s.description, s.status_check, s.sort_order,
			       COALESCE(ss.alive, 0), COALESCE(ss.last_check, '0001-01-01 00:00:00')
			FROM services s
			LEFT JOIN service_status ss ON s.id = ss.service_id
			WHERE s.category_id = ?
		`
		if profile != "" && c.SortMode == "usage" {
			query := base + ` AND s.id IN (SELECT service_id FROM visibility WHERE profile = ?)
				ORDER BY (SELECT COALESCE(click_count,0) FROM service_clicks WHERE service_id=s.id AND profile=?) DESC, s.sort_order ASC`
			sRows, sErr = DB.Query(query, c.ID, profile, profile)
		} else if profile != "" {
			query := base + ` AND s.id IN (SELECT service_id FROM visibility WHERE profile = ?) ORDER BY s.sort_order ASC`
			sRows, sErr = DB.Query(query, c.ID, profile)
		} else {
			sRows, sErr = DB.Query(base+` ORDER BY s.sort_order ASC`, c.ID)
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

func AddCategory(name, layout, color string) (int64, error) {
	res, err := DB.Exec(`INSERT INTO categories (name, layout, color, sort_order) VALUES (?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM categories))`, name, layout, color)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetPages returns all pages for a profile ordered by sort_order.
func GetPages(profile string) ([]Page, error) {
	rows, err := DB.Query(`SELECT id, profile, name, icon, sort_order FROM pages WHERE profile = ? ORDER BY sort_order ASC`, profile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pages []Page
	for rows.Next() {
		var p Page
		if err := rows.Scan(&p.ID, &p.Profile, &p.Name, &p.Icon, &p.SortOrder); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, nil
}

// AddPage inserts a new page and returns its ID.
func AddPage(profile, name, icon string) (int64, error) {
	res, err := DB.Exec(`INSERT INTO pages (profile, name, icon, sort_order) VALUES (?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM pages WHERE profile = ?))`, profile, name, icon, profile)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DeletePage removes a page; categories/widgets with this page_id get page_id=NULL (ON DELETE SET NULL).
func DeletePage(id int) error {
	_, err := DB.Exec(`DELETE FROM pages WHERE id = ?`, id)
	return err
}

// UpdatePage renames a page and/or changes its icon.
func UpdatePage(id int, name, icon string) error {
	_, err := DB.Exec(`UPDATE pages SET name=?, icon=? WHERE id=?`, name, icon, id)
	return err
}

// UpdatePageSort sets sort_order for a page.
func UpdatePageSort(id, sortOrder int) error {
	_, err := DB.Exec(`UPDATE pages SET sort_order = ? WHERE id = ?`, sortOrder, id)
	return err
}

// SetCategoryPage assigns a category to a page (0 = unassigned).
func SetCategoryPage(categoryID, pageID int) error {
	if pageID == 0 {
		_, err := DB.Exec(`UPDATE categories SET page_id = NULL WHERE id = ?`, categoryID)
		return err
	}
	_, err := DB.Exec(`UPDATE categories SET page_id = ? WHERE id = ?`, pageID, categoryID)
	return err
}

// SetWidgetPage assigns a widget to a page (0 = unassigned).
func SetWidgetPage(widgetID, pageID int) error {
	if pageID == 0 {
		_, err := DB.Exec(`UPDATE widgets SET page_id = NULL WHERE id = ?`, widgetID)
		return err
	}
	_, err := DB.Exec(`UPDATE widgets SET page_id = ? WHERE id = ?`, pageID, widgetID)
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

func UpdateService(id int, name, url, icon, desc, statusCheck string, profiles []string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE services SET name=?, url=?, icon=?, description=?, status_check=? WHERE id=?`,
		name, url, icon, desc, statusCheck, id); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM service_profiles WHERE service_id=?`, id); err != nil {
		return err
	}
	for _, p := range profiles {
		if _, err := tx.Exec(`INSERT INTO service_profiles (service_id, profile) VALUES (?, ?)`, id, p); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func UpdateCategory(id int, name, layout, color string) error {
	_, err := DB.Exec(`UPDATE categories SET name=?, layout=?, color=? WHERE id=?`, name, layout, color, id)
	return err
}

// GetService returns a single service by ID.
func GetService(id int) (*Service, error) {
	row := DB.QueryRow(`SELECT id, category_id, name, url, icon, description, status_check, sort_order FROM services WHERE id=?`, id)
	var s Service
	if err := row.Scan(&s.ID, &s.CategoryID, &s.Name, &s.URL, &s.Icon, &s.Description, &s.StatusCheck, &s.SortOrder); err != nil {
		return nil, err
	}
	rows, err := DB.Query(`SELECT profile FROM service_profiles WHERE service_id=?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		s.VisibleTo = append(s.VisibleTo, p)
	}
	return &s, nil
}

// GetCategory returns a single category by ID (without services).
func GetCategory(id int) (*Category, error) {
	row := DB.QueryRow(`SELECT id, name, layout, color, sort_order, COALESCE(col_span,1) FROM categories WHERE id=?`, id)
	var c Category
	if err := row.Scan(&c.ID, &c.Name, &c.Layout, &c.Color, &c.SortOrder, &c.ColSpan); err != nil {
		return nil, err
	}
	return &c, nil
}

func UpdateCategorySpan(id, span int) error {
	if span < 1 || span > 3 {
		span = 1
	}
	_, err := DB.Exec(`UPDATE categories SET col_span=? WHERE id=?`, span, id)
	return err
}

// Widgets

func AddWidget(name, icalURL, profile string) error {
	config := fmt.Sprintf(`{"url": "%s"}`, icalURL)
	_, err := DB.Exec(`INSERT INTO widgets (name, type, config, profile, sort_order) VALUES (?, 'ical', ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM widgets))`, name, config, profile)
	return err
}

func AddWidgetTyped(name, widgetType, config, profile string) error {
	_, err := DB.Exec(`INSERT INTO widgets (name, type, config, profile, sort_order) VALUES (?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM widgets))`, name, widgetType, config, profile)
	return err
}

func DeleteWidget(id int) error {
	_, err := DB.Exec(`DELETE FROM widgets WHERE id = ?`, id)
	return err
}

func populateWidgetFields(w *Widget) {
	if w.Type == "bookmarks" {
		var cfg struct {
			Links []BookmarkLink `json:"links"`
		}
		if err := json.Unmarshal([]byte(w.Config), &cfg); err == nil {
			w.BookmarkLinks = cfg.Links
		}
		if w.BookmarkLinks == nil {
			w.BookmarkLinks = []BookmarkLink{}
		}
	}
	if w.Type == "clock" {
		var cfg struct {
			Mode        string `json:"mode"`
			Timezone    string `json:"timezone"`
			ShowSeconds bool   `json:"show_seconds"`
			ShowDate    bool   `json:"show_date"`
			Countdown   string `json:"countdown"`
		}
		if err := json.Unmarshal([]byte(w.Config), &cfg); err == nil {
			w.ClockMode = cfg.Mode
			if w.ClockMode == "" {
				w.ClockMode = "digital"
			}
			w.ClockTimezone = cfg.Timezone
			if w.ClockTimezone == "" {
				w.ClockTimezone = "Europe/Berlin"
			}
			w.ClockShowSeconds = cfg.ShowSeconds
			w.ClockShowDate = cfg.ShowDate
			w.ClockCountdown = cfg.Countdown
		}
	}
}

func GetWidgets(profile string) ([]Widget, error) {
	rows, err := DB.Query(`SELECT id, type, name, config, profile, sort_order, COALESCE(page_id,0) FROM widgets WHERE profile = ? OR profile = 'all' ORDER BY sort_order ASC`, profile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var widgets []Widget
	for rows.Next() {
		var w Widget
		if err := rows.Scan(&w.ID, &w.Type, &w.Name, &w.Config, &w.Profile, &w.SortOrder, &w.PageID); err != nil {
			return nil, err
		}
		populateWidgetFields(&w)
		widgets = append(widgets, w)
	}
	return widgets, nil
}

// UpdateWidget partially updates a widget's mutable fields (nil = no change).
func UpdateWidget(id int, name, config, profile *string) error {
	if name == nil && config == nil && profile == nil {
		return nil
	}
	q := `UPDATE widgets SET`
	args := []any{}
	sep := " "
	if name != nil {
		q += sep + `name = ?`
		args = append(args, *name)
		sep = ", "
	}
	if config != nil {
		q += sep + `config = ?`
		args = append(args, *config)
		sep = ", "
	}
	if profile != nil {
		q += sep + `profile = ?`
		args = append(args, *profile)
	}
	q += ` WHERE id = ?`
	args = append(args, id)
	_, err := DB.Exec(q, args...)
	return err
}

// UpdateWidgetSort sets sort_order for a single widget.
func UpdateWidgetSort(id, sortOrder int) error {
	_, err := DB.Exec(`UPDATE widgets SET sort_order = ? WHERE id = ?`, sortOrder, id)
	return err
}

func GetAllWidgets() ([]Widget, error) {
	rows, err := DB.Query(`SELECT id, type, name, config, profile, sort_order, COALESCE(page_id,0) FROM widgets ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var widgets []Widget
	for rows.Next() {
		var w Widget
		if err := rows.Scan(&w.ID, &w.Type, &w.Name, &w.Config, &w.Profile, &w.SortOrder, &w.PageID); err != nil {
			return nil, err
		}
		populateWidgetFields(&w)
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
    profiles, _ := GetProfiles()
    result := make(map[string]string)
    for _, p := range profiles {
        result[p.Slug] = GetSearchEngine(p.Slug)
    }
    return result
}

// UserPreferences holds per-profile UI preferences.
type UserPreferences struct {
	Profile        string `json:"profile"`
	Theme          string `json:"theme"`
	AccentColor    string `json:"accent_color"`
	SearchEngine   string `json:"search_engine"`
	Background     string `json:"background"`
	Language       string `json:"language"`
	Layout         string `json:"layout"`
	CustomCSS      string `json:"custom_css"`
	BackgroundMode string `json:"background_mode"` // aurora, time, weather, image
}

// GetUserPreferences returns preferences for a profile (defaults if not found).
func GetUserPreferences(profile string) (*UserPreferences, error) {
	p := &UserPreferences{
		Profile:        profile,
		Theme:          "dark",
		AccentColor:    "#6366f1",
		SearchEngine:   "https://duckduckgo.com/",
		Background:     "aurora",
		Language:       "de",
		Layout:         "grid",
		BackgroundMode: "aurora",
	}
	row := DB.QueryRow(`SELECT theme, accent_color, search_engine, background, language, layout, COALESCE(custom_css,''), COALESCE(background_mode,'aurora')
		FROM user_preferences WHERE profile = ?`, profile)
	err := row.Scan(&p.Theme, &p.AccentColor, &p.SearchEngine, &p.Background, &p.Language, &p.Layout, &p.CustomCSS, &p.BackgroundMode)
	if err == sql.ErrNoRows {
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// SetUserPreferences upserts preferences for a profile.
func SetUserPreferences(profile string, prefs UserPreferences) error {
	_, err := DB.Exec(`INSERT INTO user_preferences (profile, theme, accent_color, search_engine, background, language, layout, custom_css, background_mode)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile) DO UPDATE SET
			theme = excluded.theme,
			accent_color = excluded.accent_color,
			search_engine = excluded.search_engine,
			background = excluded.background,
			language = excluded.language,
			layout = excluded.layout,
			custom_css = excluded.custom_css,
			background_mode = excluded.background_mode`,
		profile, prefs.Theme, prefs.AccentColor, prefs.SearchEngine,
		prefs.Background, prefs.Language, prefs.Layout, prefs.CustomCSS, prefs.BackgroundMode)
	return err
}

// GetWeatherCache returns the cached WeatherCache for a weather widget (nil if none).
func GetWeatherCache(widgetID int) (*WeatherCache, error) {
	row := DB.QueryRow(`SELECT data FROM widget_cache WHERE widget_id = ?`, widgetID)
	var data string
	if err := row.Scan(&data); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var cache WeatherCache
	if err := json.Unmarshal([]byte(data), &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

// ShortURL represents a shortened URL entry.
type ShortURL struct {
	Code      string
	URL       string
	Clicks    int
	CreatedAt string
}

// CreateShortURL inserts a new short URL entry.
func CreateShortURL(code, url string) error {
	_, err := DB.Exec(`INSERT INTO short_urls (code, url) VALUES (?, ?)`, code, url)
	return err
}

// GetShortURL returns the ShortURL for the given code, or nil if not found.
func GetShortURL(code string) (*ShortURL, error) {
	row := DB.QueryRow(`SELECT code, url, clicks, created_at FROM short_urls WHERE code = ?`, code)
	var s ShortURL
	if err := row.Scan(&s.Code, &s.URL, &s.Clicks, &s.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// IncrementClicks increments the click counter for a short URL.
func IncrementClicks(code string) error {
	_, err := DB.Exec(`UPDATE short_urls SET clicks = clicks + 1 WHERE code = ?`, code)
	return err
}

// GetAllShortURLs returns all short URL entries ordered by creation date desc.
func GetAllShortURLs() ([]ShortURL, error) {
	rows, err := DB.Query(`SELECT code, url, clicks, created_at FROM short_urls ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var urls []ShortURL
	for rows.Next() {
		var s ShortURL
		if err := rows.Scan(&s.Code, &s.URL, &s.Clicks, &s.CreatedAt); err != nil {
			return nil, err
		}
		urls = append(urls, s)
	}
	return urls, nil
}

// DeleteShortURL removes a short URL entry.
func DeleteShortURL(code string) error {
	_, err := DB.Exec(`DELETE FROM short_urls WHERE code = ?`, code)
	return err
}

// GetProfiles returns all profiles ordered by sort_order.
func GetProfiles() ([]Profile, error) {
	rows, err := DB.Query(`SELECT id, slug, name, is_default, sort_order FROM profiles ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []Profile
	for rows.Next() {
		var p Profile
		var isDefault int
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &isDefault, &p.SortOrder); err != nil {
			return nil, err
		}
		p.IsDefault = (isDefault == 1)
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// GetDefaultProfile returns the profile with is_default=1.
func GetDefaultProfile() (*Profile, error) {
	row := DB.QueryRow(`SELECT id, slug, name, is_default, sort_order FROM profiles WHERE is_default = 1`)
	var p Profile
	var isDefault int
	if err := row.Scan(&p.ID, &p.Slug, &p.Name, &isDefault, &p.SortOrder); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No default profile found
		}
		return nil, err
	}
	p.IsDefault = (isDefault == 1)
	return &p, nil
}

// GetProfileBySlug returns a profile by its slug.
func GetProfileBySlug(slug string) (*Profile, error) {
	row := DB.QueryRow(`SELECT id, slug, name, is_default, sort_order FROM profiles WHERE slug = ?`, slug)
	var p Profile
	var isDefault int
	if err := row.Scan(&p.ID, &p.Slug, &p.Name, &isDefault, &p.SortOrder); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Profile not found
		}
		return nil, err
	}
	p.IsDefault = (isDefault == 1)
	return &p, nil
}

// AddProfile adds a new profile.
func AddProfile(name, slug string) error {
	_, err := DB.Exec(`INSERT INTO profiles (slug, name, sort_order) VALUES (?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM profiles))`, slug, name)
	return err
}

// DeleteProfile deletes a profile and all associated visibility entries.
// Does not delete if the profile is the default.
func DeleteProfile(slug string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if it's the default profile
	var isDefault int
	err = tx.QueryRow(`SELECT is_default FROM profiles WHERE slug = ?`, slug).Scan(&isDefault)
	if err != nil {
		return err
	}
	if isDefault == 1 {
		return fmt.Errorf("cannot delete default profile")
	}

	// Delete associated visibility entries
	if _, err := tx.Exec(`DELETE FROM visibility WHERE profile = ?`, slug); err != nil {
		return err
	}
	// Delete associated user preferences
	if _, err := tx.Exec(`DELETE FROM user_preferences WHERE profile = ?`, slug); err != nil {
		return err
	}
	// Delete associated user settings (search engines)
	if _, err := tx.Exec(`DELETE FROM user_settings WHERE profile = ?`, slug); err != nil {
		return err
	}
	// Delete associated widgets
	if _, err := tx.Exec(`DELETE FROM widgets WHERE profile = ?`, slug); err != nil {
		return err
	}

	// Delete the profile
	if _, err := tx.Exec(`DELETE FROM profiles WHERE slug = ?`, slug); err != nil {
		return err
	}

	return tx.Commit()
}

// SetDefaultProfile sets a profile as default, unsetting all others.
func SetDefaultProfile(slug string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unset all other defaults
	if _, err := tx.Exec(`UPDATE profiles SET is_default = 0 WHERE is_default = 1`); err != nil {
		return err
	}

	// Set the chosen profile as default
	res, err := tx.Exec(`UPDATE profiles SET is_default = 1 WHERE slug = ?`, slug)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("profile with slug %s not found", slug)
	}

	return tx.Commit()
}

// RecordClick increments the click count for a service+profile pair.
func RecordClick(serviceID int, profile string) error {
	_, err := DB.Exec(`
		INSERT INTO service_clicks (service_id, profile, click_count, last_clicked)
		VALUES (?, ?, 1, datetime('now'))
		ON CONFLICT(service_id, profile) DO UPDATE SET
			click_count = click_count + 1,
			last_clicked = datetime('now')`,
		serviceID, profile)
	return err
}

// GetServiceURL returns the URL of a service by ID.
func GetServiceURL(id int) (string, error) {
	var url string
	err := DB.QueryRow(`SELECT url FROM services WHERE id = ?`, id).Scan(&url)
	return url, err
}

// SetCategorySortMode sets the sort mode for a category.
func SetCategorySortMode(categoryID int, mode string) error {
	if mode != "manual" && mode != "usage" {
		mode = "manual"
	}
	_, err := DB.Exec(`UPDATE categories SET sort_mode = ? WHERE id = ?`, mode, categoryID)
	return err
}

// GetRSSCache returns parsed RSS items from widget cache.
func GetRSSCache(widgetID int) ([]RSSItem, error) {
	cache, err := GetWidgetCache(widgetID)
	if err != nil || cache == nil {
		return nil, err
	}
	return cache.RSSItems, nil
}

// GetTodos returns all todos for a widget, ordered by done then sort_order.
func GetTodos(widgetID int) ([]TodoItem, error) {
	rows, err := DB.Query(
		`SELECT id, widget_id, text, done, due_date, sort_order FROM todos WHERE widget_id = ? ORDER BY done ASC, sort_order ASC, id ASC`,
		widgetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var todos []TodoItem
	for rows.Next() {
		var t TodoItem
		var done int
		if err := rows.Scan(&t.ID, &t.WidgetID, &t.Text, &done, &t.DueDate, &t.SortOrder); err != nil {
			return nil, err
		}
		t.Done = done == 1
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

// AddTodo adds a new todo item for a widget.
func AddTodo(widgetID int, text, dueDate string) (int64, error) {
	res, err := DB.Exec(
		`INSERT INTO todos (widget_id, text, due_date) VALUES (?, ?, ?)`,
		widgetID, text, dueDate,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ToggleTodo flips the done state of a todo.
func ToggleTodo(id int) error {
	_, err := DB.Exec(`UPDATE todos SET done = 1 - done WHERE id = ?`, id)
	return err
}

// DeleteTodo removes a todo item.
func DeleteTodo(id int) error {
	_, err := DB.Exec(`DELETE FROM todos WHERE id = ?`, id)
	return err
}

// GetTodoWidgetID returns the widget_id for a todo (ownership check).
func GetTodoWidgetID(todoID int) (int, error) {
	var wid int
	err := DB.QueryRow(`SELECT widget_id FROM todos WHERE id = ?`, todoID).Scan(&wid)
	return wid, err
}

// GetWidgetByID returns a single widget by ID with fields populated.
func GetWidgetByID(id int) (Widget, error) {
	var w Widget
	row := DB.QueryRow(`SELECT id, type, name, config, profile, sort_order, COALESCE(page_id,0) FROM widgets WHERE id = ?`, id)
	if err := row.Scan(&w.ID, &w.Type, &w.Name, &w.Config, &w.Profile, &w.SortOrder, &w.PageID); err != nil {
		return w, err
	}
	populateWidgetFields(&w)
	return w, nil
}

// AddBookmarkLink appends a link to a bookmarks widget config.
func AddBookmarkLink(widgetID int, link BookmarkLink) error {
	var configStr string
	if err := DB.QueryRow(`SELECT config FROM widgets WHERE id = ?`, widgetID).Scan(&configStr); err != nil {
		return err
	}
	var cfg struct {
		Layout string         `json:"layout"`
		Links  []BookmarkLink `json:"links"`
	}
	_ = json.Unmarshal([]byte(configStr), &cfg)
	if cfg.Layout == "" {
		cfg.Layout = "grid"
	}
	cfg.Links = append(cfg.Links, link)
	newConfig, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	s := string(newConfig)
	return UpdateWidget(widgetID, nil, &s, nil)
}

// DeleteBookmarkLink removes a link by index from a bookmarks widget config.
func DeleteBookmarkLink(widgetID, idx int) error {
	var configStr string
	if err := DB.QueryRow(`SELECT config FROM widgets WHERE id = ?`, widgetID).Scan(&configStr); err != nil {
		return err
	}
	var cfg struct {
		Layout string         `json:"layout"`
		Links  []BookmarkLink `json:"links"`
	}
	_ = json.Unmarshal([]byte(configStr), &cfg)
	if idx < 0 || idx >= len(cfg.Links) {
		return fmt.Errorf("bookmark index out of range")
	}
	cfg.Links = append(cfg.Links[:idx], cfg.Links[idx+1:]...)
	newConfig, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	s := string(newConfig)
	return UpdateWidget(widgetID, nil, &s, nil)
}

// GetNote returns the note content for a widget (empty string if none).
func GetNote(widgetID int) (string, error) {
	var content string
	err := DB.QueryRow(`SELECT content FROM notes WHERE widget_id = ?`, widgetID).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content, err
}

// SaveNote inserts or updates the note content for a widget.
func SaveNote(widgetID int, content string) error {
	_, err := DB.Exec(`
		INSERT INTO notes (widget_id, content, updated_at) VALUES (?, ?, datetime('now'))
		ON CONFLICT(widget_id) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at`,
		widgetID, content)
	return err
}

// GetTopClicks returns the top clicked services ordered by click count.
func GetTopClicks(profile string, limit int) ([]ClickStat, error) {
	var rows *sql.Rows
	var err error
	if profile != "" {
		rows, err = DB.Query(`
			SELECT sc.service_id, s.name, s.url, COALESCE(s.icon,''),
			       sc.click_count, COALESCE(sc.last_clicked,''), sc.profile
			FROM service_clicks sc JOIN services s ON s.id = sc.service_id
			WHERE sc.profile = ?
			ORDER BY sc.click_count DESC LIMIT ?`, profile, limit)
	} else {
		rows, err = DB.Query(`
			SELECT sc.service_id, s.name, s.url, COALESCE(s.icon,''),
			       sc.click_count, COALESCE(sc.last_clicked,''), sc.profile
			FROM service_clicks sc JOIN services s ON s.id = sc.service_id
			ORDER BY sc.click_count DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []ClickStat
	for rows.Next() {
		var s ClickStat
		if err := rows.Scan(&s.ServiceID, &s.ServiceName, &s.ServiceURL, &s.ServiceIcon,
			&s.ClickCount, &s.LastClicked, &s.Profile); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

