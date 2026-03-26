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

// ABResult holds a single data point for an A/B test.
type ABResult struct {
	PromptHash string  `json:"prompt_hash"`
	Model      string  `json:"model"`
	Tokens     int     `json:"tokens"`
	CostUSD    float64 `json:"cost_usd"`
	LatencyMs  int64   `json:"latency_ms"` // placeholder, always 0 (no real inference)
}

// ABSummary aggregates the comparison between two models.
type ABSummary struct {
	ModelAAvgCost float64 `json:"model_a_avg_cost"`
	ModelBAvgCost float64 `json:"model_b_avg_cost"`
	SavingsPct    float64 `json:"savings_pct"`
	Recommendation string `json:"recommendation"`
}

// ABTest is an in-memory A/B cost comparison between two models.
type ABTest struct {
	Name       string     `json:"name"`
	ModelA     string     `json:"model_a"`
	ModelB     string     `json:"model_b"`
	SampleSize int        `json:"sample_size"`
	Results    []ABResult `json:"results"`
}

// NewABTest creates a new ABTest for two model keys.
func NewABTest(name, modelA, modelB string) *ABTest {
	return &ABTest{
		Name:   name,
		ModelA: modelA,
		ModelB: modelB,
	}
}

// promptHash returns a short deterministic hash for a prompt string.
func promptHash(prompt string) string {
	h := 0
	for _, c := range prompt {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	digits := "0123456789abcdef"
	result := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		result[i] = digits[h&0xf]
		h >>= 4
	}
	return string(result)
}

// AddResult records one cost data point for the given model and prompt.
// tokens is the estimated input token count; output tokens are estimated
// automatically at 1:1 ratio to keep things simple.
func (t *ABTest) AddResult(model, prompt string, tokens int) {
	outputTokens := tokens // 1:1 default estimate
	cost := pricing.Estimate(model, tokens, outputTokens)
	t.Results = append(t.Results, ABResult{
		PromptHash: promptHash(prompt),
		Model:      model,
		Tokens:     tokens + outputTokens,
		CostUSD:    cost,
		LatencyMs:  0,
	})
	t.SampleSize = len(t.Results)
}

// Summary computes average costs per model and recommends the cheaper one.
func (t *ABTest) Summary() ABSummary {
	var sumA, sumB float64
	var countA, countB int

	for _, r := range t.Results {
		switch r.Model {
		case t.ModelA:
			sumA += r.CostUSD
			countA++
		case t.ModelB:
			sumB += r.CostUSD
			countB++
		}
	}

	var avgA, avgB float64
	if countA > 0 {
		avgA = sumA / float64(countA)
	}
	if countB > 0 {
		avgB = sumB / float64(countB)
	}

	var savingsPct float64
	var recommendation string

	switch {
	case avgA == 0 && avgB == 0:
		recommendation = "no data"
	case avgA == 0:
		recommendation = t.ModelB
	case avgB == 0:
		recommendation = t.ModelA
	case avgA <= avgB:
		if avgB > 0 {
			savingsPct = (avgB - avgA) / avgB * 100
		}
		recommendation = t.ModelA
	default:
		if avgA > 0 {
			savingsPct = (avgA - avgB) / avgA * 100
		}
		recommendation = t.ModelB
	}

	return ABSummary{
		ModelAAvgCost:  avgA,
		ModelBAvgCost:  avgB,
		SavingsPct:     savingsPct,
		Recommendation: recommendation,
	}
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
