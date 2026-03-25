package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

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

func GetWidgetByID(id int) (Widget, error) {
	var w Widget
	row := DB.QueryRow(`SELECT id, type, name, config, profile, sort_order, COALESCE(page_id,0) FROM widgets WHERE id = ?`, id)
	if err := row.Scan(&w.ID, &w.Type, &w.Name, &w.Config, &w.Profile, &w.SortOrder, &w.PageID); err != nil {
		return w, err
	}
	populateWidgetFields(&w)
	return w, nil
}

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

func UpdateWidgetSort(id, sortOrder int) error {
	_, err := DB.Exec(`UPDATE widgets SET sort_order = ? WHERE id = ?`, sortOrder, id)
	return err
}

func SetWidgetPage(widgetID, pageID int) error {
	if pageID == 0 {
		_, err := DB.Exec(`UPDATE widgets SET page_id = NULL WHERE id = ?`, widgetID)
		return err
	}
	_, err := DB.Exec(`UPDATE widgets SET page_id = ? WHERE id = ?`, pageID, widgetID)
	return err
}

func GetWidgetCache(widgetID int) (*WidgetCacheEntry, error) {
	row := DB.QueryRow(`SELECT data FROM widget_cache WHERE widget_id = ?`, widgetID)
	var data string
	if err := row.Scan(&data); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var entry WidgetCacheEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func GetWidgetCacheRaw(widgetID int, target interface{}) error {
	row := DB.QueryRow(`SELECT data FROM widget_cache WHERE widget_id = ?`, widgetID)
	var data string
	if err := row.Scan(&data); err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), target)
}

func UpdateWidgetCache(widgetID int, data string) error {
	_, err := DB.Exec(`INSERT INTO widget_cache (widget_id, data, fetched_at) VALUES (?, ?, datetime('now')) ON CONFLICT(widget_id) DO UPDATE SET data = ?, fetched_at = datetime('now')`, widgetID, data, data)
	return err
}

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

func ToggleTodo(id int) error {
	_, err := DB.Exec(`UPDATE todos SET done = 1 - done WHERE id = ?`, id)
	return err
}

func DeleteTodo(id int) error {
	_, err := DB.Exec(`DELETE FROM todos WHERE id = ?`, id)
	return err
}

func GetTodoWidgetID(todoID int) (int, error) {
	var wid int
	err := DB.QueryRow(`SELECT widget_id FROM todos WHERE id = ?`, todoID).Scan(&wid)
	return wid, err
}

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

func GetNote(widgetID int) (string, error) {
	var content string
	err := DB.QueryRow(`SELECT content FROM notes WHERE widget_id = ?`, widgetID).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content, err
}

func SaveNote(widgetID int, content string) error {
	_, err := DB.Exec(`
		INSERT INTO notes (widget_id, content, updated_at) VALUES (?, ?, datetime('now'))
		ON CONFLICT(widget_id) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at`,
		widgetID, content)
	return err
}
