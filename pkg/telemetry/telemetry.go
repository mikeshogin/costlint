package telemetry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// TelemetryEvent represents a single cost-tracking event ingested from promptlint or other sources.
type TelemetryEvent struct {
	Timestamp  string  `json:"timestamp"`
	PromptHash string  `json:"prompt_hash,omitempty"`
	Model      string  `json:"model"`
	Complexity string  `json:"complexity,omitempty"`
	Tokens     int     `json:"tokens"`
	CostUSD    float64 `json:"cost_usd"`
	Source     string  `json:"source"`
}

// PromptlintRecord is the expected shape of a single promptlint analyze output line.
type PromptlintRecord struct {
	Timestamp string             `json:"timestamp"`
	RoutedTo  string             `json:"routed_to"`
	Analysis  *PromptlintAnalysis `json:"analysis,omitempty"`
	// Direct token counts when available.
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// PromptlintAnalysis contains the nested analysis block from promptlint output.
type PromptlintAnalysis struct {
	Words      int    `json:"words"`
	Complexity string `json:"complexity"`
	Action     string `json:"action"`
	Model      string `json:"suggested_model"`
	Hash       string `json:"prompt_hash,omitempty"`
}

// ModelSummary holds aggregated metrics for a single model.
type ModelSummary struct {
	Events   int     `json:"events"`
	Tokens   int     `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
}

// ComplexitySummary holds aggregated metrics for a single complexity level.
type ComplexitySummary struct {
	Events  int     `json:"events"`
	Tokens  int     `json:"tokens"`
	CostUSD float64 `json:"cost_usd"`
}

// Summary contains aggregated telemetry metrics.
type Summary struct {
	TotalEvents      int                          `json:"total_events"`
	TotalCostUSD     float64                      `json:"total_cost_usd"`
	ByModel          map[string]*ModelSummary     `json:"by_model"`
	ByComplexity     map[string]*ComplexitySummary `json:"by_complexity"`
}

// TelemetryLog manages reading and writing TelemetryEvent records to a JSONL file.
type TelemetryLog struct {
	path string
}

// NewTelemetryLog creates a TelemetryLog backed by the given file path.
func NewTelemetryLog(path string) *TelemetryLog {
	return &TelemetryLog{path: path}
}

// Append writes a single event as a JSON line to the log file.
func (l *TelemetryLog) Append(ev TelemetryEvent) error {
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open telemetry log: %w", err)
	}
	defer f.Close()

	line, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	_, err = fmt.Fprintf(f, "%s\n", line)
	return err
}

// Load reads all events from the log file. Returns an empty slice when the file
// does not exist yet.
func (l *TelemetryLog) Load() ([]TelemetryEvent, error) {
	f, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open telemetry log: %w", err)
	}
	defer f.Close()

	var events []TelemetryEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev TelemetryEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// Skip malformed lines.
			continue
		}
		events = append(events, ev)
	}
	return events, scanner.Err()
}

// Ingest parses one or more promptlint analyze output lines (JSONL) and appends
// a TelemetryEvent for each valid record to the log.
func (l *TelemetryLog) Ingest(promptlintJSON []byte) (int, error) {
	var ingested int
	for _, raw := range strings.Split(string(promptlintJSON), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var rec PromptlintRecord
		if err := json.Unmarshal([]byte(raw), &rec); err != nil {
			// Skip lines that don't match the expected schema.
			continue
		}

		ev := eventFromRecord(rec)
		if err := l.Append(ev); err != nil {
			return ingested, err
		}
		ingested++
	}
	return ingested, nil
}

// eventFromRecord converts a PromptlintRecord to a TelemetryEvent.
func eventFromRecord(rec PromptlintRecord) TelemetryEvent {
	ts := rec.Timestamp
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}

	model := rec.RoutedTo
	var complexity, hash string
	tokens := rec.InputTokens + rec.OutputTokens

	if rec.Analysis != nil {
		if rec.Analysis.Model != "" && model == "" {
			model = rec.Analysis.Model
		}
		complexity = rec.Analysis.Complexity
		hash = rec.Analysis.Hash
		// Estimate tokens from word count when explicit counts are absent.
		if tokens == 0 && rec.Analysis.Words > 0 {
			tokens = int(float64(rec.Analysis.Words) * 1.33)
		}
	}

	// Rough cost estimate: use simple per-token rate based on model tier.
	cost := estimateCost(model, tokens)

	return TelemetryEvent{
		Timestamp:  ts,
		PromptHash: hash,
		Model:      model,
		Complexity: complexity,
		Tokens:     tokens,
		CostUSD:    cost,
		Source:     "promptlint",
	}
}

// estimateCost provides a simple token-based cost estimate.
// Rates are intentionally conservative approximations (input-only pricing).
func estimateCost(model string, tokens int) float64 {
	const million = 1_000_000.0
	ratePerMillion := map[string]float64{
		"haiku":  0.25,
		"sonnet": 3.00,
		"opus":   15.00,
	}
	rate, ok := ratePerMillion[model]
	if !ok {
		rate = 3.00 // default to sonnet pricing
	}
	return (float64(tokens) / million) * rate
}

// Summary aggregates all events in the log and returns a Summary.
func (l *TelemetryLog) Summary() (*Summary, error) {
	events, err := l.Load()
	if err != nil {
		return nil, err
	}

	s := &Summary{
		ByModel:      make(map[string]*ModelSummary),
		ByComplexity: make(map[string]*ComplexitySummary),
	}

	for _, ev := range events {
		s.TotalEvents++
		s.TotalCostUSD += ev.CostUSD

		// By model.
		if _, ok := s.ByModel[ev.Model]; !ok {
			s.ByModel[ev.Model] = &ModelSummary{}
		}
		ms := s.ByModel[ev.Model]
		ms.Events++
		ms.Tokens += ev.Tokens
		ms.CostUSD += ev.CostUSD

		// By complexity.
		cplx := ev.Complexity
		if cplx == "" {
			cplx = "unknown"
		}
		if _, ok := s.ByComplexity[cplx]; !ok {
			s.ByComplexity[cplx] = &ComplexitySummary{}
		}
		cs := s.ByComplexity[cplx]
		cs.Events++
		cs.Tokens += ev.Tokens
		cs.CostUSD += ev.CostUSD
	}

	return s, nil
}
