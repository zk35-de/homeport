package core

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"git.zk35.de/secalpha/homeport/internal/db"
	"github.com/emersion/go-ical"
)

// FetchCalDAVEvents fetches iCal events from a CalDAV URL with optional Basic Auth.
func FetchCalDAVEvents(url, username, password string) ([]db.ICalEvent, error) {
	client := http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("caldav: build request: %w", err)
	}
	if username != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("caldav: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("caldav: status %s", resp.Status)
	}

	parser := ical.NewDecoder(resp.Body)
	cal, err := parser.Decode()
	if err != nil {
		return nil, fmt.Errorf("caldav: parse: %w", err)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.AddDate(0, 0, 1)

	var events []db.ICalEvent
	for _, event := range cal.Events() {
		start, err := event.DateTimeStart(now.Location())
		if err != nil {
			continue
		}
		end, err := event.DateTimeEnd(now.Location())
		if err != nil {
			continue
		}
		if start.Before(today) {
			continue
		}
		titleProp := event.Props.Get(ical.PropSummary)
		if titleProp == nil {
			continue
		}
		startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		events = append(events, db.ICalEvent{
			Title:      titleProp.Value,
			Start:      start.Format("2006-01-02 15:04"),
			End:        end.Format("2006-01-02 15:04"),
			IsToday:    startDay.Equal(today),
			IsTomorrow: startDay.Equal(tomorrow),
		})
	}

	sort.Slice(events, func(i, j int) bool { return events[i].Start < events[j].Start })
	if len(events) > 10 {
		events = events[:10]
	}
	return events, nil
}
