package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

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

// AddDiscoveryItem adds a new discovery item if it doesn't already exist.
func AddDiscoveryItem(containerID, suggested string) error {
	_, err := DB.Exec(`INSERT OR IGNORE INTO discovery_inbox (container_id, suggested) VALUES (?, ?)`, containerID, suggested)
	return err
}


// IgnoreDiscoveryItem sets the ignored flag for a discovery item.
func IgnoreDiscoveryItem(id int) error {
	_, err := DB.Exec(`UPDATE discovery_inbox SET ignored = 1 WHERE id = ?`, id)
	return err
}

// AcceptDiscoveryItem reads an item, creates a service from it, then deletes the item.
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

	var svc SuggestedService
	if err := json.Unmarshal([]byte(suggestedJSON), &svc); err != nil {
		return fmt.Errorf("failed to unmarshal suggested service for item %d: %w", id, err)
	}

	categoryID := 1
	profs, _ := GetProfiles()
	profiles := make([]string, len(profs))
	for i, p := range profs {
		profiles[i] = p.Slug
	}

	var catID int
	err = tx.QueryRow(`SELECT id FROM categories WHERE name = ?`, svc.Category).Scan(&catID)
	if err == sql.ErrNoRows {
		res, err := tx.Exec(`INSERT INTO categories (name, layout, color) VALUES (?, ?, ?)`, svc.Category, "tiles", "indigo")
		if err != nil {
			return fmt.Errorf("failed to create category %s: %w", svc.Category, err)
		}
		lastID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get last insert ID for category: %w", err)
		}
		categoryID = int(lastID)
	} else if err != nil {
		return fmt.Errorf("failed to query category %s: %w", svc.Category, err)
	} else {
		categoryID = catID
	}

	res, err := tx.Exec(`INSERT INTO services (category_id, name, url, icon, description, status_check, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM services WHERE category_id = ?))`,
		categoryID, svc.Name, svc.URL, svc.Icon, svc.Description, svc.StatusCheck, categoryID)
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

