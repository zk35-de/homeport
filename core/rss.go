package core

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type RSSItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	PubDate string `json:"pub_date"`
}

// FetchRSSFeed fetches and parses an RSS 2.0 or Atom feed.
func FetchRSSFeed(feedURL string) ([]RSSItem, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(feedURL)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Try RSS 2.0
	type rssItem struct {
		Title   string `xml:"title"`
		Link    string `xml:"link"`
		PubDate string `xml:"pubDate"`
	}
	type rssFeed struct {
		XMLName xml.Name `xml:"rss"`
		Items   []rssItem `xml:"channel>item"`
	}
	var rss rssFeed
	if xml.Unmarshal(body, &rss) == nil && len(rss.Items) > 0 {
		items := make([]RSSItem, 0, len(rss.Items))
		for _, it := range rss.Items {
			items = append(items, RSSItem{
				Title:   strings.TrimSpace(it.Title),
				URL:     strings.TrimSpace(it.Link),
				PubDate: it.PubDate,
			})
		}
		return items, nil
	}

	// Try Atom
	type atomLink struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
	}
	type atomEntry struct {
		Title   string     `xml:"title"`
		Updated string     `xml:"updated"`
		Links   []atomLink `xml:"link"`
	}
	type atomFeed struct {
		XMLName xml.Name    `xml:"feed"`
		Entries []atomEntry `xml:"entry"`
	}
	var atom atomFeed
	if xml.Unmarshal(body, &atom) == nil && len(atom.Entries) > 0 {
		items := make([]RSSItem, 0, len(atom.Entries))
		for _, e := range atom.Entries {
			url := ""
			for _, l := range e.Links {
				if l.Rel == "alternate" || l.Rel == "" {
					url = l.Href
					break
				}
			}
			items = append(items, RSSItem{
				Title:   strings.TrimSpace(e.Title),
				URL:     url,
				PubDate: e.Updated,
			})
		}
		return items, nil
	}

	return nil, fmt.Errorf("unsupported feed format or empty feed")
}
