package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// Config holds all runtime configuration for homeport.
type Config struct {
	Port           string
	DBPath         string
	Token          string
	CORS           []string
	BackupDir      string
	BackupInterval string
	BackupMaxKeep  int
	AuthEnabled    bool
	PublicProfile  string
	SessionDays    int
}

// Load reads configuration from environment variables and applies defaults.
// If HOMEPORT_TOKEN is empty, a random 32-byte hex token is generated and
// printed to stderr so the operator can record it.
func Load() *Config {
	port := os.Getenv("HOMEPORT_PORT")
	if port == "" {
		port = "8855"
	}

	dbPath := os.Getenv("HOMEPORT_DB")
	if dbPath == "" {
		dbPath = "./data/homeport.db"
	}

	token := os.Getenv("HOMEPORT_TOKEN")
	if token == "" {
		token = generateToken()
		fmt.Fprintf(os.Stderr, "HOMEPORT_TOKEN not set – generated token: %s\n", token)
	}

	rawCORS := os.Getenv("HOMEPORT_CORS")
	var cors []string
	if rawCORS == "" {
		cors = []string{"*"}
	} else {
		for _, s := range strings.Split(rawCORS, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				cors = append(cors, s)
			}
		}
	}

	backupDir := os.Getenv("HOMEPORT_BACKUP_DIR")
	if backupDir == "" {
		backupDir = "./data/backups"
	}

	backupInterval := os.Getenv("HOMEPORT_BACKUP_INTERVAL")

	backupMaxKeepStr := os.Getenv("HOMEPORT_BACKUP_MAX_KEEP")
	backupMaxKeep := 7
	if backupMaxKeepStr != "" {
		if fmt.Sscanf(backupMaxKeepStr, "%d", &backupMaxKeep); backupMaxKeep <= 0 {
			backupMaxKeep = 7
		}
	}

	authEnabled := os.Getenv("HOMEPORT_AUTH") == "true"
	publicProfile := os.Getenv("HOMEPORT_PUBLIC_PROFILE")

	sessionDays := 30
	if s := os.Getenv("HOMEPORT_SESSION_DAYS"); s != "" {
		if fmt.Sscanf(s, "%d", &sessionDays); sessionDays <= 0 {
			sessionDays = 30
		}
	}

	return &Config{
		Port:           port,
		DBPath:         dbPath,
		Token:          token,
		CORS:           cors,
		BackupDir:      backupDir,
		BackupInterval: backupInterval,
		BackupMaxKeep:  backupMaxKeep,
		AuthEnabled:    authEnabled,
		PublicProfile:  publicProfile,
		SessionDays:    sessionDays,
	}
}

func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("config: failed to generate token: %v", err))
	}
	return hex.EncodeToString(b)
}
