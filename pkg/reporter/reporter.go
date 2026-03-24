package reporter

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mshogin/costlint/pkg/pricing"
)

// Record represents one telemetry entry from promptlint or direct logging.
type Record struct {
	Timestamp    string `json:"timestamp"`
	Model        string `json:"routed_to"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	Source       string `json:"source,omitempty"`
}

// Report contains aggregated cost analysis.
type Report struct {
	TotalRequests int                `json:"total_requests"`
	TotalInput    int                `json:"total_input_tokens"`
	TotalOutput   int                `json:"total_output_tokens"`
	TotalTokens   int                `json:"total_tokens"`
	ByModel       map[string]*ModelStats `json:"by_model"`
	EstimatedCost float64            `json:"estimated_cost_usd"`
	OptimalCost   float64            `json:"optimal_cost_usd"`
	SavingsUSD    float64            `json:"potential_savings_usd"`
	SavingsPct    float64            `json:"potential_savings_pct"`
}

// ModelStats contains per-model statistics.
type ModelStats struct {
	Requests     int     `json:"requests"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// GenerateFromFile reads a JSONL telemetry file and produces a report.
func GenerateFromFile(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read telemetry: %w", err)
	}

	report := &Report{
		ByModel: make(map[string]*ModelStats),
	}

	for _, line := range strings.Split(string(data), "\n") {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}

		report.TotalRequests++
		report.TotalInput += r.InputTokens
		report.TotalOutput += r.OutputTokens

		if _, ok := report.ByModel[r.Model]; !ok {
			report.ByModel[r.Model] = &ModelStats{}
		}
		stats := report.ByModel[r.Model]
		stats.Requests++
		stats.InputTokens += r.InputTokens
		stats.OutputTokens += r.OutputTokens
		stats.CostUSD += pricing.Estimate(r.Model, r.InputTokens, r.OutputTokens)
	}

	report.TotalTokens = report.TotalInput + report.TotalOutput

	// Calculate total and optimal costs
	for _, stats := range report.ByModel {
		report.EstimatedCost += stats.CostUSD
	}

	// Optimal: route everything through cheapest sufficient model
	// For now, assume haiku for simple (< 1000 tokens), sonnet for medium, opus for complex
	for model, stats := range report.ByModel {
		if model == "opus" {
			// Could 50% of opus requests be handled by sonnet?
			saveable := stats.Requests / 2
			report.OptimalCost += pricing.Estimate("sonnet", stats.InputTokens*saveable/stats.Requests, stats.OutputTokens*saveable/stats.Requests)
			report.OptimalCost += pricing.Estimate("opus", stats.InputTokens*(stats.Requests-saveable)/stats.Requests, stats.OutputTokens*(stats.Requests-saveable)/stats.Requests)
		} else {
			report.OptimalCost += stats.CostUSD
		}
	}

	if report.EstimatedCost > 0 {
		report.SavingsUSD = report.EstimatedCost - report.OptimalCost
		report.SavingsPct = (report.SavingsUSD / report.EstimatedCost) * 100
	}

	return report, nil
}

// FormatReport produces a human-readable report string.
func FormatReport(r *Report) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Cost Report:\n")
	fmt.Fprintf(&b, "  Total requests: %d\n", r.TotalRequests)
	fmt.Fprintf(&b, "  Total tokens: %d (in: %d / out: %d)\n", r.TotalTokens, r.TotalInput, r.TotalOutput)
	fmt.Fprintf(&b, "\n  By model:\n")

	for model, stats := range r.ByModel {
		fmt.Fprintf(&b, "    %s: %d requests, %dK tokens, ~$%.2f\n",
			model, stats.Requests, (stats.InputTokens+stats.OutputTokens)/1000, stats.CostUSD)
	}

	fmt.Fprintf(&b, "\n  Estimated total: ~$%.2f\n", r.EstimatedCost)
	fmt.Fprintf(&b, "  With optimal routing: ~$%.2f (savings: %.0f%%)\n", r.OptimalCost, r.SavingsPct)

	return b.String()
}
