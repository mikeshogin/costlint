package pricing

// Model represents an LLM model with pricing information.
type Model struct {
	Name           string  `json:"name"`
	InputPerMToken float64 `json:"input_per_m_token"`  // $ per 1M input tokens
	OutputPerMToken float64 `json:"output_per_m_token"` // $ per 1M output tokens
}

// Default pricing table (Anthropic API, as of 2026-03).
var Models = map[string]Model{
	"haiku": {
		Name:            "claude-haiku-4-5",
		InputPerMToken:  0.80,
		OutputPerMToken: 4.00,
	},
	"sonnet": {
		Name:            "claude-sonnet-4-6",
		InputPerMToken:  3.00,
		OutputPerMToken: 15.00,
	},
	"opus": {
		Name:            "claude-opus-4-6",
		InputPerMToken:  15.00,
		OutputPerMToken: 75.00,
	},
}

// Estimate calculates cost in USD for given token counts.
func Estimate(modelKey string, inputTokens, outputTokens int) float64 {
	model, ok := Models[modelKey]
	if !ok {
		return 0
	}
	inputCost := float64(inputTokens) / 1_000_000 * model.InputPerMToken
	outputCost := float64(outputTokens) / 1_000_000 * model.OutputPerMToken
	return inputCost + outputCost
}

// CompareModels returns cost estimates for the same token counts across all models.
func CompareModels(inputTokens, outputTokens int) map[string]float64 {
	result := make(map[string]float64)
	for key := range Models {
		result[key] = Estimate(key, inputTokens, outputTokens)
	}
	return result
}

// Savings calculates how much would be saved by routing to a cheaper model.
func Savings(fromModel, toModel string, inputTokens, outputTokens int) float64 {
	fromCost := Estimate(fromModel, inputTokens, outputTokens)
	toCost := Estimate(toModel, inputTokens, outputTokens)
	if toCost >= fromCost {
		return 0
	}
	return fromCost - toCost
}

// SavingsPercent calculates percentage savings.
func SavingsPercent(fromModel, toModel string, inputTokens, outputTokens int) float64 {
	fromCost := Estimate(fromModel, inputTokens, outputTokens)
	if fromCost == 0 {
		return 0
	}
	saved := Savings(fromModel, toModel, inputTokens, outputTokens)
	return (saved / fromCost) * 100
}
