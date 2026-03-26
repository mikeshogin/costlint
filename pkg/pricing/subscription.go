package pricing

import "fmt"

// SubscriptionPlan represents a flat-fee monthly subscription with optional token inclusions.
type SubscriptionPlan struct {
	Key              string  `json:"key"`
	Name             string  `json:"name"`
	MonthlyFeeUSD    float64 `json:"monthly_fee_usd"`
	// IncludedTokens == 0 means unlimited.
	IncludedTokens   int64   `json:"included_tokens"`
	// OveragePerMToken == 0 means no overage (hard cap or unlimited).
	OveragePerMToken float64 `json:"overage_per_m_token"`
}

// SubscriptionPlans lists the available Claude MAX subscription tiers.
var SubscriptionPlans = map[string]SubscriptionPlan{
	"claude_max_5": {
		Key:              "claude_max_5",
		Name:             "Claude MAX $100",
		MonthlyFeeUSD:    100.0,
		IncludedTokens:   5_000_000,
		OveragePerMToken: 0, // hard cap, no overage
	},
	"claude_max_20": {
		Key:              "claude_max_20",
		Name:             "Claude MAX $200",
		MonthlyFeeUSD:    200.0,
		IncludedTokens:   0, // unlimited
		OveragePerMToken: 0,
	},
}

// MonthlyUsage tracks token consumption for a subscription period.
type MonthlyUsage struct {
	Plan              SubscriptionPlan `json:"plan"`
	UsedTokens        int64            `json:"used_tokens"`
	// RemainingTokens == -1 means unlimited.
	RemainingTokens   int64            `json:"remaining_tokens"`
	EffectiveCostUSD  float64          `json:"effective_cost_usd"`
	EffectiveCPMToken float64          `json:"effective_cost_per_m_token"`
}

// NewMonthlyUsage creates a MonthlyUsage for the given plan key and token consumption.
func NewMonthlyUsage(planKey string, usedTokens int64) (*MonthlyUsage, error) {
	plan, ok := SubscriptionPlans[planKey]
	if !ok {
		return nil, fmt.Errorf("unknown subscription plan: %s (available: claude_max_5, claude_max_20)", planKey)
	}

	mu := &MonthlyUsage{
		Plan:       plan,
		UsedTokens: usedTokens,
	}

	// Calculate remaining tokens.
	if plan.IncludedTokens == 0 {
		mu.RemainingTokens = -1 // unlimited
	} else {
		remaining := plan.IncludedTokens - usedTokens
		if remaining < 0 {
			remaining = 0
		}
		mu.RemainingTokens = remaining
	}

	// Calculate effective total cost (subscription fee + overage).
	overageCost := 0.0
	if plan.IncludedTokens > 0 && plan.OveragePerMToken > 0 && usedTokens > plan.IncludedTokens {
		excess := usedTokens - plan.IncludedTokens
		overageCost = float64(excess) / 1_000_000 * plan.OveragePerMToken
	}
	mu.EffectiveCostUSD = plan.MonthlyFeeUSD + overageCost

	if usedTokens > 0 {
		mu.EffectiveCPMToken = mu.EffectiveCostUSD / (float64(usedTokens) / 1_000_000)
	}

	return mu, nil
}

// SubscriptionComparison holds a side-by-side comparison of one subscription plan vs pay-as-you-go.
type SubscriptionComparison struct {
	Plan            SubscriptionPlan `json:"plan"`
	ModelKey        string           `json:"model"`
	MonthlyTokens   int64            `json:"monthly_tokens"`
	SubscriptionUSD float64          `json:"subscription_usd"`
	PayAsYouGoUSD   float64          `json:"pay_as_you_go_usd"`
	SavingsUSD      float64          `json:"savings_usd"`
	SavingsPct      float64          `json:"savings_pct"`
	Recommendation  string           `json:"recommendation"`
}

// CompareSubscription calculates whether a subscription plan is cheaper than pay-as-you-go
// for a given monthly token volume. modelKey selects the pay-as-you-go model pricing.
// Tokens are assumed to be split 50/50 input/output.
func CompareSubscription(planKey, modelKey string, monthlyTokens int64) (*SubscriptionComparison, error) {
	plan, ok := SubscriptionPlans[planKey]
	if !ok {
		return nil, fmt.Errorf("unknown subscription plan: %s (available: claude_max_5, claude_max_20)", planKey)
	}

	// Pay-as-you-go cost: assume 50/50 input/output split.
	half := int(monthlyTokens / 2)
	payg := Estimate(modelKey, half, half)

	// Subscription effective cost for this volume.
	mu, err := NewMonthlyUsage(planKey, monthlyTokens)
	if err != nil {
		return nil, err
	}
	subUSD := mu.EffectiveCostUSD

	savings := payg - subUSD
	savingsPct := 0.0
	if payg > 0 {
		savingsPct = savings / payg * 100
	}

	recommendation := "subscription saves money"
	if savings <= 0 {
		recommendation = "pay-as-you-go is cheaper"
	}

	return &SubscriptionComparison{
		Plan:            plan,
		ModelKey:        modelKey,
		MonthlyTokens:   monthlyTokens,
		SubscriptionUSD: subUSD,
		PayAsYouGoUSD:   payg,
		SavingsUSD:      savings,
		SavingsPct:      savingsPct,
		Recommendation:  recommendation,
	}, nil
}

// CompareAllSubscriptions compares all subscription plans against pay-as-you-go for a given
// monthly token volume and model.
func CompareAllSubscriptions(modelKey string, monthlyTokens int64) ([]SubscriptionComparison, error) {
	results := make([]SubscriptionComparison, 0, len(SubscriptionPlans))
	for planKey := range SubscriptionPlans {
		cmp, err := CompareSubscription(planKey, modelKey, monthlyTokens)
		if err != nil {
			return nil, err
		}
		results = append(results, *cmp)
	}
	return results, nil
}
