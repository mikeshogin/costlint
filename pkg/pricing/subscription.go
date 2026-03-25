package pricing

// SubscriptionPlan represents a Claude subscription tier.
type SubscriptionPlan struct {
	Name           string  `json:"name"`
	MonthlyCostUSD float64 `json:"monthly_cost_usd"`
	TokensPerMonth int     `json:"tokens_per_month"` // approximate
	ModelsIncluded []string `json:"models_included"`
}

// Known subscription plans.
var Plans = map[string]SubscriptionPlan{
	"pro": {
		Name:           "Claude Pro",
		MonthlyCostUSD: 20.00,
		TokensPerMonth: 5_000_000,
		ModelsIncluded: []string{"sonnet", "haiku"},
	},
	"max5": {
		Name:           "Claude MAX (5x)",
		MonthlyCostUSD: 100.00,
		TokensPerMonth: 25_000_000,
		ModelsIncluded: []string{"opus", "sonnet", "haiku"},
	},
	"max20": {
		Name:           "Claude MAX (20x)",
		MonthlyCostUSD: 200.00,
		TokensPerMonth: 100_000_000,
		ModelsIncluded: []string{"opus", "sonnet", "haiku"},
	},
}

// SubscriptionComparison compares API cost vs subscription cost.
type SubscriptionComparison struct {
	APICostUSD         float64            `json:"api_cost_usd"`
	BestPlan           string             `json:"best_plan"`
	BestPlanCostUSD    float64            `json:"best_plan_cost_usd"`
	SavingsUSD         float64            `json:"savings_usd"`
	SavingsPct         float64            `json:"savings_pct"`
	PlanComparisons    map[string]PlanCost `json:"plan_comparisons"`
}

// PlanCost contains cost info for one plan.
type PlanCost struct {
	MonthlyCost   float64 `json:"monthly_cost_usd"`
	TokensUsed    int     `json:"tokens_used"`
	TokensAllowed int     `json:"tokens_allowed"`
	UtilizationPct float64 `json:"utilization_pct"`
	CostPerToken   float64 `json:"effective_cost_per_m_tokens"`
	Verdict        string  `json:"verdict"` // "cheaper", "more_expensive", "over_limit"
}

// CompareWithSubscription compares API usage cost against subscription plans.
func CompareWithSubscription(totalTokens int, apiCostUSD float64) SubscriptionComparison {
	result := SubscriptionComparison{
		APICostUSD:      apiCostUSD,
		PlanComparisons: make(map[string]PlanCost),
	}

	bestSavings := 0.0
	bestPlan := ""

	for planKey, plan := range Plans {
		pc := PlanCost{
			MonthlyCost:   plan.MonthlyCostUSD,
			TokensUsed:    totalTokens,
			TokensAllowed: plan.TokensPerMonth,
		}

		if plan.TokensPerMonth > 0 {
			pc.UtilizationPct = float64(totalTokens) / float64(plan.TokensPerMonth) * 100
			pc.CostPerToken = plan.MonthlyCostUSD / (float64(plan.TokensPerMonth) / 1_000_000)
		}

		if totalTokens > plan.TokensPerMonth {
			pc.Verdict = "over_limit"
		} else if plan.MonthlyCostUSD < apiCostUSD {
			pc.Verdict = "cheaper"
			savings := apiCostUSD - plan.MonthlyCostUSD
			if savings > bestSavings {
				bestSavings = savings
				bestPlan = planKey
			}
		} else {
			pc.Verdict = "more_expensive"
		}

		result.PlanComparisons[planKey] = pc
	}

	if bestPlan != "" {
		result.BestPlan = bestPlan
		result.BestPlanCostUSD = Plans[bestPlan].MonthlyCostUSD
		result.SavingsUSD = bestSavings
		if apiCostUSD > 0 {
			result.SavingsPct = (bestSavings / apiCostUSD) * 100
		}
	}

	return result
}
