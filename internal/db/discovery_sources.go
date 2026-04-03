package db

import "time"

// DiscoverySource is a configured source for auto-discovery.
type DiscoverySource struct {
	ID        int
	Type      string // 'npm', 'docker'
	Name      string
	URL       string
	Token     string // identity:secret for NPM, empty for Docker
	Enabled   bool
	Interval  int // seconds
	CreatedAt time.Time
}

func GetDiscoverySources() ([]DiscoverySource, error) {
	rows, err := DB.Query(`SELECT id, type, name, url, token, enabled, interval, created_at FROM discovery_sources ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sources []DiscoverySource
	for rows.Next() {
		var s DiscoverySource
		var enabled int
		var createdAt string
		if err := rows.Scan(&s.ID, &s.Type, &s.Name, &s.URL, &s.Token, &enabled, &s.Interval, &createdAt); err != nil {
			return nil, err
		}
		s.Enabled = enabled == 1
		s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

func AddDiscoverySource(typ, name, url, token string, interval int) (int64, error) {
	res, err := DB.Exec(
		`INSERT INTO discovery_sources (type, name, url, token, interval) VALUES (?, ?, ?, ?, ?)`,
		typ, name, url, token, interval,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func DeleteDiscoverySource(id int) error {
	_, err := DB.Exec(`DELETE FROM discovery_sources WHERE id=?`, id)
	return err
}

func SetDiscoverySourceEnabled(id int, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	_, err := DB.Exec(`UPDATE discovery_sources SET enabled=? WHERE id=?`, en, id)
	return err
}

// AddDiscoveryItemExt adds a discovery item with external_id + source_id.
// Returns true if a new row was inserted (not a duplicate).
func AddDiscoveryItemExt(externalID, suggested string, sourceID int) (bool, error) {
	res, err := DB.Exec(
		`INSERT OR IGNORE INTO discovery_inbox (container_id, external_id, suggested, source_id) VALUES (?, ?, ?, ?)`,
		externalID, externalID, suggested, sourceID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
