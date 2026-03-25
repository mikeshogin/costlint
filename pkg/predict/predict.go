package predict

import (
	"fmt"
	"strings"

	"github.com/mshogin/costlint/pkg/counter"
	"github.com/mshogin/costlint/pkg/pricing"
)

type Prediction struct {
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

func Predict(prompt string) []Prediction {
	tc := counter.CountAccurate(prompt)
	outputEstimate := tc.Input * 2

	var predictions []Prediction
	for _, model := range []string{"haiku", "sonnet", "opus"} {
		cost := pricing.Estimate(model, tc.Input, outputEstimate)
		predictions = append(predictions, Prediction{
			Model:        model,
			InputTokens:  tc.Input,
			OutputTokens: outputEstimate,
			TotalTokens:  tc.Input + outputEstimate,
			CostUSD:      cost,
		})
	}
	return predictions
}

func FormatPredictions(preds []Prediction) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-10s %-12s %-12s %-10s\n", "Model", "Input(est)", "Output(est)", "Cost(est)")
	fmt.Fprintf(&b, "%-10s %-12s %-12s %-10s\n", "-----", "----------", "-----------", "---------")
	for _, p := range preds {
		fmt.Fprintf(&b, "%-10s %-12s %-12s $%-9.4f\n",
			p.Model, formatTokens(p.InputTokens), formatTokens(p.OutputTokens), p.CostUSD)
	}
	return b.String()
}

func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("~%dK", n/1000)
	}
	return fmt.Sprintf("~%d", n)
}
