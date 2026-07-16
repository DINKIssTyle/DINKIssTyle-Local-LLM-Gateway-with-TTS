package mcp

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseDuckDuckGoResultsKeepsRowsAlignedAndNormalizesURLs(t *testing.T) {
	input := `<html><body><table>
<tr><td><a class='result-link' href='//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fguide%3Futm_source%3Dddg'>Example Guide</a></td></tr>
<tr><td class='result-snippet'>First snippet</td></tr>
<tr><td><a class='result-link' href='https://second.example/news'>Second Result</a></td></tr>
<tr><td class='result-snippet'>Second snippet</td></tr>
</table></body></html>`

	results, err := parseDuckDuckGoResults(input, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %#v", len(results), results)
	}
	if results[0].Link != "https://example.com/guide" {
		t.Fatalf("unexpected normalized URL: %q", results[0].Link)
	}
	if results[0].Snippet != "First snippet" || results[1].Snippet != "Second snippet" {
		t.Fatalf("snippets were not kept with their rows: %#v", results)
	}
}

func TestParseNaverSearchResultsExtractsCompactCards(t *testing.T) {
	input := `<html><body><div class='card'>
<a class='news_tit' href='https://news.example/article?utm_medium=portal'>기사 제목</a>
<div class='news_dsc'>기사 요약 내용입니다.</div>
</div></body></html>`

	results, err := parseNaverSearchResults(input, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Link != "https://news.example/article" || results[0].Snippet != "기사 요약 내용입니다." {
		t.Fatalf("unexpected result: %#v", results[0])
	}
}

func TestClassifySearchResultQualityUsesDomainBoundaries(t *testing.T) {
	if got := classifySearchResultQuality("https://agency.gov.example.com/page"); got == "authoritative" {
		t.Fatalf("lookalike domain classified as authoritative")
	}
	if got := classifySearchResultQuality("https://data.example.gov/report"); got != "authoritative" {
		t.Fatalf("real gov suffix not classified as authoritative: %s", got)
	}
	if got := classifySearchResultQuality("https://notwikipedia.org/page"); got == "encyclopedic" {
		t.Fatalf("lookalike Wikipedia domain classified as encyclopedic")
	}
}

func TestSearchCacheTTLReflectsFreshness(t *testing.T) {
	if got := searchCacheTTLForQuery("서울 오늘 날씨"); got != time.Minute {
		t.Fatalf("volatile query TTL = %s", got)
	}
	if got := searchCacheTTLForQuery("Go 공식 문서"); got != 30*time.Minute {
		t.Fatalf("stable query TTL = %s", got)
	}
}

func TestSearchWebMultiRunsBothQueriesConcurrentlyAndKeepsOrder(t *testing.T) {
	var active int32
	var peak int32
	search := func(query string) (string, error) {
		current := atomic.AddInt32(&active, 1)
		for {
			observed := atomic.LoadInt32(&peak)
			if current <= observed || atomic.CompareAndSwapInt32(&peak, observed, current) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return "result for " + query, nil
	}

	result, err := searchWebMultiWith([]string{"first angle", "second angle"}, search)
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&peak) != 2 {
		t.Fatalf("expected two concurrent searches, peak concurrency was %d", peak)
	}
	firstIndex := strings.Index(result, "Query 1: first angle")
	secondIndex := strings.Index(result, "Query 2: second angle")
	if firstIndex < 0 || secondIndex < 0 || firstIndex >= secondIndex {
		t.Fatalf("results did not preserve input order:\n%s", result)
	}
}

func TestSearchWebMultiReturnsPartialEvidence(t *testing.T) {
	result, err := searchWebMultiWith([]string{"working", "failing"}, func(query string) (string, error) {
		if query == "failing" {
			return "", errors.New("provider unavailable")
		}
		return fmt.Sprintf("evidence for %s", query), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "evidence for working") || !strings.Contains(result, "1 of 2 searches failed") {
		t.Fatalf("partial evidence or disclosure missing:\n%s", result)
	}
}

func TestSearchWebMultiRejectsDuplicateOrWrongQueryCount(t *testing.T) {
	search := func(query string) (string, error) { return query, nil }
	for _, queries := range [][]string{{"one"}, {"same", " SAME "}, {"one", "two", "three"}} {
		if _, err := searchWebMultiWith(queries, search); err == nil {
			t.Fatalf("expected invalid query set to fail: %#v", queries)
		}
	}
}

func TestRecentBufferedSourcesMemoryRanksGloballyWithDiversity(t *testing.T) {
	userID := "test-global-ranking"
	now := time.Now()
	webBufferMu.Lock()
	webBuffers[userID] = &userWebBuffer{
		Sources: map[string]*BufferedWebSource{
			"weak":   {SourceID: "weak", UserID: userID, Title: "Unrelated", Summary: "other material", Chunks: []BufferedWebChunk{{Index: 0, Text: "unrelated text"}}, FetchedAt: now},
			"best":   {SourceID: "best", UserID: userID, Title: "Alpha report", Summary: "alpha alpha", Chunks: []BufferedWebChunk{{Index: 0, Text: "alpha alpha alpha primary evidence"}, {Index: 1, Text: "alpha extra evidence"}}, FetchedAt: now.Add(-time.Minute)},
			"second": {SourceID: "second", UserID: userID, Title: "Alpha confirmation", Summary: "alpha", Chunks: []BufferedWebChunk{{Index: 0, Text: "alpha independent confirmation"}}, FetchedAt: now.Add(-2 * time.Minute)},
		},
		Order: []string{"best", "second", "weak"},
	}
	webBufferMu.Unlock()
	t.Cleanup(func() {
		webBufferMu.Lock()
		delete(webBuffers, userID)
		webBufferMu.Unlock()
	})

	result, err := readRecentBufferedSourcesMemory(userID, nil, "alpha", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Source ID: best") || !strings.Contains(result, "Source ID: second") {
		t.Fatalf("expected two relevant sources, got:\n%s", result)
	}
	if strings.Contains(result, "Source ID: weak") {
		t.Fatalf("unrelated recent source consumed evidence budget:\n%s", result)
	}
}
