package counter

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// CountAccurate estimates tokens using cl100k_base tokenizer approximation.
// This is the encoding used by GPT-4 and Claude models.
//
// Key rules of cl100k_base:
// - Whitespace is often merged with the following token
// - Common English words of length <= 3 are usually 1 token
// - Longer words are split on case transitions, special chars, subword boundaries
// - Punctuation: most are 1 token each; some sequences merge (e.g., "...", "->")
// - Numbers: digit runs are 1 token up to ~3-4 digits; longer runs split
// - Non-ASCII (Cyrillic, CJK): each char is ~1-3 bytes in UTF-8 and maps to ~1 token per char
// - Newlines / special whitespace: each newline is typically 1 token
// - Code: operators like "==", "!=", "<=", ">=" are 1 token each
//
// Accuracy target: within 10% of real tiktoken for typical English/code text.
func CountAccurate(text string) TokenCount {
	if len(text) == 0 {
		return TokenCount{}
	}

	tokens := tokenize(text)

	if tokens == 0 && len(text) > 0 {
		tokens = 1
	}

	return TokenCount{
		Input: tokens,
		Total: tokens,
	}
}

// tokenize is the core tokenization approximation.
func tokenize(text string) int {
	tokens := 0
	runes := []rune(text)
	n := len(runes)
	i := 0

	for i < n {
		r := runes[i]

		// Newlines: each newline is ~1 token (sometimes merged with space)
		if r == '\n' {
			tokens++
			i++
			// Consume following spaces/tabs that are merged with the newline
			for i < n && (runes[i] == ' ' || runes[i] == '\t') {
				i++
			}
			continue
		}

		// Spaces: leading spaces before words/tokens often merge with them in cl100k.
		// A run of spaces is usually 1 token (up to ~4 spaces merge together).
		if r == ' ' || r == '\t' {
			// Count the run of spaces
			spaceCount := 0
			for i < n && (runes[i] == ' ' || runes[i] == '\t') {
				spaceCount++
				i++
			}
			// In cl100k_base, spaces merge with the *next* token.
			// So we only count extra space tokens if there are many spaces
			// (e.g., code indentation). 1-4 spaces = 0 extra tokens (merged).
			// Each additional group of ~4 spaces adds a token.
			extraSpaceTokens := spaceCount / 4
			tokens += extraSpaceTokens
			continue
		}

		// Carriage return
		if r == '\r' {
			i++
			continue
		}

		// Numbers: digit runs. cl100k splits long digit runs.
		// 1-3 digits = 1 token, then roughly 1 token per 2-3 additional digits.
		if unicode.IsDigit(r) {
			start := i
			for i < n && unicode.IsDigit(runes[i]) {
				i++
			}
			digitLen := i - start
			// Handle decimal point or comma as part of number
			if i < n && (runes[i] == '.' || runes[i] == ',') && i+1 < n && unicode.IsDigit(runes[i+1]) {
				i++ // consume the separator
				for i < n && unicode.IsDigit(runes[i]) {
					i++
					digitLen++
				}
			}
			// cl100k tokenizes numbers roughly: every 1-3 digits = 1 token
			tokens += max(1, (digitLen+2)/3)
			continue
		}

		// Multi-char operator sequences (common in code)
		if i+1 < n {
			twoChar := string(runes[i : i+2])
			switch twoChar {
			case "==", "!=", "<=", ">=", "->", "<-", "=>", "::", "//", "/*", "*/",
				"++", "--", "&&", "||", "<<", ">>", "**", "..", ":=":
				tokens++
				i += 2
				continue
			}
		}
		// Three-char sequences
		if i+2 < n {
			threeChar := string(runes[i : i+3])
			switch threeChar {
			case "...", "/**", "```":
				tokens++
				i += 3
				continue
			}
		}

		// Punctuation and symbols: 1 token each
		if unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.IsMark(r) {
			tokens++
			i++
			continue
		}

		// Letters: collect a word segment
		if unicode.IsLetter(r) {
			wordStart := i
			for i < n && (unicode.IsLetter(runes[i]) || (runes[i] == '_' && i > wordStart)) {
				i++
			}
			word := string(runes[wordStart:i])

			// Handle snake_case: split on underscores
			if strings.Contains(word, "_") {
				parts := strings.Split(word, "_")
				for _, p := range parts {
					if len(p) > 0 {
						tokens += estimateWordTokens(p)
					}
				}
				continue
			}

			tokens += estimateWordTokens(word)
			continue
		}

		// Anything else (control chars, etc.): 1 token
		tokens++
		i++
	}

	return tokens
}

// estimateWordTokens estimates how many cl100k tokens a single word segment becomes.
// Does not handle snake_case (caller should split first).
func estimateWordTokens(word string) int {
	if len(word) == 0 {
		return 0
	}

	runes := []rune(word)

	// Check if all ASCII
	isASCII := utf8.RuneCountInString(word) == len(word)

	if !isASCII {
		// Non-ASCII (Cyrillic, CJK, Arabic, etc.)
		// In cl100k_base: Cyrillic letters are each typically 1-2 tokens.
		// Conservative estimate: 1 token per character for CJK (single-byte in BPE sense)
		// and 1 token per 1-2 chars for Cyrillic.
		charCount := utf8.RuneCountInString(word)
		byteCount := len(word) // UTF-8 byte count

		if byteCount >= charCount*3 {
			// Likely CJK (3 bytes per char): ~1 token per char
			return max(1, charCount)
		}
		// Cyrillic / other 2-byte scripts: ~1.5 tokens per char -> roughly 1 per 2 chars + 1
		return max(1, (charCount+1)/2+1)
	}

	// ASCII word
	wordLen := len(runes)

	// Very short words (1-3 chars): always 1 token
	if wordLen <= 3 {
		return 1
	}

	// Short common words (4-6 chars): almost always 1 token
	if wordLen <= 6 {
		return 1
	}

	// camelCase splitting: each transition from lower to upper starts a new token
	camelParts := splitCamelCase(word)
	if len(camelParts) > 1 {
		total := 0
		for _, part := range camelParts {
			total += estimateWordTokens(part)
		}
		return total
	}

	// Longer pure ASCII words: BPE tends to split them
	// Empirically, for English text cl100k handles common words well.
	// For uncommon/long words, ~1 token per 4 chars.
	if wordLen <= 8 {
		return 1
	}
	if wordLen <= 12 {
		return 2
	}
	// Very long words
	return max(2, wordLen/4)
}

// splitCamelCase splits a word on camelCase boundaries.
// "camelCase" -> ["camel", "Case"]
// "HTTPRequest" -> ["HTTP", "Request"]
func splitCamelCase(word string) []string {
	runes := []rune(word)
	n := len(runes)
	var parts []string
	start := 0

	for i := 1; i < n; i++ {
		// Lower to Upper transition: "camelCase" -> split before 'C'
		if unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i]) {
			parts = append(parts, string(runes[start:i]))
			start = i
			continue
		}
		// Upper to Lower transition when preceded by multiple uppers: "HTTPRequest" -> "HTTP", "Request"
		if i >= 2 && unicode.IsUpper(runes[i-1]) && unicode.IsUpper(runes[i-2]) && unicode.IsLower(runes[i]) {
			parts = append(parts, string(runes[start:i-1]))
			start = i - 1
		}
	}

	if start < n {
		parts = append(parts, string(runes[start:]))
	}

	if len(parts) <= 1 {
		return []string{word}
	}
	return parts
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
