package counter

import (
	"strings"
	"unicode"
)

// TokenCount holds input and output token estimates.
type TokenCount struct {
	Input  int `json:"input_tokens"`
	Output int `json:"output_tokens"`
	Total  int `json:"total_tokens"`
}

// Count estimates token count for a text string.
// Uses word-based approximation (1 token ~ 0.75 words for English).
// For production use, integrate tiktoken or cl100k_base tokenizer.
func Count(text string) TokenCount {
	words := countWords(text)
	// Approximation: ~1.33 tokens per word (English average)
	tokens := int(float64(words) * 1.33)
	if tokens < 1 && len(text) > 0 {
		tokens = 1
	}
	return TokenCount{
		Input: tokens,
		Total: tokens,
	}
}

// EstimateOutput estimates output tokens based on input and task type.
func EstimateOutput(inputTokens int, taskType string) int {
	switch taskType {
	case "fix":
		return int(float64(inputTokens) * 0.5) // fixes tend to be shorter
	case "create":
		return int(float64(inputTokens) * 2.0) // creation produces more output
	case "review":
		return int(float64(inputTokens) * 0.3) // reviews are concise
	case "question":
		return int(float64(inputTokens) * 1.5) // answers can be verbose
	default:
		return inputTokens // 1:1 ratio as default
	}
}

func countWords(s string) int {
	inWord := false
	count := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			if inWord {
				count++
				inWord = false
			}
		} else {
			inWord = true
		}
	}
	if inWord {
		count++
	}
	return count
}

// CountWithContext estimates tokens including system prompt and context.
func CountWithContext(prompt, systemPrompt string, contextFiles []string) TokenCount {
	promptCount := Count(prompt)
	systemCount := Count(systemPrompt)

	contextTokens := 0
	for _, content := range contextFiles {
		c := Count(content)
		contextTokens += c.Input
	}

	total := promptCount.Input + systemCount.Input + contextTokens
	return TokenCount{
		Input: total,
		Total: total,
	}
}

// FormatCount returns a human-readable token count string.
func FormatCount(tc TokenCount) string {
	if tc.Total >= 1000000 {
		return formatFloat(float64(tc.Total)/1000000) + "M"
	}
	if tc.Total >= 1000 {
		return formatFloat(float64(tc.Total)/1000) + "K"
	}
	return strings.TrimRight(strings.TrimRight(formatFloat(float64(tc.Total)), "0"), ".")
}

func formatFloat(f float64) string {
	s := strings.TrimRight(strings.TrimRight(
		strings.Replace(
			strings.Replace(
				formatDecimal(f), ",", "", -1,
			), " ", "", -1,
		), "0"), ".")
	return s
}

func formatDecimal(f float64) string {
	if f == float64(int(f)) {
		return strings.Replace(
			strings.TrimRight(strings.TrimRight(
				strings.Replace(
					formatInt(int(f)), "", "", 0,
				), "0"), "."),
			"", "", 0)
	}
	// Simple formatting
	whole := int(f)
	frac := int((f - float64(whole)) * 100)
	if frac == 0 {
		return formatInt(whole)
	}
	return formatInt(whole) + "." + formatInt(frac)
}

func formatInt(n int) string {
	s := ""
	if n == 0 {
		return "0"
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
