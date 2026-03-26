package cache

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

// Metrics contains all cache-related metrics.
type Metrics struct {
	// Cache performance
	TotalRequests     int     `json:"total_requests"`
	CacheHits         int     `json:"cache_hits"`
	CacheMisses       int     `json:"cache_misses"`
	CacheHitRate      float64 `json:"cache_hit_rate"`
	CacheHitTokens    int     `json:"cache_hit_tokens"`
	CacheMissTokens   int     `json:"cache_miss_tokens"`
	CacheSavingsUSD   float64 `json:"cache_savings_usd"`
	PotentialSavings  float64 `json:"potential_savings_usd"`

	// Block analysis
	AvgBlocksPerReq   float64 `json:"avg_blocks_per_request"`
	BlockReuseRate    float64 `json:"block_reuse_rate"`
	BlockStabilityAvg float64 `json:"block_stability_score"`

	// Temporal
	AvgRequestInterval float64 `json:"avg_request_interval_sec"`
	SessionCount       int     `json:"session_count"`

	// Mathematical
	ContentEntropy     float64 `json:"content_entropy"`
	JaccardSimilarity  float64 `json:"jaccard_similarity_avg"`

	// Optimization hints
	Hints []string `json:"optimization_hints,omitempty"`
}

// CacheRecord represents one request/response with cache data.
type CacheRecord struct {
	Timestamp            string `json:"timestamp"`
	SessionID            string `json:"session_id,omitempty"`
	InputTokens          int    `json:"input_tokens"`
	OutputTokens         int    `json:"output_tokens"`
	CacheCreationTokens  int    `json:"cache_creation_input_tokens"`
	CacheReadTokens      int    `json:"cache_read_input_tokens"`
	Model                string `json:"model,omitempty"`
	SystemPromptHash     string `json:"system_prompt_hash,omitempty"`
	BlockCount           int    `json:"block_count,omitempty"`
}

// Cache pricing: cache reads are 90% cheaper than regular input.
const (
	CacheReadDiscount = 0.9  // 90% discount on cached tokens
	CacheWritePremium = 0.25 // 25% premium on cache creation
)

// AnalyzeFromFile reads JSONL cache telemetry and produces metrics.
func AnalyzeFromFile(path string) (*Metrics, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cache telemetry: %w", err)
	}

	var records []CacheRecord
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var r CacheRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		records = append(records, r)
	}

	if len(records) == 0 {
		return &Metrics{}, nil
	}

	return calculate(records), nil
}

func calculate(records []CacheRecord) *Metrics {
	m := &Metrics{
		TotalRequests: len(records),
	}

	totalBlocks := 0
	sessions := make(map[string]bool)
	var prevHash string
	sameHashCount := 0

	for _, r := range records {
		// Cache hit/miss
		if r.CacheReadTokens > 0 {
			m.CacheHits++
			m.CacheHitTokens += r.CacheReadTokens
		}
		if r.CacheCreationTokens > 0 {
			m.CacheMisses++
			m.CacheMissTokens += r.CacheCreationTokens
		}

		// Block count
		if r.BlockCount > 0 {
			totalBlocks += r.BlockCount
		}

		// Sessions
		if r.SessionID != "" {
			sessions[r.SessionID] = true
		}

		// System prompt stability
		if r.SystemPromptHash != "" {
			if r.SystemPromptHash == prevHash {
				sameHashCount++
			}
			prevHash = r.SystemPromptHash
		}
	}

	m.SessionCount = len(sessions)
	if m.SessionCount == 0 {
		m.SessionCount = 1
	}

	// Cache hit rate
	totalCacheTokens := m.CacheHitTokens + m.CacheMissTokens
	if totalCacheTokens > 0 {
		m.CacheHitRate = float64(m.CacheHitTokens) / float64(totalCacheTokens) * 100
	}

	// Cache savings: cached tokens cost 90% less
	// Regular price assumed $3/1M (sonnet input)
	regularPrice := float64(m.CacheHitTokens) / 1_000_000 * 3.0
	cachedPrice := float64(m.CacheHitTokens) / 1_000_000 * 3.0 * (1 - CacheReadDiscount)
	m.CacheSavingsUSD = regularPrice - cachedPrice

	// Potential savings: if ALL miss tokens were cached
	potentialRegular := float64(m.CacheMissTokens) / 1_000_000 * 3.0
	potentialCached := float64(m.CacheMissTokens) / 1_000_000 * 3.0 * (1 - CacheReadDiscount)
	m.PotentialSavings = potentialRegular - potentialCached

	// Block analysis
	if m.TotalRequests > 0 {
		m.AvgBlocksPerReq = float64(totalBlocks) / float64(m.TotalRequests)
	}

	// System prompt stability score
	if m.TotalRequests > 1 {
		m.BlockStabilityAvg = float64(sameHashCount) / float64(m.TotalRequests-1) * 100
	}

	// Content entropy (simplified: based on token distribution)
	m.ContentEntropy = calculateEntropy(records)

	// Jaccard similarity between consecutive requests
	m.JaccardSimilarity = calculateJaccardAvg(records)

	// Block reuse rate
	if m.CacheHits+m.CacheMisses > 0 {
		m.BlockReuseRate = float64(m.CacheHits) / float64(m.CacheHits+m.CacheMisses) * 100
	}

	// Generate optimization hints
	m.Hints = generateHints(m)

	return m
}

