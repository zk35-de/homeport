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

// CloneToAndrea copies all services visible to 'markus' (except cyan categories) to 'andrea'.
func CloneToAndrea() (added int, skipped int, err error) {
	tx, err := DB.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
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

	rows, err := tx.Query(`
		SELECT s.id
		FROM services s
		JOIN visibility vm ON s.id = vm.service_id
		JOIN categories c ON s.category_id = c.id
		WHERE vm.profile = 'markus' AND c.color != 'cyan'
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
		var exists int
		err := tx.QueryRow(`SELECT COUNT(*) FROM visibility WHERE service_id = ? AND profile = 'andrea'`, serviceID).Scan(&exists)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to check existing visibility for service %d: %w", serviceID, err)
		}
		if exists > 0 {
			skipped++
			continue
		}
		_, err = tx.Exec(`INSERT INTO visibility (service_id, profile) VALUES (?, 'andrea')`, serviceID)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to insert visibility for service %d and andrea: %w", serviceID, err)
		}
		added++
	}

	return added, skipped, nil
}
