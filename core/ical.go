package core

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"git.zk35.de/secalpha/homeport/internal/db"
	"github.com/emersion/go-ical"
)

// FetchICalEvents fetches and processes iCal events from a given URL.
func FetchICalEvents(url string) ([]db.ICalEvent, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ical url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch ical url: status %s", resp.Status)
	}

	parser := ical.NewDecoder(resp.Body)
	cal, err := parser.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to parse ical body: %w", err)
	}

	var events []db.ICalEvent
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.AddDate(0, 0, 1)

	for _, event := range cal.Events() {
		start, err := event.DateTimeStart(now.Location())
		if err != nil {
			continue // Skip events with invalid start time
		}
		end, err := event.DateTimeEnd(now.Location())
		if err != nil {
			continue // Skip events with invalid end time
		}

		// Skip events that started before today (yesterday and older)
		if start.Before(today) {
			continue
		}

		titleProp := event.Props.Get(ical.PropSummary)
		if titleProp == nil {
			continue
		}

		startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		isToday := startDay.Equal(today)
		isTomorrow := startDay.Equal(tomorrow)

		events = append(events, db.ICalEvent{
			Title:      titleProp.Value,
			Start:      start.Format("2006-01-02 15:04"),
			End:        end.Format("2006-01-02 15:04"),
			IsToday:    isToday,
			IsTomorrow: isTomorrow,
		})
	}

	// Sort events by start date
	sort.Slice(events, func(i, j int) bool {
		return events[i].Start < events[j].Start
	})

	// Limit to max 10 events
	if len(events) > 10 {
		events = events[:10]
	}

	return events, nil
}
