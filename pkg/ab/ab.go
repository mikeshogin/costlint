package ab

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/mshogin/costlint/pkg/pricing"
)

// Experiment defines an A/B test configuration.
type Experiment struct {
	Name   string  `json:"name"`
	Groups []Group `json:"groups"`
}

// Group defines a traffic split group.
type Group struct {
	Name    string `json:"name"`
	Model   string `json:"model"`
	Weight  int    `json:"weight"` // percentage of traffic (all groups must sum to 100)
}

// Assignment represents a routing decision for one request.
type Assignment struct {
	ExperimentName string `json:"experiment"`
	GroupName      string `json:"group"`
	Model          string `json:"model"`
	Timestamp      string `json:"timestamp"`
}

// Result contains aggregated A/B test results.
type Result struct {
	ExperimentName string              `json:"experiment"`
	TotalRequests  int                 `json:"total_requests"`
	Groups         map[string]*GroupResult `json:"groups"`
}

// GroupResult contains per-group metrics.
type GroupResult struct {
	Requests     int     `json:"requests"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	AvgTokens    float64 `json:"avg_tokens_per_request"`
}

// Router assigns requests to groups based on weights.
type Router struct {
	experiment Experiment
	rng        *rand.Rand
}

// NewRouter creates an A/B test router.
func NewRouter(exp Experiment) *Router {
	return &Router{
		experiment: exp,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Assign picks a group for this request based on weights.
func (r *Router) Assign() Assignment {
	roll := r.rng.Intn(100)
	cumulative := 0
	for _, g := range r.experiment.Groups {
		cumulative += g.Weight
		if roll < cumulative {
			return Assignment{
				ExperimentName: r.experiment.Name,
				GroupName:      g.Name,
				Model:          g.Model,
				Timestamp:      time.Now().UTC().Format(time.RFC3339),
			}
		}
	}
	// Fallback to last group
	last := r.experiment.Groups[len(r.experiment.Groups)-1]
	return Assignment{
		ExperimentName: r.experiment.Name,
		GroupName:      last.Name,
		Model:          last.Model,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
}

// DefaultExperiment returns a standard 30/30/40 split.
func DefaultExperiment() Experiment {
	return Experiment{
		Name: "model-routing-v1",
		Groups: []Group{
			{Name: "haiku", Model: "haiku", Weight: 30},
			{Name: "sonnet", Model: "sonnet", Weight: 30},
			{Name: "opus", Model: "opus", Weight: 40},
		},
	}
}

// AnalyzeResults reads assignment log and produces comparison.
func AnalyzeResults(logPath string) (*Result, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("read ab log: %w", err)
	}

	result := &Result{
		Groups: make(map[string]*GroupResult),
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var record struct {
			Assignment
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}

		if result.ExperimentName == "" {
			result.ExperimentName = record.ExperimentName
		}
		result.TotalRequests++

		if _, ok := result.Groups[record.GroupName]; !ok {
			result.Groups[record.GroupName] = &GroupResult{}
		}
		g := result.Groups[record.GroupName]
		g.Requests++
		g.InputTokens += record.InputTokens
		g.OutputTokens += record.OutputTokens
		g.CostUSD += pricing.Estimate(record.Model, record.InputTokens, record.OutputTokens)
	}

	// Calculate averages
	for _, g := range result.Groups {
		if g.Requests > 0 {
			g.AvgTokens = float64(g.InputTokens+g.OutputTokens) / float64(g.Requests)
		}
	}

	return result, nil
}

// FormatResults produces a human-readable A/B comparison.
func FormatResults(r *Result) string {
	var b strings.Builder

	fmt.Fprintf(&b, "A/B Test Results: %s\n", r.ExperimentName)
	fmt.Fprintf(&b, "  Total requests: %d\n\n", r.TotalRequests)

	fmt.Fprintf(&b, "  %-10s %-10s %-12s %-12s %-10s\n", "Group", "Requests", "Tokens", "Avg/Req", "Cost")
	fmt.Fprintf(&b, "  %-10s %-10s %-12s %-12s %-10s\n", "-----", "--------", "------", "-------", "----")

	for name, g := range r.Groups {
		totalTokens := g.InputTokens + g.OutputTokens
		fmt.Fprintf(&b, "  %-10s %-10d %-12d %-12.0f $%-9.2f\n",
			name, g.Requests, totalTokens, g.AvgTokens, g.CostUSD)
	}

	return b.String()
}
