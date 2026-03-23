package db

import (
	"database/sql"
)

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

func DeleteCategory(id int) error {
	_, err := DB.Exec(`DELETE FROM categories WHERE id = ?`, id)
	return err
}

func UpdateCategory(id int, name, layout, color string) error {
	_, err := DB.Exec(`UPDATE categories SET name=?, layout=?, color=? WHERE id=?`, name, layout, color, id)
	return err
}

func GetCategory(id int) (*Category, error) {
	row := DB.QueryRow(`SELECT id, name, layout, color, sort_order, COALESCE(col_span,1) FROM categories WHERE id=?`, id)
	var c Category
	if err := row.Scan(&c.ID, &c.Name, &c.Layout, &c.Color, &c.SortOrder, &c.ColSpan); err != nil {
		return nil, err
	}
	return &c, nil
}

func UpdateCategorySort(id, newOrder int) error {
	_, err := DB.Exec(`UPDATE categories SET sort_order = ? WHERE id = ?`, newOrder, id)
	return err
}

func UpdateCategorySpan(id, span int) error {
	if span < 1 || span > 3 {
		span = 1
	}
	_, err := DB.Exec(`UPDATE categories SET col_span=? WHERE id=?`, span, id)
	return err
}

func SetCategorySortMode(categoryID int, mode string) error {
	if mode != "manual" && mode != "usage" {
		mode = "manual"
	}
	_, err := DB.Exec(`UPDATE categories SET sort_mode = ? WHERE id = ?`, mode, categoryID)
	return err
}

func SetCategoryPage(categoryID, pageID int) error {
	if pageID == 0 {
		_, err := DB.Exec(`UPDATE categories SET page_id = NULL WHERE id = ?`, categoryID)
		return err
	}
	_, err := DB.Exec(`UPDATE categories SET page_id = ? WHERE id = ?`, pageID, categoryID)
	return err
}

func ReorderCategories(items []ReorderItem) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, it := range items {
		if _, err := tx.Exec(`UPDATE categories SET sort_order = ? WHERE id = ?`, it.SortOrder, it.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
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

func DeleteService(id int) error {
	_, err := DB.Exec(`DELETE FROM services WHERE id = ?`, id)
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

	if _, err := tx.Exec(`DELETE FROM visibility WHERE service_id=?`, id); err != nil {
		return err
	}
	for _, p := range profiles {
		if _, err := tx.Exec(`INSERT INTO visibility (service_id, profile) VALUES (?, ?)`, id, p); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func GetService(id int) (*Service, error) {
	row := DB.QueryRow(`SELECT id, category_id, name, url, icon, description, status_check, sort_order FROM services WHERE id=?`, id)
	var s Service
	if err := row.Scan(&s.ID, &s.CategoryID, &s.Name, &s.URL, &s.Icon, &s.Description, &s.StatusCheck, &s.SortOrder); err != nil {
		return nil, err
	}
	rows, err := DB.Query(`SELECT profile FROM visibility WHERE service_id=?`, id)
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

func UpdateServiceSort(id, newOrder int) error {
	_, err := DB.Exec(`UPDATE services SET sort_order = ? WHERE id = ?`, newOrder, id)
	return err
}

func ReorderServices(items []ReorderItem) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, it := range items {
		if _, err := tx.Exec(`UPDATE services SET sort_order = ? WHERE id = ?`, it.SortOrder, it.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
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
	rows, err := DB.Query(`SELECT id, url, status_check FROM services`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.URL, &s.StatusCheck); err != nil {
			return nil, err
		}
		if s.StatusCheck == "" {
			s.StatusCheck = s.URL
		}
		services = append(services, s)
	}
	return services, nil
}

func GetServiceURL(id int) (string, error) {
	var url string
	err := DB.QueryRow(`SELECT url FROM services WHERE id = ?`, id).Scan(&url)
	return url, err
}
