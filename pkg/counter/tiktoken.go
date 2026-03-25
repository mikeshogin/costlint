package counter

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// CountAccurate estimates tokens using a more accurate method than word-based.
// Uses character-level heuristics that approximate cl100k_base tokenizer behavior.
//
// Rules (approximating GPT/Claude tokenization):
// - Common English words: ~1 token per word
// - Code tokens: operators and punctuation are often 1 token each
// - Numbers: each digit group is ~1 token
// - Whitespace: usually merged with adjacent token
// - Non-ASCII (Russian, Chinese, etc.): ~1.5-2 tokens per word
// - Code identifiers: camelCase splits into ~2-3 tokens
func CountAccurate(text string) TokenCount {
	if len(text) == 0 {
		return TokenCount{}
	}

	tokens := 0
	i := 0
	runes := []rune(text)

	for i < len(runes) {
		r := runes[i]

		// Skip whitespace (merged with next token)
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// Numbers: digit groups
		if unicode.IsDigit(r) {
			for i < len(runes) && unicode.IsDigit(runes[i]) {
				i++
			}
			tokens++
			continue
		}

		// Punctuation and operators: usually 1 token each
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			tokens++
			i++
			continue
		}

		// Words
		if unicode.IsLetter(r) {
			wordStart := i
			for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			word := string(runes[wordStart:i])

			// Count tokens for this word
			tokens += estimateWordTokens(word)
			continue
		}

		// Anything else: 1 token
		tokens++
		i++
	}

	// Minimum 1 token for non-empty text
	if tokens == 0 && len(text) > 0 {
		tokens = 1
	}

	return TokenCount{
		Input: tokens,
		Total: tokens,
	}
}

// estimateWordTokens estimates how many tokens a single word becomes.
func estimateWordTokens(word string) int {
	if len(word) == 0 {
		return 0
	}

	// Check if word is ASCII (English)
	isASCII := true
	for _, r := range word {
		if r > 127 {
			isASCII = false
			break
		}
	}

	if !isASCII {
		// Non-ASCII: roughly 1 token per 2-3 characters (UTF-8 encoded)
		byteLen := len([]byte(word))
		charLen := utf8.RuneCountInString(word)
		// Russian/Cyrillic: ~1 token per 2 chars
		// CJK: ~1 token per 1-2 chars
		return max(1, byteLen/3)
		_ = charLen
	}

	// Short common English words: 1 token
	if len(word) <= 6 {
		return 1
	}

	// camelCase splitting
	splits := countCamelCaseSplits(word)
	if splits > 1 {
		return splits
	}

	// snake_case splitting
	if strings.Contains(word, "_") {
		parts := strings.Split(word, "_")
		count := 0
		for _, p := range parts {
			if len(p) > 0 {
				count++
			}
		}
		return max(1, count)
	}

	// Long words: roughly 1 token per 4 characters
	if len(word) > 12 {
		return len(word) / 4
	}

	// Medium words: 1-2 tokens
	if len(word) > 8 {
		return 2
	}

	return 1
}

func countCamelCaseSplits(word string) int {
	splits := 1
	runes := []rune(word)
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) && i > 0 && unicode.IsLower(runes[i-1]) {
			splits++
		}
	}
	return splits
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
