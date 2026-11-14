package core

import (
	"fmt"
	"strings"
	"testing"
)

func TestHintKeywordDictionaryAndParticles(t *testing.T) {
	for _, text := range []string{"청동기 시대에는 청동으로 도구를 만들었대.", "피라미드가 왕의 무덤이래.", "맥북 에어 M4의 무게를 이야기했어.", "국가에서는 정책을 발표했대.", "국가는 중요한 개념이지.", "SQLite는 로컬 데이터베이스야.", "EmbeddingGemma 2로 자료를 찾았어."} {
		candidates := hintKeywords(text)
		if len(candidates) == 0 {
			t.Fatal(text)
		}
		for _, c := range candidates {
			if !strings.Contains(text, c.Surface) || !strings.Contains(c.Evidence, c.Surface) {
				t.Fatalf("lost evidence: %+v", c)
			}
		}
	}
	for _, tc := range []struct{ text, want string }{{"청동기 시대에는 청동으로 도구를 만들었대.", "청동기 시대"}, {"피라미드가 왕의 무덤이래.", "피라미드"}, {"국가", "국가"}, {"맥북 에어 M4의 무게", "맥북 에어 M4"}} {
		found := false
		for _, c := range hintKeywords(tc.text) {
			if normalizeHintKeyword(c.Term) == normalizeHintKeyword(tc.want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%q missing from %+v", tc.want, hintKeywords(tc.text))
		}
	}
	for _, c := range hintKeywords("국가") {
		if c.Term == "국" {
			t.Fatal("particle stripping corrupted known name")
		}
	}
	if len(hintLexicon.rows) < 1000000 {
		t.Fatalf("incomplete title index: %d", len(hintLexicon.rows))
	}
	if n := len(hintKeywords(strings.Repeat("청동기 시대 피라미드 맥북 에어 M4 SQLite RAG MCP ", 30))); n > 6 {
		t.Fatalf("unbounded candidates: %d", n)
	}
}

func TestKeywordDecisionUsesObservedContextAndShownFacts(t *testing.T) {
	req := glassesHintRequest{Text: "피라미드가 왕실 무덤이었다고 하더라.", ShownInfo: "피라미드는 왕실 무덤으로 건립되었습니다."}
	d, ok, err := decideKeywordHint(req, func(text string, criteria map[string]string, instructions string) (jevReply, error) {
		if !strings.Contains(text, req.Text) || !strings.Contains(text, req.ShownInfo) || !strings.Contains(instructions, "다른 사실") {
			t.Fatal(text, instructions)
		}
		for key, description := range criteria {
			if strings.HasSuffix(description, ": 피라미드") {
				return jevReply{Selected: key, Probabilities: map[string]float64{key: .85, "none": .15}}, nil
			}
		}
		return jevReply{}, fmt.Errorf("expected dictionary candidate")
	})
	if err != nil || !ok || !d.Hint || d.Keyword != "피라미드" || d.KeywordSurface != "피라미드" || !strings.Contains(d.Focus, "왕실 무덤") {
		t.Fatalf("%+v %v %v", d, ok, err)
	}
	_, ok, err = decideKeywordHint(req, func(string, map[string]string, string) (jevReply, error) {
		return jevReply{Selected: "none", Probabilities: map[string]float64{"none": .99, "focus_0": .01}}, nil
	})
	if err != nil || ok {
		t.Fatal("dictionary membership must not force a hint", ok, err)
	}
}

func BenchmarkHintKeywordExtraction(b *testing.B) {
	lookupHintKeyword("피라미드")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hintKeywords("피라미드가 왕실 무덤이래. 청동기 시대에는 청동을 사용했대. 맥북 에어 M4의 무게도 궁금하네.")
	}
}
