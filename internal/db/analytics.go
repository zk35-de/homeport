package db

import (
	"database/sql"
	"fmt"
)

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

// CloneServicesToProfile copies all services visible to srcProfile into dstProfile.
// Services in categories with color 'cyan' are excluded (admin-only convention).
// Returns counts of added and skipped services.
func CloneServicesToProfile(srcProfile, dstProfile string) (int, int, error) {
	tx, err := DB.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT s.id FROM services s
		JOIN visibility v ON v.service_id = s.id
		JOIN categories c ON c.id = s.category_id
		WHERE v.profile = ? AND c.color != 'cyan'`, srcProfile)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	added, skipped := 0, 0
	for rows.Next() {
		var serviceID int
		if err := rows.Scan(&serviceID); err != nil {
			return 0, 0, err
		}
		var exists int
		err := tx.QueryRow(`SELECT COUNT(*) FROM visibility WHERE service_id = ? AND profile = ?`, serviceID, dstProfile).Scan(&exists)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to check visibility for service %d and profile %s: %w", serviceID, dstProfile, err)
		}
		if exists > 0 {
			skipped++
			continue
		}
		_, err = tx.Exec(`INSERT INTO visibility (service_id, profile) VALUES (?, ?)`, serviceID, dstProfile)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to insert visibility for service %d and profile %s: %w", serviceID, dstProfile, err)
		}
		added++
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return added, skipped, nil
}
