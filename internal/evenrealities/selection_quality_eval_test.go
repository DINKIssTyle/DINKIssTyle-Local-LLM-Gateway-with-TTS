package evenrealities

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

// Opt-in behavioral evaluation against an actual OpenAI-compatible model.
// Uses production request assembly; ordinary unit tests never invoke a model.
func TestHintSourceQualityLive(t *testing.T) {
	endpoint, model := os.Getenv("DKST_HINT_SELECTION_LIVE_URL"), os.Getenv("DKST_HINT_SELECTION_LIVE_MODEL")
	if endpoint == "" || model == "" {
		t.Skip("set DKST_HINT_SELECTION_LIVE_URL (full chat completions URL) and DKST_HINT_SELECTION_LIVE_MODEL")
	}
	type source struct {
		title, url string
		texts      []string
	}
	for _, tc := range []struct {
		name, task string
		sources    []source
		record     int
		sentences  []int
	}{
		{"ads_and_essay_only", "문 손잡이가 힘을 줄이는 원리", []source{
			{"손잡이 이야기", "https://example.com/blog", []string{"손잡이를 돌릴 때마다 삶의 새로운 가능성을 생각하게 됩니다."}},
			{"손잡이 판매", "https://example.com/shop", []string{"우리 손잡이는 당신의 일상을 바꿀 최고의 선택입니다."}},
		}, 0, []int{}},
		{"vendor_fact_without_ad", "가상 장치 A의 최대 USB PD 입력 전력", []source{
			{"장치 A 사용 설명서", "https://example.com/vendor/manual", []string{"장치 A로 지금 최고의 경험을 시작하세요.", "장치 A의 USB PD 입력은 최대 65W입니다."}},
		}, 1, []int{2}},
		{"technical_blog_is_useful", "문 손잡이가 힘을 줄이는 원리", []source{
			{"건축 에세이", "https://example.com/essay", []string{"문은 우리의 마음을 열어주는 소중한 존재입니다."}},
			{"개발자의 물리 실험 노트", "https://example.com/blog/lever", []string{"회전축에서 먼 손잡이에 힘을 가하면 같은 회전력을 더 작은 힘으로 만들 수 있습니다."}},
		}, 2, []int{1}},
		{"independent_comparison", "가상 장치 A와 B 중 같은 조건에서 배터리가 오래가는 장치", []source{
			{"장치 A 공식 소개", "https://example.com/vendor", []string{"장치 A는 업계 최고의 배터리 경험을 제공합니다."}},
			{"독립 측정 보고서", "https://example.com/lab", []string{"같은 밝기와 작업 조건에서 장치 A는 8시간, 장치 B는 11시간 작동했습니다."}},
		}, 2, []int{1}},
		{"anecdote_is_not_comparison", "가상 장치 A와 B 중 같은 조건에서 배터리가 오래가는 장치", []source{
			{"장치 A 개인 후기", "https://example.com/blog/review", []string{"필자는 장치 A를 여행 중 하루 동안 충전 없이 사용해 만족했습니다."}},
		}, 0, []int{}},
		{"requested_personal_experience", "가상 장치 A를 여행에서 사용한 개인의 경험", []source{
			{"장치 A 개인 후기", "https://example.com/blog/review", []string{"필자는 장치 A를 여행 중 하루 동안 충전 없이 사용해 만족했습니다."}},
		}, 1, []int{1}},
		{"mixed_claim_cannot_be_separated", "가상 장치 A의 성능이 B보다 우수한지 확인", []source{
			{"장치 A 소개", "https://example.com/vendor", []string{"업계 최고의 장치 A는 어떤 경쟁 제품보다도 뛰어나므로 지금 구매하세요."}},
		}, 0, []int{}},
	} {
		for _, contextual := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/contextual=%t", tc.name, contextual), func(t *testing.T) {
				var choices []map[string]any
				wantSelection := 0
				for i, record := range tc.sources {
					for j, text := range record.texts {
						id := len(choices) + 1
						choices = append(choices, map[string]any{"selection": id, "record": i + 1, "sentence": j + 1, "title": record.title, "url": record.url, "text": text})
						if i+1 == tc.record && len(tc.sentences) == 1 && j+1 == tc.sentences[0] {
							wantSelection = id
						}
					}
				}
				body, err := PrepareSourceSelection(mustJSON(map[string]any{"model": model, "stream": false}), map[string]any{"task": tc.task, "already_heard": "", "choices": choices}, contextual)
				if err != nil {
					t.Fatal(err)
				}
				req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Content-Type", "application/json")
				if token := os.Getenv("DKST_HINT_SELECTION_LIVE_TOKEN"); token != "" {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				res, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				if res.StatusCode != http.StatusOK {
					t.Fatalf("model HTTP %d", res.StatusCode)
				}
				var reply struct {
					Choices []struct {
						Message struct{ Content string }
					}
				}
				if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&reply); err != nil || len(reply.Choices) == 0 {
					t.Fatalf("invalid model response: %v", err)
				}
				var actual map[string]any
				if err := json.Unmarshal([]byte(reply.Choices[0].Message.Content), &actual); err != nil {
					t.Fatalf("selection JSON missing: %v", err)
				}
				want := map[string]any{"selection": wantSelection}
				if contextual {
					want = map[string]any{"record": tc.record, "sentences": tc.sentences}
				}
				var normalized map[string]any
				_ = json.Unmarshal(mustJSON(want), &normalized)
				if !reflect.DeepEqual(actual, normalized) {
					t.Errorf("got %v, want %v", actual, normalized)
				}
			})
		}
	}
}
