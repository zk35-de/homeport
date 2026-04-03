// seed: clear all user data and insert demo entries for README screenshots
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/zk35-de/homeport/internal/db"
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
		{"External", "gray", "tiles"},
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
	// alive: 1=green glow, 0=red glow, -1=no_check (no glow)
	type svc struct {
		cat   string
		name  string
		url   string
		icon  string
		desc  string
		alive int // 1=up, 0=down, -1=no_check
	}
	services := []svc{
		// fake homelab URLs – no_check=1 (no glow, honest)
		{"Media", "Jellyfin", "http://jellyfin.home:8096", "🎬", "Media server", -1},
		{"Media", "Navidrome", "http://navidrome.home:4533", "🎵", "Music streaming", -1},
		{"Media", "Kavita", "http://kavita.home:5000", "📚", "Comics & books", -1},
		{"Media", "Immich", "http://immich.home:2283", "📷", "Photo backup", -1},
		{"Infrastructure", "Nginx Proxy Manager", "http://npm.home:81", "🔀", "Reverse proxy", -1},
		{"Infrastructure", "Portainer", "http://portainer.home:9000", "🐳", "Container management", -1},
		{"Infrastructure", "Pi-hole", "http://pihole.home/admin", "🕳️", "DNS & ad blocking", -1},
		{"Infrastructure", "Vaultwarden", "http://vault.home:8080", "🔐", "Password manager", -1},
		{"Home Automation", "Home Assistant", "http://homeassistant.home:8123", "🏠", "Smart home", -1},
		{"Home Automation", "Zigbee2MQTT", "http://zigbee.home:8080", "📡", "Zigbee bridge", -1},
		{"Home Automation", "Node-RED", "http://nodered.home:1880", "🔴", "Automation flows", -1},
		{"Development", "Gitea", "http://git.home:3000", "🐙", "Git hosting", -1},
		{"Development", "Drone CI", "http://ci.home:8000", "⚙️", "CI/CD", -1},
		{"Development", "VS Code Server", "http://code.home:8443", "💻", "Browser IDE", -1},
		{"Monitoring", "Grafana", "http://grafana.home:3000", "📊", "Dashboards", -1},
		{"Monitoring", "Prometheus", "http://prometheus.home:9090", "🔥", "Metrics", -1},
		{"Monitoring", "Uptime Kuma", "http://uptime.home:3001", "📈", "Status monitoring", -1},
		// real URLs – no_check=0, checker confirms green; one red demo
		{"External", "GitHub", "https://github.com", "🐙", "Code hosting", 1},
		{"External", "Docker Hub", "https://hub.docker.com", "🐳", "Container registry", 1},
		{"External", "Let's Encrypt", "https://letsencrypt.org", "🔒", "Free TLS certificates", 1},
		{"External", "Cloudflare", "https://dash.cloudflare.com", "☁️", "DNS & CDN", 1},
		{"External", "Offline Service", "http://10.0.0.254:9999", "❌", "Unreachable demo", 0},
	}

	for i, s := range services {
		catID := catIDs[s.cat]
		noCheck := 0
		if s.alive == -1 {
			noCheck = 1
		}
		res, err := db.DB.Exec(
			`INSERT INTO services (category_id, name, url, icon, description, sort_order, no_check) VALUES (?,?,?,?,?,?,?)`,
			catID, s.name, s.url, s.icon, s.desc, i, noCheck,
		)
		if err != nil {
			log.Fatalf("insert service %s: %v", s.name, err)
		}
		svcID, _ := res.LastInsertId()

		db.DB.Exec(`INSERT INTO visibility (service_id, profile) VALUES (?,?)`, svcID, "default")

		// insert status so glow renders immediately
		if s.alive >= 0 {
			db.DB.Exec(
				`INSERT INTO service_status (service_id, alive, last_check) VALUES (?, ?, datetime('now'))`,
				svcID, s.alive,
			)
		}
	}

	// ── Preferences ────────────────────────────────────────────────────
	db.DB.Exec(`INSERT OR REPLACE INTO user_preferences (profile, theme, accent_color, search_engine, language)
		VALUES ('default','dark','#6366f1','https://duckduckgo.com/','en')`)

	fmt.Println("✓ Demo data seeded")
}
