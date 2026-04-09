package mcp

import (
	"strings"
	"unicode"
)

const ftsIndexVersion = "cjk_bigram_v1"

func buildFTSIndexedText(text string) string {
	tokens := buildFTSIndexTokens(text)
	if len(tokens) == 0 {
		return strings.TrimSpace(strings.ToLower(text))
	}
	return strings.Join(tokens, " ")
}

func buildFTSQueryClauses(query string) []string {
	fields := splitSearchFields(query)
	if len(fields) == 0 {
		return nil
	}

	clauses := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = normalizeSearchField(field)
		if field == "" {
			continue
		}
		clauseKey := field
		if hasEastAsianRune(field) {
			clauseKey = "cjk:" + field
		}
		if _, exists := seen[clauseKey]; exists {
			continue
		}
		seen[clauseKey] = struct{}{}

		if hasEastAsianRune(field) {
			grams := uniqueRuneNGrams(field, 2)
			switch len(grams) {
			case 0:
				continue
			case 1:
				clauses = append(clauses, quoteFTSPhrase(grams[0]))
			default:
				parts := make([]string, 0, len(grams))
				for _, gram := range grams {
					parts = append(parts, quoteFTSPhrase(gram))
				}
				clauses = append(clauses, "("+strings.Join(parts, " AND ")+")")
			}
			continue
		}

		clauses = append(clauses, quoteFTSPhrase(field))
	}
	return clauses
}

func buildFTSIndexTokens(text string) []string {
	fields := splitSearchFields(text)
	if len(fields) == 0 {
		return nil
	}

	tokens := make([]string, 0, len(fields)*2)
	seen := make(map[string]struct{}, len(fields)*2)
	for _, field := range fields {
		field = normalizeSearchField(field)
		if field == "" {
			continue
		}
		appendUniqueToken(&tokens, seen, field)
		if hasEastAsianRune(field) {
			for _, gram := range uniqueRuneNGrams(field, 2) {
				appendUniqueToken(&tokens, seen, gram)
			}
		}
	}
	return tokens
}

func tokenizeQuery(query string) []string {
	return buildFTSIndexTokens(query)
}

func splitSearchFields(text string) []string {
	return strings.FieldsFunc(strings.TrimSpace(text), func(r rune) bool {
		return !isSearchTokenRune(r)
	})
}

func isSearchTokenRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

func normalizeSearchField(field string) string {
	field = strings.ToLower(strings.TrimSpace(field))
	if runeLen(field) < 2 {
		return ""
	}
	return field
}

func hasEastAsianRune(text string) bool {
	for _, r := range text {
		if unicode.In(r, unicode.Hangul, unicode.Han, unicode.Hiragana, unicode.Katakana) {
			return true
		}
	}
	return false
}

func uniqueRuneNGrams(text string, size int) []string {
	runes := []rune(text)
	if len(runes) < size || size <= 0 {
		return nil
	}

	out := make([]string, 0, len(runes)-size+1)
	seen := make(map[string]struct{}, len(runes)-size+1)
	for i := 0; i <= len(runes)-size; i++ {
		gram := string(runes[i : i+size])
		if _, exists := seen[gram]; exists {
			continue
		}
		seen[gram] = struct{}{}
		out = append(out, gram)
	}
	return out
}

func appendUniqueToken(tokens *[]string, seen map[string]struct{}, token string) {
	if token == "" {
		return
	}
	if _, exists := seen[token]; exists {
		return
	}
	seen[token] = struct{}{}
	*tokens = append(*tokens, token)
}

func runeLen(text string) int {
	return len([]rune(text))
}
