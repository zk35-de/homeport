package db

import (
	"database/sql"
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

func InitDB(dbPath string) error {
	if dbPath == "" {
		dbPath = "./data/homeport.db"
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	var err error
	DB, err = sql.Open("sqlite", dbPath)
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
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
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
