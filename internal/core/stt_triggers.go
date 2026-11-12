package core

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type sttTriggerAlias struct {
	text  string
	runes []rune
}

func compactTrigger(text string) []rune {
	out := []rune{}
	for _, r := range norm.NFC.String(text) {
		if !unicode.IsSpace(r) {
			out = append(out, unicode.ToLower(r))
		}
	}
	return out
}

func parseSTTTriggers(configured string) ([]sttTriggerAlias, error) {
	aliases := []sttTriggerAlias{}
	seen := map[string]bool{}
	for _, value := range strings.FieldsFunc(configured, func(r rune) bool { return r == ',' || r == '，' || r == ';' || r == '\n' }) {
		value = strings.TrimSpace(value)
		key := compactTrigger(value)
		if len(key) == 0 {
			continue
		}
		if len(key) > 40 {
			return nil, fmt.Errorf("AI 호출어 하나는 40자 이하로 입력하세요")
		}
		if seen[string(key)] {
			continue
		}
		seen[string(key)] = true
		aliases = append(aliases, sttTriggerAlias{value, key})
		if len(aliases) > 8 {
			return nil, fmt.Errorf("AI 호출어는 최대 8개까지 등록할 수 있습니다")
		}
	}
	return aliases, nil
}

// Compare compacted NFC text while retaining source offsets for the question.
// Explicit aliases avoid broad fuzzy matching that could activate on unrelated speech.
func matchSTTTrigger(text string, aliases []sttTriggerAlias) (bool, string, string) {
	text = norm.NFC.String(text)
	chars := []rune{}
	starts, ends := []int{}, []int{}
	for offset, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		chars = append(chars, unicode.ToLower(r))
		starts = append(starts, offset)
		ends = append(ends, offset+utf8.RuneLen(r))
	}
	for start := range chars {
		length := 0
		for _, alias := range aliases {
			n := len(alias.runes)
			if start+n > len(chars) || n <= length {
				continue
			}
			matches := true
			for i, r := range alias.runes {
				if chars[start+i] != r {
					matches = false
					break
				}
			}
			if !matches {
				continue
			}
			// Latin wake words must be whole words ("day" must not match "holiday").
			if alias.runes[0] < 128 && unicode.IsLetter(alias.runes[0]) {
				before, _ := utf8.DecodeLastRuneInString(text[:starts[start]])
				after, _ := utf8.DecodeRuneInString(text[ends[start+n-1]:])
				if unicode.IsLetter(before) || unicode.IsNumber(before) || unicode.IsLetter(after) || unicode.IsNumber(after) {
					continue
				}
			}
			length = n
		}
		if length == 0 {
			continue
		}
		end := ends[start+length-1]
		prompt := strings.TrimLeftFunc(text[end:], func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) })
		if strings.HasPrefix(prompt, "야 ") || strings.HasPrefix(prompt, "아 ") {
			prompt = strings.TrimSpace(prompt[len("야"):])
		}
		return true, prompt, text[starts[start]:end]
	}
	return false, "", ""
}
