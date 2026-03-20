package core_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"log"
	"testing"
	"time"

	"git.zk35.de/secalpha/homeport/core"
)

func icalTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

func buildMockICAL() string {
	now := time.Now().UTC()
	stamp := icalTime(now)
	// Events relative to now so tests don't break based on time of day
	todayFuture := now.Add(2 * time.Hour)    // 2h from now → future, today
	tomorrowMid := now.Add(26 * time.Hour)   // 26h from now → tomorrow
	dayAfter := now.Add(50 * time.Hour)      // 50h from now → day after tomorrow
	pastYesterday := now.Add(-25 * time.Hour) // yesterday → past

	return fmt.Sprintf(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:event1@test
DTSTAMP:%s
DTSTART:%s
DTEND:%s
SUMMARY:Today's Meeting
END:VEVENT
BEGIN:VEVENT
UID:event2@test
DTSTAMP:%s
DTSTART:%s
DTEND:%s
SUMMARY:Tomorrow's Presentation
END:VEVENT
BEGIN:VEVENT
UID:event3@test
DTSTAMP:%s
DTSTART:%s
DTEND:%s
SUMMARY:Past Event
END:VEVENT
BEGIN:VEVENT
UID:event4@test
DTSTAMP:%s
DTSTART:%s
DTEND:%s
SUMMARY:Day After Tomorrow
END:VEVENT
END:VCALENDAR`,
		stamp, icalTime(todayFuture), icalTime(todayFuture.Add(time.Hour)),
		stamp, icalTime(tomorrowMid), icalTime(tomorrowMid.Add(time.Hour)),
		stamp, icalTime(pastYesterday), icalTime(pastYesterday.Add(time.Hour)),
		stamp, icalTime(dayAfter), icalTime(dayAfter.Add(time.Hour)),
	)
}

func TestFetchICalEvents(t *testing.T) {
	log.SetOutput(os.Stderr)

	mockICAL := buildMockICAL()

	// Create a mock HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test.ics" {
			w.Header().Set("Content-Type", "text/calendar")
			fmt.Fprint(w, mockICAL)
		} else if r.URL.Path == "/empty.ics" {
			w.Header().Set("Content-Type", "text/calendar")
			fmt.Fprint(w, `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Example Corp//NONSGML Events//EN
END:VCALENDAR`)
		} else if r.URL.Path == "/invalid.ics" {
			w.Header().Set("Content-Type", "text/calendar")
			fmt.Fprint(w, `THIS IS NOT ICAL DATA`)
		} else {
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer ts.Close()

	// Test case 1: Successful fetch and parsing
	t.Run("successful fetch", func(t *testing.T) {
		events, err := core.FetchICalEvents(ts.URL + "/test.ics")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(events) != 3 { // Past events should be filtered out. Day after tomorrow is included.
			t.Fatalf("Expected 3 events, got %d", len(events))
		}

		// Events should be sorted by start time
		if events[0].Title != "Today's Meeting" {
			t.Errorf("Expected first event to be 'Today's Meeting', got '%s'", events[0].Title)
		}
		if events[0].IsToday != true {
			t.Errorf("Expected 'Today's Meeting' to be IsToday=true")
		}
		if events[1].Title != "Tomorrow's Presentation" {
			t.Errorf("Expected second event to be 'Tomorrow's Presentation', got '%s'", events[1].Title)
		}
		if events[1].IsTomorrow != true {
			t.Errorf("Expected 'Tomorrow's Presentation' to be IsTomorrow=true")
		}
		if events[2].Title != "Day After Tomorrow" {
			t.Errorf("Expected third event to be 'Day After Tomorrow', got '%s'", events[2].Title)
		}
		if events[2].IsToday != false || events[2].IsTomorrow != false {
			t.Errorf("Expected 'Day After Tomorrow' to be IsToday=false and IsTomorrow=false")
		}
	})

	// Test case 2: Empty iCal
	t.Run("empty ical", func(t *testing.T) {
		events, err := core.FetchICalEvents(ts.URL + "/empty.ics")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(events) != 0 {
			t.Fatalf("Expected 0 events, got %d", len(events))
		}
	})

	// Test case 3: Invalid iCal content
	t.Run("invalid ical content", func(t *testing.T) {
		events, err := core.FetchICalEvents(ts.URL + "/invalid.ics")
		if err == nil {
			t.Fatalf("Expected error for invalid iCal content, got nil")
		}
		if events != nil {
			t.Fatalf("Expected nil events for invalid iCal content, got %v", events)
		}
	})

	// Test case 4: Non-existent URL
	t.Run("non-existent url", func(t *testing.T) {
		events, err := core.FetchICalEvents(ts.URL + "/nonexistent.ics")
		if err == nil {
			t.Fatalf("Expected error for non-existent URL, got nil")
		}
		if events != nil {
			t.Fatalf("Expected nil events for non-existent URL, got %v", events)
		}
	})

	// Test case 5: Network error (e.g., unreachable host)
	t.Run("network error", func(t *testing.T) {
		events, err := core.FetchICalEvents("http://localhost:9999/unreachable.ics")
		if err == nil {
			t.Fatalf("Expected error for network unreachable, got nil")
		}
		if events != nil {
			t.Fatalf("Expected nil events for network unreachable, got %v", events)
		}
	})

	// Test case 6: Event limit (more than 10 events)
	t.Run("event limit", func(t *testing.T) {
		longICAL := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Example Corp//NONSGML Events//EN
`
		for i := 1; i <= 15; i++ {
			eventTime := time.Now().Add(time.Duration(i) * time.Hour)
			longICAL += fmt.Sprintf(`BEGIN:VEVENT
UID:event%d@example.com
DTSTAMP:%s
DTSTART:%s
DTEND:%s
SUMMARY:Event %d
END:VEVENT
`, i, eventTime.Format("20060102T150405Z"), eventTime.Format("20060102T150405Z"), eventTime.Add(time.Hour).Format("20060102T150405Z"), i)
		}
		longICAL += `END:VCALENDAR`

		tsLong := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/calendar")
			fmt.Fprint(w, longICAL)
		}))
		defer tsLong.Close()

		events, err := core.FetchICalEvents(tsLong.URL)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(events) != 10 {
			t.Fatalf("Expected 10 events (due to limit), got %d", len(events))
		}
		if events[0].Title != "Event 1" { // Ensure sorting and then truncation
			t.Errorf("Expected first event to be 'Event 1', got '%s'", events[0].Title)
		}
		if events[9].Title != "Event 10" {
			t.Errorf("Expected last event to be 'Event 10', got '%s'", events[9].Title)
		}
	})
}