func calculateEntropy(records []CacheRecord) float64 {
	if len(records) == 0 {
		return 0
	}
	// Entropy based on token size distribution
	totalTokens := 0
	for _, r := range records {
		totalTokens += r.InputTokens
	}
	if totalTokens == 0 {
		return 0
	}

	entropy := 0.0
	for _, r := range records {
		if r.InputTokens > 0 {
			p := float64(r.InputTokens) / float64(totalTokens)
			if p > 0 {
				entropy -= p * math.Log2(p)
			}
		}
	}
	return math.Round(entropy*100) / 100
}

func calculateJaccardAvg(records []CacheRecord) float64 {
	if len(records) < 2 {
		return 0
	}
	// Simplified Jaccard: compare cache token overlap between consecutive requests
	totalSimilarity := 0.0
	count := 0
	for i := 1; i < len(records); i++ {
		prev := records[i-1]
		curr := records[i]
		// Use token counts as proxy for content overlap
		intersection := min(prev.CacheReadTokens, curr.CacheReadTokens)
		union := max(prev.InputTokens, curr.InputTokens)
		if union > 0 {
			totalSimilarity += float64(intersection) / float64(union)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return math.Round(totalSimilarity/float64(count)*100) / 100
}

func generateHints(m *Metrics) []string {
	var hints []string

	if m.CacheHitRate < 30 {
		hints = append(hints, fmt.Sprintf("Low cache hit rate (%.0f%%). Consider stabilizing system prompts and context files.", m.CacheHitRate))
	}

	if m.BlockStabilityAvg < 50 && m.TotalRequests > 5 {
		hints = append(hints, fmt.Sprintf("System prompt changes frequently (stability %.0f%%). Pin system prompt for better caching.", m.BlockStabilityAvg))
	}

	if m.PotentialSavings > 1.0 {
		hints = append(hints, fmt.Sprintf("Potential savings of $%.2f if cache misses were eliminated. Investigate why blocks aren't cached.", m.PotentialSavings))
	}

	if m.CacheHitRate > 80 {
		hints = append(hints, fmt.Sprintf("Excellent cache performance (%.0f%% hit rate). Current setup is well-optimized.", m.CacheHitRate))
	}

	if m.JaccardSimilarity < 0.2 && m.TotalRequests > 3 {
		hints = append(hints, fmt.Sprintf("Low request similarity (%.2f). Requests vary too much for effective caching.", m.JaccardSimilarity))
	}

	return hints
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CacheSimulator tracks prompts and detects similarity-based cache hits.
type CacheSimulator struct {
	model     string
	threshold float64
	prompts   []string
	Metrics   CacheMetrics
}

// CacheMetrics holds telemetry for the cache simulation.
type CacheMetrics struct {
	TotalPrompts  int     `json:"total_prompts"`
	CacheHits     int     `json:"cache_hits"`
	CacheMisses   int     `json:"cache_misses"`
	HitRate       float64 `json:"hit_rate"`
	TokensSaved   int     `json:"tokens_saved"`
	CostSavedUSD  float64 `json:"cost_saved_usd"`
}

// NewCacheSimulator creates a simulator for the given model with default 0.5 Jaccard threshold.
func NewCacheSimulator(model string) *CacheSimulator {
	return &CacheSimulator{
		model:     model,
		threshold: 0.5,
	}
}

// Add registers a prompt and returns whether it was a cache hit.
func (s *CacheSimulator) Add(prompt string) bool {
	hit := false
	tokens := estimateTokens(prompt)

	for _, prev := range s.prompts {
		if jaccard(prompt, prev) >= s.threshold {
			hit = true
			break
		}
	}

	s.prompts = append(s.prompts, prompt)
	s.Metrics.TotalPrompts++

	if hit {
		s.Metrics.CacheHits++
		s.Metrics.TokensSaved += tokens
		s.Metrics.CostSavedUSD += costForTokens(s.model, tokens)
	} else {
		s.Metrics.CacheMisses++
	}

	if s.Metrics.TotalPrompts > 0 {
		s.Metrics.HitRate = float64(s.Metrics.CacheHits) / float64(s.Metrics.TotalPrompts) * 100
	}

	return hit
}

// jaccard computes Jaccard similarity between two strings based on word sets.
func jaccard(a, b string) float64 {
	setA := wordSet(a)
	setB := wordSet(b)
	if len(setA) == 0 && len(setB) == 0 {
		return 1.0
	}
	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func wordSet(s string) map[string]bool {
	set := make(map[string]bool)
	for _, word := range strings.Fields(strings.ToLower(s)) {
		if len(word) > 0 {
			set[word] = true
		}
	}
	return set
}

// estimateTokens approximates token count: ~4 chars per token.
func estimateTokens(s string) int {
	n := len(s) / 4
	if n == 0 {
		n = 1
	}
	return n
}

func costForTokens(model string, tokens int) float64 {
	// Cache read tokens cost 90% less than regular input.
	// Use a simple per-model price table ($ per 1M input tokens).
	pricePerM := map[string]float64{
		"haiku":  0.80,
		"sonnet": 3.00,
		"opus":   15.00,
	}
	p, ok := pricePerM[model]
	if !ok {
		p = 3.00 // default to sonnet
	}
	regularCost := float64(tokens) / 1_000_000 * p
	cachedCost := regularCost * (1 - CacheReadDiscount)
	return regularCost - cachedCost
}

// FormatCacheReport produces human-readable output for CacheMetrics.
func FormatCacheReport(m CacheMetrics) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Cache Simulation Results:\n")
	fmt.Fprintf(&b, "  Total prompts : %d\n", m.TotalPrompts)
	fmt.Fprintf(&b, "  Cache hits    : %d\n", m.CacheHits)
	fmt.Fprintf(&b, "  Cache misses  : %d\n", m.CacheMisses)
	fmt.Fprintf(&b, "  Hit rate      : %.1f%%\n", m.HitRate)
	fmt.Fprintf(&b, "  Tokens saved  : %d\n", m.TokensSaved)
	fmt.Fprintf(&b, "  Cost saved    : $%.4f\n", m.CostSavedUSD)
	return b.String()
}

// FormatReport produces human-readable cache metrics report.
func FormatReport(m *Metrics) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Cache Metrics Report:\n")
	fmt.Fprintf(&b, "  Requests: %d (sessions: %d)\n", m.TotalRequests, m.SessionCount)
	fmt.Fprintf(&b, "\n  Cache Performance:\n")
	fmt.Fprintf(&b, "    Hit rate: %.1f%%\n", m.CacheHitRate)
	fmt.Fprintf(&b, "    Cached tokens: %d\n", m.CacheHitTokens)
	fmt.Fprintf(&b, "    Uncached tokens: %d\n", m.CacheMissTokens)
	fmt.Fprintf(&b, "    Savings from cache: $%.2f\n", m.CacheSavingsUSD)
	fmt.Fprintf(&b, "    Potential additional savings: $%.2f\n", m.PotentialSavings)
	fmt.Fprintf(&b, "\n  Block Analysis:\n")
	fmt.Fprintf(&b, "    Avg blocks/request: %.1f\n", m.AvgBlocksPerReq)
	fmt.Fprintf(&b, "    Block reuse rate: %.1f%%\n", m.BlockReuseRate)
	fmt.Fprintf(&b, "    System prompt stability: %.1f%%\n", m.BlockStabilityAvg)
	fmt.Fprintf(&b, "\n  Mathematical:\n")
	fmt.Fprintf(&b, "    Content entropy: %.2f bits\n", m.ContentEntropy)
	fmt.Fprintf(&b, "    Jaccard similarity (avg): %.2f\n", m.JaccardSimilarity)

	if len(m.Hints) > 0 {
		fmt.Fprintf(&b, "\n  Optimization Hints:\n")
		for i, hint := range m.Hints {
			fmt.Fprintf(&b, "    %d. %s\n", i+1, hint)
		}
	}

	return b.String()
}
