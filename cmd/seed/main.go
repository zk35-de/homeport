// seed: clear all user data and insert demo entries for README screenshots
package main

import (
	"fmt"
	"log"
	"os"

	"git.zk35.de/secalpha/homeport/internal/db"
)

func main() {
	dbPath := "./data/homeport.db"
	if p := os.Getenv("HOMEPORT_DB"); p != "" {
		dbPath = p
	}
	if err := db.InitDB(dbPath); err != nil {
		log.Fatalf("db init: %v", err)
	}

	// ── Wipe user data ─────────────────────────────────────────────────
	tables := []string{
		"service_clicks", "service_status", "visibility",
		"services", "categories", "pages",
		"discovery_inbox", "discovery_sources",
		"user_preferences", "user_auth", "sessions",
	}
	for _, t := range tables {
		if _, err := db.DB.Exec("DELETE FROM " + t); err != nil {
			log.Printf("warn: clear %s: %v", t, err)
		}
	}
	// keep profiles, just reset to one default
	db.DB.Exec("DELETE FROM profiles")

	// ── Profile ────────────────────────────────────────────────────────
	db.DB.Exec(`INSERT INTO profiles (slug, name, is_default, sort_order) VALUES ('default','Default',1,0)`)

	// ── Categories ─────────────────────────────────────────────────────
	type cat struct {
		name   string
		color  string
		layout string
	}
	cats := []cat{
		{"Media", "indigo", "tiles"},
		{"Infrastructure", "blue", "tiles"},
		{"Home Automation", "green", "tiles"},
		{"Development", "purple", "tiles"},
		{"Monitoring", "orange", "tiles"},
	}
	catIDs := map[string]int64{}
	for i, c := range cats {
		res, err := db.DB.Exec(
			`INSERT INTO categories (name, layout, color, sort_order) VALUES (?,?,?,?)`,
			c.name, c.layout, c.color, i,
		)
		if err != nil {
			log.Fatalf("insert category %s: %v", c.name, err)
		}
		id, _ := res.LastInsertId()
		catIDs[c.name] = id
	}

	// ── Services ───────────────────────────────────────────────────────
	type svc struct {
		cat  string
		name string
		url  string
		icon string
		desc string
	}
	services := []svc{
		{"Media", "Jellyfin", "http://jellyfin.home:8096", "/api/favicon?url=http://jellyfin.home:8096", "Media server"},
		{"Media", "Navidrome", "http://navidrome.home:4533", "/api/favicon?url=http://navidrome.home:4533", "Music streaming"},
		{"Media", "Kavita", "http://kavita.home:5000", "/api/favicon?url=http://kavita.home:5000", "Comics & books"},
		{"Media", "Immich", "http://immich.home:2283", "/api/favicon?url=http://immich.home:2283", "Photo backup"},
		{"Infrastructure", "Nginx Proxy Manager", "http://npm.home:81", "/api/favicon?url=http://npm.home:81", "Reverse proxy"},
		{"Infrastructure", "Portainer", "http://portainer.home:9000", "/api/favicon?url=http://portainer.home:9000", "Container management"},
		{"Infrastructure", "Pi-hole", "http://pihole.home/admin", "/api/favicon?url=http://pihole.home/admin", "DNS & ad blocking"},
		{"Infrastructure", "Vaultwarden", "http://vault.home:8080", "/api/favicon?url=http://vault.home:8080", "Password manager"},
		{"Home Automation", "Home Assistant", "http://homeassistant.home:8123", "/api/favicon?url=http://homeassistant.home:8123", "Smart home"},
		{"Home Automation", "Zigbee2MQTT", "http://zigbee.home:8080", "/api/favicon?url=http://zigbee.home:8080", "Zigbee bridge"},
		{"Home Automation", "Node-RED", "http://nodered.home:1880", "/api/favicon?url=http://nodered.home:1880", "Automation flows"},
		{"Development", "Gitea", "http://git.home:3000", "/api/favicon?url=http://git.home:3000", "Git hosting"},
		{"Development", "Drone CI", "http://ci.home:8000", "/api/favicon?url=http://ci.home:8000", "CI/CD"},
		{"Development", "VS Code Server", "http://code.home:8443", "/api/favicon?url=http://code.home:8443", "Browser IDE"},
		{"Monitoring", "Grafana", "http://grafana.home:3000", "/api/favicon?url=http://grafana.home:3000", "Dashboards"},
		{"Monitoring", "Prometheus", "http://prometheus.home:9090", "/api/favicon?url=http://prometheus.home:9090", "Metrics"},
		{"Monitoring", "Uptime Kuma", "http://uptime.home:3001", "/api/favicon?url=http://uptime.home:3001", "Status monitoring"},
	}

	for i, s := range services {
		catID := catIDs[s.cat]
		res, err := db.DB.Exec(
			`INSERT INTO services (category_id, name, url, icon, description, sort_order, no_check) VALUES (?,?,?,?,?,?,1)`,
			catID, s.name, s.url, s.icon, s.desc, i,
		)
		if err != nil {
			log.Fatalf("insert service %s: %v", s.name, err)
		}
		svcID, _ := res.LastInsertId()
		// visibility for default profile
		db.DB.Exec(`INSERT INTO visibility (service_id, profile) VALUES (?,?)`, svcID, "default")
	}

	// ── Preferences ────────────────────────────────────────────────────
	db.DB.Exec(`INSERT OR REPLACE INTO user_preferences (profile, theme, accent_color, search_engine, language)
		VALUES ('default','dark','#6366f1','https://duckduckgo.com/','en')`)

	fmt.Println("✓ Demo data seeded")
}
