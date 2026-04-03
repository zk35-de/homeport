package db

import (
	"database/sql"
	"fmt"
)

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

func GetDefaultProfile() (*Profile, error) {
	row := DB.QueryRow(`SELECT id, slug, name, is_default, sort_order FROM profiles WHERE is_default = 1`)
	var p Profile
	var isDefault int
	if err := row.Scan(&p.ID, &p.Slug, &p.Name, &isDefault, &p.SortOrder); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	p.IsDefault = (isDefault == 1)
	return &p, nil
}

func GetProfileBySlug(slug string) (*Profile, error) {
	row := DB.QueryRow(`SELECT id, slug, name, is_default, sort_order FROM profiles WHERE slug = ?`, slug)
	var p Profile
	var isDefault int
	if err := row.Scan(&p.ID, &p.Slug, &p.Name, &isDefault, &p.SortOrder); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	p.IsDefault = (isDefault == 1)
	return &p, nil
}

func AddProfile(name, slug string) error {
	_, err := DB.Exec(`INSERT INTO profiles (slug, name, sort_order) VALUES (?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM profiles))`, slug, name)
	return err
}

func DeleteProfile(slug string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var isDefault int
	err = tx.QueryRow(`SELECT is_default FROM profiles WHERE slug = ?`, slug).Scan(&isDefault)
	if err != nil {
		return err
	}
	if isDefault == 1 {
		return fmt.Errorf("cannot delete default profile")
	}

	if _, err := tx.Exec(`DELETE FROM visibility WHERE profile = ?`, slug); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_preferences WHERE profile = ?`, slug); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_settings WHERE profile = ?`, slug); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM widgets WHERE profile = ?`, slug); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM profiles WHERE slug = ?`, slug); err != nil {
		return err
	}

	return tx.Commit()
}

func SetDefaultProfile(slug string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE profiles SET is_default = 0 WHERE is_default = 1`); err != nil {
		return err
	}

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

func AddPage(profile, name, icon string) (int64, error) {
	res, err := DB.Exec(`INSERT INTO pages (profile, name, icon, sort_order) VALUES (?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM pages WHERE profile = ?))`, profile, name, icon, profile)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetPage(id int) (*Page, error) {
	row := DB.QueryRow(`SELECT id, profile, name, icon, sort_order FROM pages WHERE id = ?`, id)
	var p Page
	if err := row.Scan(&p.ID, &p.Profile, &p.Name, &p.Icon, &p.SortOrder); err != nil {
		return nil, err
	}
	return &p, nil
}

func DeletePage(id int) error {
	_, err := DB.Exec(`DELETE FROM pages WHERE id = ?`, id)
	return err
}

func UpdatePage(id int, name, icon string) error {
	_, err := DB.Exec(`UPDATE pages SET name=?, icon=? WHERE id=?`, name, icon, id)
	return err
}

func UpdatePageSort(id, sortOrder int) error {
	_, err := DB.Exec(`UPDATE pages SET sort_order = ? WHERE id = ?`, sortOrder, id)
	return err
}

func GetUserPreferences(profile string) (*UserPreferences, error) {
	p := &UserPreferences{
		Profile:        profile,
		Theme:          "dark",
		AccentColor:    "#6366f1",
		SearchEngine:   "https://duckduckgo.com/",
		Language:       "de",


	}
	row := DB.QueryRow(`SELECT theme, accent_color, search_engine, language, COALESCE(custom_css,'')
		FROM user_preferences WHERE profile = ?`, profile)
	err := row.Scan(&p.Theme, &p.AccentColor, &p.SearchEngine, &p.Language, &p.CustomCSS)
	if err == sql.ErrNoRows {
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func SetUserPreferences(profile string, prefs UserPreferences) error {
	_, err := DB.Exec(`INSERT INTO user_preferences (profile, theme, accent_color, search_engine, language, custom_css)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile) DO UPDATE SET
			theme = excluded.theme,
			accent_color = excluded.accent_color,
			search_engine = excluded.search_engine,
			language = excluded.language,
			custom_css = excluded.custom_css`,
		profile, prefs.Theme, prefs.AccentColor, prefs.SearchEngine,
		prefs.Language, prefs.CustomCSS)
	return err
}

