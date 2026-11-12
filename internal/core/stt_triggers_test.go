package core

import "testing"

func TestSTTTriggerAliases(t *testing.T) {
	aliases, err := parseSTTTriggers("웬즈데이, 웬즈 데이, 웸즈 데이, Wednesday")
	if err != nil || len(aliases) != 3 {
		t.Fatalf("normalized aliases: %+v %v", aliases, err)
	}
	for _, tc := range []struct {
		text, prompt, matched string
		want                  bool
	}{
		{"웬즈 데이 오늘 일정 알려줘", "오늘 일정 알려줘", "웬즈 데이", true},
		{"웸즈 데이, 지금 몇 시야?", "지금 몇 시야?", "웸즈 데이", true},
		{"웬즈데이야 날씨 알려줘", "날씨 알려줘", "웬즈데이", true},
		{"WEDNESDAY, hello", "hello", "WEDNESDAY", true},
		{"웬즈 데이", "", "웬즈 데이", true},
		{"오늘 회의는 세 시입니다", "", "", false},
		{"웬스데이 오늘 일정", "", "", false},
	} {
		got, prompt, matched := matchSTTTrigger(tc.text, aliases)
		if got != tc.want || prompt != tc.prompt || matched != tc.matched {
			t.Errorf("%q: %v %q %q", tc.text, got, prompt, matched)
		}
	}
	short, _ := parseSTTTriggers("day")
	if ok, _, _ := matchSTTTrigger("holiday today", short); ok {
		t.Error("substring activated Latin wake word")
	}
}
