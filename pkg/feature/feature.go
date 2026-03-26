package feature

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mshogin/costlint/pkg/pricing"
)

// TokenEntry records a single token usage event within a feature session.
type TokenEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Tokens      int       `json:"tokens"`
	CostUSD     float64   `json:"cost_usd"`
	Description string    `json:"description"`
}

// FeatureSession tracks token usage and cost for a single feature (issue).
type FeatureSession struct {
	IssueID     string       `json:"issue_id"`
	StartTime   time.Time    `json:"start_time"`
	EndTime     *time.Time   `json:"end_time,omitempty"`
	TokensTotal int          `json:"tokens_total"`
	CostTotal   float64      `json:"cost_total"`
	Model       string       `json:"model"`
	Entries     []TokenEntry `json:"entries"`
}

// FeatureSummary is a read-only snapshot of a completed or active session.
type FeatureSummary struct {
	IssueID     string     `json:"issue_id"`
	Model       string     `json:"model"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	TokensTotal int        `json:"tokens_total"`
	CostTotal   float64    `json:"cost_total"`
	EntryCount  int        `json:"entry_count"`
	Active      bool       `json:"active"`
}

// persistRecord is the on-disk format for a completed session stored in the JSONL file.
type persistRecord struct {
	Session FeatureSession `json:"session"`
}

// FeatureTracker manages active sessions (persisted to a state file) and completed
// sessions (appended to a JSONL history file).
type FeatureTracker struct {
	historyPath string // ~/.costlint-features.jsonl  - completed sessions
	statePath   string // ~/.costlint-features-active.json - active sessions map
}

// defaultPaths returns the history and state file paths derived from the base directory.
func defaultPaths() (history, state string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".costlint-features.jsonl", ".costlint-features-active.json"
	}
	return home + "/.costlint-features.jsonl", home + "/.costlint-features-active.json"
}

// NewFeatureTracker creates a tracker backed by the default files.
func NewFeatureTracker() *FeatureTracker {
	h, s := defaultPaths()
	return &FeatureTracker{historyPath: h, statePath: s}
}

// NewFeatureTrackerWithPath creates a tracker backed by given paths (useful for tests).
func NewFeatureTrackerWithPath(historyPath, statePath string) *FeatureTracker {
	return &FeatureTracker{historyPath: historyPath, statePath: statePath}
}

// StartTracking begins a new session for the given issue. Returns an error if the
// issue already has an active session.
func (ft *FeatureTracker) StartTracking(issueID string, model string) error {
	if _, ok := pricing.Models[model]; !ok {
		return fmt.Errorf("unknown model %q; valid values: haiku, sonnet, opus", model)
	}

	active, err := ft.loadActive()
	if err != nil {
		return err
	}
	if _, exists := active[issueID]; exists {
		return fmt.Errorf("session for issue %s is already active", issueID)
	}

	active[issueID] = &FeatureSession{
		IssueID:   issueID,
		StartTime: time.Now().UTC(),
		Model:     model,
		Entries:   []TokenEntry{},
	}
	return ft.saveActive(active)
}

// AddTokens records token usage for an active session. The cost is calculated
// using the model's input pricing (tokens treated as input tokens).
func (ft *FeatureTracker) AddTokens(issueID string, tokens int, description string) error {
	active, err := ft.loadActive()
	if err != nil {
		return err
	}

	sess, ok := active[issueID]
	if !ok {
		return fmt.Errorf("no active session for issue %s; run 'track start' first", issueID)
	}

	cost := pricing.Estimate(sess.Model, tokens, 0)
	entry := TokenEntry{
		Timestamp:   time.Now().UTC(),
		Tokens:      tokens,
		CostUSD:     cost,
		Description: description,
	}
	sess.Entries = append(sess.Entries, entry)
	sess.TokensTotal += tokens
	sess.CostTotal += cost

	return ft.saveActive(active)
}

// StopTracking ends a session, persists it to the history file, and returns a summary.
func (ft *FeatureTracker) StopTracking(issueID string) (FeatureSummary, error) {
	active, err := ft.loadActive()
	if err != nil {
		return FeatureSummary{}, err
	}

	sess, ok := active[issueID]
	if !ok {
		return FeatureSummary{}, fmt.Errorf("no active session for issue %s", issueID)
	}

	now := time.Now().UTC()
	sess.EndTime = &now

	if err := ft.appendHistory(sess); err != nil {
		return FeatureSummary{}, fmt.Errorf("persisting session for issue %s: %w", issueID, err)
	}

	delete(active, issueID)
	if err := ft.saveActive(active); err != nil {
		return FeatureSummary{}, err
	}

	return summaryFrom(sess, false), nil
}

// ActiveSessions returns summaries of all in-progress sessions.
func (ft *FeatureTracker) ActiveSessions() ([]FeatureSummary, error) {
	active, err := ft.loadActive()
	if err != nil {
		return nil, err
	}
	var out []FeatureSummary
	for _, sess := range active {
		out = append(out, summaryFrom(sess, true))
	}
	return out, nil
}

// Report reads all persisted sessions from history and all active sessions,
// then returns their summaries.
func (ft *FeatureTracker) Report() ([]FeatureSummary, error) {
	persisted, err := ft.loadHistory()
	if err != nil {
		return nil, err
	}
	var out []FeatureSummary
	for i := range persisted {
		out = append(out, summaryFrom(&persisted[i], false))
	}

	active, err := ft.loadActive()
	if err != nil {
		return nil, err
	}
	for _, sess := range active {
		out = append(out, summaryFrom(sess, true))
	}
	return out, nil
}

// loadActive reads the active sessions state file. Returns an empty map if the
// file does not exist.
func (ft *FeatureTracker) loadActive() (map[string]*FeatureSession, error) {
	data, err := os.ReadFile(ft.statePath)
	if os.IsNotExist(err) {
		return make(map[string]*FeatureSession), nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]*FeatureSession
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing active sessions state: %w", err)
	}
	if m == nil {
		m = make(map[string]*FeatureSession)
	}
	return m, nil
}

// saveActive writes the active sessions map to the state file (atomic overwrite).
func (ft *FeatureTracker) saveActive(active map[string]*FeatureSession) error {
	data, err := json.MarshalIndent(active, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ft.statePath, data, 0644)
}

// appendHistory appends a completed session record to the JSONL history file.
func (ft *FeatureTracker) appendHistory(sess *FeatureSession) error {
	f, err := os.OpenFile(ft.historyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	rec := persistRecord{Session: *sess}
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, string(line))
	return err
}

// loadHistory reads and parses every completed session record from the JSONL history file.
func (ft *FeatureTracker) loadHistory() ([]FeatureSession, error) {
	data, err := os.ReadFile(ft.historyPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var sessions []FeatureSession
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var rec persistRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue // skip malformed lines
		}
		sessions = append(sessions, rec.Session)
	}
	return sessions, nil
}

// splitLines splits a byte slice by newlines.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

func summaryFrom(sess *FeatureSession, active bool) FeatureSummary {
	return FeatureSummary{
		IssueID:     sess.IssueID,
		Model:       sess.Model,
		StartTime:   sess.StartTime,
		EndTime:     sess.EndTime,
		TokensTotal: sess.TokensTotal,
		CostTotal:   sess.CostTotal,
		EntryCount:  len(sess.Entries),
		Active:      active,
	}
}
