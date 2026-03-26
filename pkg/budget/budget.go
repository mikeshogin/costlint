package budget

import (
	"encoding/json"
	"fmt"

	"github.com/mshogin/costlint/pkg/pricing"
)

// Budget tracks token usage and cost against a maximum USD budget for a session.
type Budget struct {
	MaxUSD     float64 `json:"max_usd"`
	Model      string  `json:"model"`
	UsedTokens int     `json:"used_tokens"`
	UsedUSD    float64 `json:"used_usd"`
}

// NewBudget creates a new Budget with the given max spend and model.
func NewBudget(maxUSD float64, model string) *Budget {
	return &Budget{
		MaxUSD: maxUSD,
		Model:  model,
	}
}

// Add records additional tokens and recalculates the cumulative cost.
// Tokens are treated as input tokens for cost calculation.
func (b *Budget) Add(tokens int) {
	b.UsedTokens += tokens
	b.UsedUSD = pricing.Estimate(b.Model, b.UsedTokens, 0)
}

// Remaining returns how much budget (in USD) is still available.
func (b *Budget) Remaining() float64 {
	r := b.MaxUSD - b.UsedUSD
	if r < 0 {
		return 0
	}
	return r
}

// IsOverBudget returns true when the used cost has reached or exceeded the max budget.
func (b *Budget) IsOverBudget() bool {
	return b.UsedUSD >= b.MaxUSD
}

// Alert returns a non-empty warning message when usage has reached 80% of the budget,
// and returns an empty string when everything is within safe limits.
func (b *Budget) Alert() string {
	if b.MaxUSD == 0 {
		return ""
	}
	pct := b.UsedUSD / b.MaxUSD * 100
	if b.IsOverBudget() {
		return fmt.Sprintf(
			"BUDGET EXCEEDED: used $%.4f of $%.2f (%.1f%%) - %d tokens",
			b.UsedUSD, b.MaxUSD, pct, b.UsedTokens,
		)
	}
	if pct >= 80 {
		return fmt.Sprintf(
			"BUDGET ALERT: %.1f%% used ($%.4f of $%.2f, %d tokens) - $%.4f remaining",
			pct, b.UsedUSD, b.MaxUSD, b.UsedTokens, b.Remaining(),
		)
	}
	return ""
}

// ToJSON serialises the current budget state to JSON.
func (b *Budget) ToJSON() ([]byte, error) {
	type output struct {
		MaxUSD      float64 `json:"max_usd"`
		Model       string  `json:"model"`
		UsedTokens  int     `json:"used_tokens"`
		UsedUSD     float64 `json:"used_usd"`
		RemainingUSD float64 `json:"remaining_usd"`
		PctUsed     float64 `json:"pct_used"`
		IsOverBudget bool   `json:"is_over_budget"`
		Alert       string  `json:"alert,omitempty"`
	}
	pct := 0.0
	if b.MaxUSD > 0 {
		pct = b.UsedUSD / b.MaxUSD * 100
	}
	return json.Marshal(output{
		MaxUSD:       b.MaxUSD,
		Model:        b.Model,
		UsedTokens:   b.UsedTokens,
		UsedUSD:      b.UsedUSD,
		RemainingUSD: b.Remaining(),
		PctUsed:      pct,
		IsOverBudget: b.IsOverBudget(),
		Alert:        b.Alert(),
	})
}
