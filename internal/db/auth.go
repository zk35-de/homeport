package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// UserAuth holds auth data for a profile.
type UserAuth struct {
	Profile   string
	IsAdmin   bool
	CreatedAt string
}

// SetPassword sets (or updates) the bcrypt password for a profile.
// The first user to get a password is automatically made admin.
func SetPassword(profile, plaintext string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Check if this is the first user_auth entry → make admin
	var count int
	DB.QueryRow(`SELECT COUNT(*) FROM user_auth`).Scan(&count)
	isAdmin := 0
	if count == 0 {
		isAdmin = 1
	}

	_, err = DB.Exec(`
		INSERT INTO user_auth (profile, password, is_admin, updated_at)
		VALUES (?, ?, ?, datetime('now'))
		ON CONFLICT(profile) DO UPDATE SET
			password   = excluded.password,
			updated_at = excluded.updated_at`,
		profile, string(hash), isAdmin)
	return err
}

// CheckPassword returns true if plaintext matches the stored hash for profile.
func CheckPassword(profile, plaintext string) bool {
	var hash string
	err := DB.QueryRow(`SELECT password FROM user_auth WHERE profile = ?`, profile).Scan(&hash)
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}

// GetUserAuth returns auth info for a profile, or nil if no password is set.
func GetUserAuth(profile string) (*UserAuth, error) {
	var a UserAuth
	var isAdmin int
	err := DB.QueryRow(`SELECT profile, is_admin, created_at FROM user_auth WHERE profile = ?`, profile).
		Scan(&a.Profile, &isAdmin, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.IsAdmin = isAdmin == 1
	return &a, nil
}

// GetAllUserAuth returns auth info for all profiles that have a password set.
func GetAllUserAuth() ([]UserAuth, error) {
	rows, err := DB.Query(`SELECT profile, is_admin, created_at FROM user_auth ORDER BY profile`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []UserAuth
	for rows.Next() {
		var a UserAuth
		var isAdmin int
		if err := rows.Scan(&a.Profile, &isAdmin, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.IsAdmin = isAdmin == 1
		result = append(result, a)
	}
	return result, nil
}

// SetAdmin sets or clears admin flag for a profile.
func SetAdmin(profile string, admin bool) error {
	val := 0
	if admin {
		val = 1
	}
	_, err := DB.Exec(`UPDATE user_auth SET is_admin = ? WHERE profile = ?`, val, profile)
	return err
}

// DeleteUserAuth removes auth entry (password) for a profile.
func DeleteUserAuth(profile string) error {
	_, err := DB.Exec(`DELETE FROM user_auth WHERE profile = ?`, profile)
	return err
}

// HasAnyPassword returns true if at least one profile has a password set.
func HasAnyPassword() bool {
	var count int
	DB.QueryRow(`SELECT COUNT(*) FROM user_auth`).Scan(&count)
	return count > 0
}

// --- Sessions ---

// CreateSession generates a new session token for a profile with given TTL.
func CreateSession(profile string, days int) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	expires := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)
	_, err := DB.Exec(`INSERT INTO sessions (token, profile, expires_at) VALUES (?, ?, ?)`,
		token, profile, expires.Format("2006-01-02 15:04:05"))
	if err != nil {
		return "", err
	}
	return token, nil
}

// GetSession returns the profile for a valid (non-expired) session token.
// Returns "" if not found or expired.
func GetSession(token string) string {
	var profile string
	err := DB.QueryRow(`
		SELECT profile FROM sessions
		WHERE token = ? AND expires_at > datetime('now')`, token).Scan(&profile)
	if err != nil {
		return ""
	}
	return profile
}

// SessionInfo holds session data including expiry.
type SessionInfo struct {
	Profile   string
	ExpiresAt time.Time
}

// GetSessionInfo returns session info for a valid (non-expired) token.
// Returns nil if not found or expired.
func GetSessionInfo(token string) *SessionInfo {
	var profile, expiresStr string
	err := DB.QueryRow(`
		SELECT profile, expires_at FROM sessions
		WHERE token = ? AND expires_at > datetime('now')`, token).Scan(&profile, &expiresStr)
	if err != nil {
		return nil
	}
	expires, err := time.Parse("2006-01-02 15:04:05", expiresStr)
	if err != nil {
		return &SessionInfo{Profile: profile}
	}
	return &SessionInfo{Profile: profile, ExpiresAt: expires.UTC()}
}

// ExtendSession updates the expiry of an existing session.
func ExtendSession(token string, days int) error {
	expires := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)
	_, err := DB.Exec(`UPDATE sessions SET expires_at = ? WHERE token = ?`,
		expires.Format("2006-01-02 15:04:05"), token)
	return err
}

// DeleteSession removes a session (logout).
func DeleteSession(token string) error {
	_, err := DB.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// PurgeExpiredSessions deletes all expired sessions.
func PurgeExpiredSessions() error {
	_, err := DB.Exec(`DELETE FROM sessions WHERE expires_at <= datetime('now')`)
	return err
}
