package mcp

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type AccountSettingsRecord struct {
	AccountID    string
	PasswordHash string
	Role         string
	CreatedAt    string
	UpdatedAt    string
	SettingsJSON string
}

func DBReady() bool {
	return db != nil
}

func ListAccountsWithSettings() ([]AccountSettingsRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	rows, err := db.Query(`
		SELECT
			a.id,
			a.password_hash,
			a.role,
			COALESCE(a.created_at, ''),
			COALESCE(a.updated_at, ''),
			COALESCE(s.settings_json, '{}')
		FROM accounts a
		LEFT JOIN user_settings s ON s.account_id = a.id
		ORDER BY a.id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts: %w", err)
	}
	defer rows.Close()

	records := make([]AccountSettingsRecord, 0)
	for rows.Next() {
		var rec AccountSettingsRecord
		if err := rows.Scan(
			&rec.AccountID,
			&rec.PasswordHash,
			&rec.Role,
			&rec.CreatedAt,
			&rec.UpdatedAt,
			&rec.SettingsJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan account row: %w", err)
		}
		rec.AccountID = strings.TrimSpace(rec.AccountID)
		rec.Role = strings.TrimSpace(rec.Role)
		rec.CreatedAt = strings.TrimSpace(rec.CreatedAt)
		rec.UpdatedAt = strings.TrimSpace(rec.UpdatedAt)
		if strings.TrimSpace(rec.SettingsJSON) == "" {
			rec.SettingsJSON = "{}"
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate account rows: %w", err)
	}

	return records, nil
}

func ReplaceAccountsWithSettings(records []AccountSettingsRecord) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin account sync transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec(`DELETE FROM user_settings`); err != nil {
		return fmt.Errorf("failed to clear user_settings: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM accounts`); err != nil {
		return fmt.Errorf("failed to clear accounts: %w", err)
	}

	accountStmt, err := tx.Prepare(`
		INSERT INTO accounts (id, password_hash, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare account insert: %w", err)
	}
	defer accountStmt.Close()

	settingsStmt, err := tx.Prepare(`
		INSERT INTO user_settings (account_id, settings_json, updated_at)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare user_settings insert: %w", err)
	}
	defer settingsStmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, rec := range records {
		accountID := strings.TrimSpace(rec.AccountID)
		if accountID == "" {
			continue
		}
		role := strings.TrimSpace(rec.Role)
		if role == "" {
			role = "user"
		}
		createdAt := strings.TrimSpace(rec.CreatedAt)
		if createdAt == "" {
			createdAt = now
		}
		updatedAt := strings.TrimSpace(rec.UpdatedAt)
		if updatedAt == "" {
			updatedAt = createdAt
		}
		settingsJSON := strings.TrimSpace(rec.SettingsJSON)
		if settingsJSON == "" {
			settingsJSON = "{}"
		}

		if _, err := accountStmt.Exec(accountID, rec.PasswordHash, role, createdAt, updatedAt); err != nil {
			return fmt.Errorf("failed to insert account %s: %w", accountID, err)
		}
		if _, err := settingsStmt.Exec(accountID, settingsJSON, updatedAt); err != nil {
			return fmt.Errorf("failed to insert settings for %s: %w", accountID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit account sync transaction: %w", err)
	}
	tx = nil
	return nil
}

func GetAccountSettingsJSON(accountID string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database is not initialized")
	}

	var settings sql.NullString
	err := db.QueryRow(`SELECT settings_json FROM user_settings WHERE account_id = ?`, strings.TrimSpace(accountID)).Scan(&settings)
	if err != nil {
		if err == sql.ErrNoRows {
			return "{}", nil
		}
		return "", fmt.Errorf("failed to load user settings for %s: %w", accountID, err)
	}
	if !settings.Valid || strings.TrimSpace(settings.String) == "" {
		return "{}", nil
	}
	return settings.String, nil
}
