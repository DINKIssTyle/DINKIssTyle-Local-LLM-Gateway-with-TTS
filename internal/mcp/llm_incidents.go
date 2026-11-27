package mcp

import (
	"fmt"
	"strings"
	"sync"
)

const maxLLMIncidentsRetained = 1000

var llmIncidentsMu sync.RWMutex

// LLMIncident records an error and recovery attempt across chat and out-of-chat
// LLM calls, segmented by model_id, model_family, scope, and failure_kind.
type LLMIncident struct {
	ID               int64  `json:"id"`
	ModelID          string `json:"model_id"`
	ModelFamily      string `json:"model_family"`
	LLMMode          string `json:"llm_mode"`
	Scope            string `json:"scope"`
	FailureKind      string `json:"failure_kind"`
	RecoveryStrategy string `json:"recovery_strategy"`
	Recovered        bool   `json:"recovered"`
	HTTPStatus       int    `json:"http_status,omitempty"`
	ErrorDetail      string `json:"error_detail,omitempty"`
	RawSnippet       string `json:"raw_snippet,omitempty"`
	ElapsedMS        int64  `json:"elapsed_ms,omitempty"`
	CreatedAt        string `json:"created_at"`
}

// ModelFailureStat aggregates failure counts and recovery rates per model and failure point.
type ModelFailureStat struct {
	ModelID        string `json:"model_id"`
	ModelFamily    string `json:"model_family"`
	LLMMode        string `json:"llm_mode"`
	Scope          string `json:"scope"`
	FailureKind    string `json:"failure_kind"`
	TotalCount     int    `json:"total_count"`
	RecoveredCount int    `json:"recovered_count"`
	LastSeenAt     string `json:"last_seen_at"`
}

func truncateUTF8Runes(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

func RecordLLMIncident(entry LLMIncident) error {
	llmIncidentsMu.Lock()
	defer llmIncidentsMu.Unlock()
	if db == nil {
		return nil
	}
	modelID := truncateUTF8Runes(entry.ModelID, 160)
	family := strings.TrimSpace(entry.ModelFamily)
	if family == "" {
		family = "other"
	}
	mode := strings.TrimSpace(entry.LLMMode)
	if mode == "" {
		mode = "standard"
	}
	scope := truncateUTF8Runes(entry.Scope, 80)
	kind := truncateUTF8Runes(entry.FailureKind, 80)
	strategy := truncateUTF8Runes(entry.RecoveryStrategy, 80)
	if strategy == "" {
		strategy = "none"
	}
	recovered := 0
	if entry.Recovered {
		recovered = 1
	}
	errDetail := truncateUTF8Runes(entry.ErrorDetail, 500)
	rawSnippet := truncateUTF8Runes(entry.RawSnippet, 600)

	_, err := db.Exec(`
		INSERT INTO llm_incidents (
			model_id, model_family, llm_mode, scope, failure_kind,
			recovery_strategy, recovered, http_status, error_detail, raw_snippet, elapsed_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, modelID, family, mode, scope, kind, strategy, recovered, entry.HTTPStatus, errDetail, rawSnippet, entry.ElapsedMS)
	if err != nil {
		return fmt.Errorf("insert llm_incident: %w", err)
	}

	// Rolling retention keeps the SQLite store bounded without user intervention.
	_, _ = db.Exec(`
		DELETE FROM llm_incidents
		WHERE id NOT IN (
			SELECT id FROM llm_incidents ORDER BY id DESC LIMIT ?
		)
	`, maxLLMIncidentsRetained)
	return nil
}

func ListLLMIncidents(modelID string, limit int) ([]LLMIncident, error) {
	llmIncidentsMu.RLock()
	defer llmIncidentsMu.RUnlock()
	if db == nil {
		return []LLMIncident{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	modelID = strings.TrimSpace(modelID)
	var (
		rows interface {
			Next() bool
			Scan(...any) error
			Close() error
			Err() error
		}
		err error
	)
	if modelID != "" {
		rows, err = db.Query(`
			SELECT id, model_id, model_family, llm_mode, scope, failure_kind,
			       recovery_strategy, recovered, http_status, error_detail, raw_snippet, elapsed_ms, created_at
			FROM llm_incidents
			WHERE model_id = ?
			ORDER BY id DESC
			LIMIT ?
		`, modelID, limit)
	} else {
		rows, err = db.Query(`
			SELECT id, model_id, model_family, llm_mode, scope, failure_kind,
			       recovery_strategy, recovered, http_status, error_detail, raw_snippet, elapsed_ms, created_at
			FROM llm_incidents
			ORDER BY id DESC
			LIMIT ?
		`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]LLMIncident, 0, limit)
	for rows.Next() {
		var item LLMIncident
		var rec int
		if err := rows.Scan(
			&item.ID, &item.ModelID, &item.ModelFamily, &item.LLMMode,
			&item.Scope, &item.FailureKind, &item.RecoveryStrategy, &rec,
			&item.HTTPStatus, &item.ErrorDetail, &item.RawSnippet, &item.ElapsedMS, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.Recovered = rec != 0
		out = append(out, item)
	}
	return out, rows.Err()
}

func GetModelFailureSummary(limit int) ([]ModelFailureStat, error) {
	llmIncidentsMu.RLock()
	defer llmIncidentsMu.RUnlock()
	if db == nil {
		return []ModelFailureStat{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := db.Query(`
		SELECT model_id, model_family, llm_mode, scope, failure_kind,
		       COUNT(*) AS total_count,
		       SUM(CASE WHEN recovered != 0 THEN 1 ELSE 0 END) AS recovered_count,
		       MAX(created_at) AS last_seen_at
		FROM llm_incidents
		GROUP BY model_id, model_family, llm_mode, scope, failure_kind
		ORDER BY total_count DESC, last_seen_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ModelFailureStat, 0, limit)
	for rows.Next() {
		var stat ModelFailureStat
		if err := rows.Scan(
			&stat.ModelID, &stat.ModelFamily, &stat.LLMMode, &stat.Scope, &stat.FailureKind,
			&stat.TotalCount, &stat.RecoveredCount, &stat.LastSeenAt,
		); err != nil {
			return nil, err
		}
		out = append(out, stat)
	}
	return out, rows.Err()
}
