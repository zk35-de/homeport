package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration for homeport.
type Config struct {
	Port           string
	DBPath         string
	BackupDir      string
	BackupInterval string
	BackupMaxKeep  int
	AuthEnabled    bool
	PublicProfile  string
	SessionDays    int
}

// Load reads configuration from environment variables and applies defaults.
func Load() *Config {
	port := os.Getenv("HOMEPORT_PORT")
	if port == "" {
		port = "8855"
	}

	dbPath := os.Getenv("HOMEPORT_DB")
	if dbPath == "" {
		dbPath = "./data/homeport.db"
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

	sessionDays := 10
	if s := os.Getenv("HOMEPORT_SESSION_DAYS"); s != "" {
		if fmt.Sscanf(s, "%d", &sessionDays); sessionDays <= 0 {
			sessionDays = 10
		}
	}

	return &Config{
		Port:           port,
		DBPath:         dbPath,
		BackupDir:      backupDir,
		BackupInterval: backupInterval,
		BackupMaxKeep:  backupMaxKeep,
		AuthEnabled:    authEnabled,
		PublicProfile:  publicProfile,
		SessionDays:    sessionDays,
	}
}
