package mcp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type searchRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn searchRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func searchResponse(req *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

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

func TestDuckDuckGoChallengePageIsRejected(t *testing.T) {
	input := `<form id="challenge-form"><div class="anomaly-modal">Unfortunately, bots use DuckDuckGo too.</div></form>`
	if !isDuckDuckGoChallengePage(input) {
		t.Fatal("DuckDuckGo challenge page was not recognized")
	}
}

func TestParseBingRSSResults(t *testing.T) {
	input := `<?xml version="1.0"?><rss><channel>
<item><title>Example &amp; Guide</title><link>https://example.com/guide?utm_source=bing</link><description>Useful &lt;b&gt;summary&lt;/b&gt;.</description></item>
<item><title>Second</title><link>https://second.example/item</link><description>Second summary.</description></item>
</channel></rss>`
	results, err := parseBingRSSResults(input, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 Bing results, got %d: %#v", len(results), results)
	}
	if results[0].Title != "Example & Guide" || results[0].Link != "https://example.com/guide" || results[0].Snippet != "Useful summary." {
		t.Fatalf("unexpected Bing result: %#v", results[0])
	}
}

func TestParseGoogleNewsRSSPreservesPublisherAndDate(t *testing.T) {
	input := `<?xml version="1.0"?><rss><channel><item>
<title>Major AI offerings at a glance - Reuters</title>
<link>https://news.google.com/rss/articles/example?oc=5</link>
<description>Current AI model summary.</description>
<pubDate>Mon, 03 Aug 2026 10:21:08 GMT</pubDate>
<source url="https://www.reuters.com">Reuters</source>
</item></channel></rss>`
	results, err := parseGoogleNewsRSSResults(input, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one Google News result: %#v", results)
	}
	result := results[0]
	if result.Publisher != "Reuters" || result.SourceURL != "https://www.reuters.com" || result.PublishedAt == "" {
		t.Fatalf("publisher metadata was lost: %#v", result)
	}
	if searchResultQuality(result) != "reputable_news" {
		t.Fatalf("publisher quality was not preserved: %s", searchResultQuality(result))
	}
}

func TestCurrentUSNewsQueryFallsBackToParsedGoogleNewsRSS(t *testing.T) {
	client := &http.Client{Transport: searchRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "lite.duckduckgo.com":
			return searchResponse(req, `<form id="challenge-form"><div class="anomaly-modal">bots use DuckDuckGo too</div></form>`), nil
		case "news.google.com":
			return searchResponse(req, `<?xml version="1.0"?><rss><channel><item><title>미국 주요 정책 최신 뉴스 - Reuters</title><link>https://news.google.com/rss/articles/us-news</link><description>미국의 현재 주요 소식입니다.</description><pubDate>Mon, 03 Aug 2026 10:21:08 GMT</pubDate><source url="https://www.reuters.com">Reuters</source></item></channel></rss>`), nil
		default:
			return searchResponse(req, `<?xml version="1.0"?><rss><channel></channel></rss>`), nil
		}
	})}

	results, provider, err := searchWebWithProviders("현재 미국 뉴스", client)
	if err != nil {
		t.Fatal(err)
	}
	if provider != "google_news_rss" || len(results) != 1 {
		t.Fatalf("exact Korean query did not use parsed Google News evidence: provider=%q results=%#v", provider, results)
	}
	if results[0].Publisher != "Reuters" || results[0].PublishedAt == "" {
		t.Fatalf("parsed publisher/date metadata was lost: %#v", results[0])
	}
}

func TestFreshnessProviderErrorReportsGoogleNewsParserOutcome(t *testing.T) {
	client := &http.Client{Transport: searchRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "lite.duckduckgo.com" {
			return searchResponse(req, `<form id="challenge-form"><div class="anomaly-modal">challenge</div></form>`), nil
		}
		return searchResponse(req, `<?xml version="1.0"?><rss><channel></channel></rss>`), nil
	})}

	_, _, err := searchWebWithProviders("현재 미국 뉴스", client)
	if err == nil {
		t.Fatal("empty provider responses unexpectedly succeeded")
	}
	for _, expected := range []string{"duckduckgo:", "google_news_rss:", "Google News RSS returned no relevant parsed results", "bing_rss:"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("provider diagnostic omitted %q: %v", expected, err)
		}
	}
}

func TestSearchRelevanceFilterRemovesProviderNoise(t *testing.T) {
	results := []webSearchResult{
		{Title: "지방자치단체 인터넷원서접수센터", Snippet: "공무원 시험 접수 안내"},
		{Title: "Ten advances in mathematics", Publisher: "openai.com"},
		{Title: "로컬 LLM 생태계 업데이트", Snippet: "새 오픈 모델 소식"},
	}
	filtered := filterRelevantSearchResults("local LLM 생태계 최신 동향 2026", results)
	if len(filtered) != 1 || filtered[0].Title != "로컬 LLM 생태계 업데이트" {
		t.Fatalf("irrelevant provider noise was not removed: %#v", filtered)
	}
	if !containsKoreanText("로컬 LLM") || containsKoreanText("local LLM") {
		t.Fatal("Korean locale detection failed")
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

func TestFormattedSearchResultsExposeProviderAndRetrievalTime(t *testing.T) {
	formatted := formatSearchResultsWithGuidance("latest example", "duckduckgo", []webSearchResult{{
		Title:   "Example report",
		Link:    "https://example.com/report",
		Snippet: "Current details.",
	}})
	for _, expected := range []string{"Search Provider: duckduckgo", "Retrieved At:", "Link: https://example.com/report"} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("formatted search result omitted %q:\n%s", expected, formatted)
		}
	}
}

func TestFreshSearchGuidanceRequiresAuthoritativeRefinementForBlogs(t *testing.T) {
	formatted := formatSearchResultsWithGuidance("latest AI model news 2026", "duckduckgo", []webSearchResult{{
		Title: "SEO roundup", Link: "https://example.tistory.com/ai", Snippet: "Unverified claims.",
	}})
	for _, expected := range []string{"refine_search_for_authoritative_source", "no_authoritative_or_reputable_source", "Do not present these results as verified facts"} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("weak-source guidance omitted %q:\n%s", expected, formatted)
		}
	}

	official := formatSearchResultsWithGuidance("latest AI model news 2026", "duckduckgo", []webSearchResult{{
		Title: "Official update", Link: "https://openai.com/news/update", Snippet: "Primary-source details.",
	}})
	if strings.Contains(official, "no_authoritative_or_reputable_source") {
		t.Fatalf("official source was classified as weak:\n%s", official)
	}
	if got := classifySearchResultQuality("https://www.reuters.com/technology/ai/"); got != "reputable_news" {
		t.Fatalf("Reuters quality=%s", got)
	}
}

func TestSearchFormattingRanksHighQualityEvidenceFirst(t *testing.T) {
	formatted := formatSearchResultsWithGuidance("latest AI news 2026", "duckduckgo", []webSearchResult{
		{Title: "Blog", Link: "https://example.tistory.com/post", Snippet: "Discovery lead."},
		{Title: "Official", Link: "https://openai.com/index/release", Snippet: "Primary evidence."},
	})
	if strings.Index(formatted, "Title: Official") > strings.Index(formatted, "Title: Blog") {
		t.Fatalf("official evidence was not ranked ahead of a blog:\n%s", formatted)
	}
	if !strings.Contains(formatted, "Top Result Quality: authoritative") {
		t.Fatalf("ranked top-result quality was not updated:\n%s", formatted)
	}
}

func TestBufferedSearchHandleCarriesWeakSourceWarning(t *testing.T) {
	content := formatSearchResultsWithGuidance("latest AI model news 2026", "duckduckgo", []webSearchResult{{
		Title: "SEO roundup", Link: "https://example.tistory.com/ai", Snippet: "Unverified claims.",
	}})
	handle := formatBufferedSourceHandle(&BufferedWebSource{
		SourceID: "src_weak", ToolName: "search_web", Query: "latest AI model news 2026",
		Content: content, FetchedAt: time.Now(),
	})
	if !strings.Contains(handle, "refine_search_for_authoritative_source") || !strings.Contains(handle, "no_authoritative_or_reputable_source") {
		t.Fatalf("buffered preview lost weak-source guidance:\n%s", handle)
	}
}

func TestBufferedSearchHandleCarriesPageReadGuidance(t *testing.T) {
	content := formatSearchResultsWithGuidance("Go 1.25 official release notes", "duckduckgo", []webSearchResult{{
		Title: "Go 1.25 Release Notes", Link: "https://go.dev/doc/go1.25", Snippet: strings.Repeat("Official release note details. ", 12),
	}})
	handle := formatBufferedSourceHandle(&BufferedWebSource{
		SourceID: "src_official", ToolName: "search_web", Query: "Go 1.25 official release notes",
		Content: content, FetchedAt: time.Now(),
	})
	if !strings.Contains(handle, "read_top_result_if_more_detail_is_needed") {
		t.Fatalf("buffered preview lost page-read guidance:\n%s", handle)
	}
}

func TestBufferedSearchHandleReturnsActualLiveEvidence(t *testing.T) {
	content := formatSearchResultsWithGuidance("latest example", "duckduckgo", []webSearchResult{
		{Title: "First report", Link: "https://example.com/first", Snippet: "First current detail."},
		{Title: "Second report", Link: "https://example.com/second", Snippet: "Second current detail."},
	})
	handle := formatBufferedSourceHandle(&BufferedWebSource{
		SourceID:  "src_test",
		ToolName:  "search_web",
		Query:     "latest example",
		Content:   content,
		FetchedAt: time.Now(),
	})
	for _, expected := range []string{"Live Web Search Evidence", "Provider: duckduckgo", "Title: First report", "Link: https://example.com/first", "Snippet: First current detail."} {
		if !strings.Contains(handle, expected) {
			t.Fatalf("buffered search handle omitted %q:\n%s", expected, handle)
		}
	}
}

func TestBufferedParallelSearchHandleKeepsBothAngles(t *testing.T) {
	first := formatSearchResultsWithGuidance("first angle", "duckduckgo", []webSearchResult{{Title: "First", Link: "https://first.example/report", Snippet: "First evidence."}})
	second := formatSearchResultsWithGuidance("second angle", "bing_rss", []webSearchResult{{Title: "Second", Link: "https://second.example/report", Snippet: "Second evidence."}})
	content := "Parallel Web Search Results\n\n=== Query 1: first angle ===\n" + first + "\n\n=== Query 2: second angle ===\n" + second
	handle := formatBufferedSourceHandle(&BufferedWebSource{
		SourceID:  "src_multi",
		ToolName:  "search_web_multi",
		Query:     "first angle | second angle",
		Content:   content,
		FetchedAt: time.Now(),
	})
	for _, expected := range []string{"Search 1", "Query: first angle", "https://first.example/report", "Search 2", "Query: second angle", "https://second.example/report"} {
		if !strings.Contains(handle, expected) {
			t.Fatalf("parallel buffered evidence omitted %q:\n%s", expected, handle)
		}
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

func TestDefaultBrowserTimingHooksDelegateToPolicies(t *testing.T) {
	if got, want := defaultReadPageTimeoutForURL("https://platform.openai.com/docs"), 35*time.Second; got != want {
		t.Fatalf("documentation page timeout = %s, want %s", got, want)
	}
	if got, want := defaultReadPageTimeoutForURL("https://example.com"), 25*time.Second; got != want {
		t.Fatalf("default page timeout = %s, want %s", got, want)
	}
	if got, want := defaultChallengeWaitIterations(25*time.Second), 9; got != want {
		t.Fatalf("challenge wait iterations = %d, want %d", got, want)
	}
}

func TestDynamicSitesPreferBrowserPageRead(t *testing.T) {
	for _, host := range []string{"www.msn.com", "weather.msn.com", "www.naver.com"} {
		if !requiresBrowserPageRead(host) {
			t.Fatalf("dynamic host %q did not prefer browser page reading", host)
		}
	}
	if requiresBrowserPageRead("example.com") {
		t.Fatal("static example host unexpectedly required browser page reading")
	}
}

func TestMSNPageReadinessWaitsForWeatherFields(t *testing.T) {
	msnExpression := pageReadinessExpression("https://www.msn.com/ko-kr/weather/forecast/in-Busan,Busan")
	for _, marker := range []string{"hasTemperature", "hasWeatherField"} {
		if !strings.Contains(msnExpression, marker) {
			t.Fatalf("MSN readiness expression omitted %q", marker)
		}
	}
	staticExpression := pageReadinessExpression("https://example.com/article")
	if !strings.Contains(staticExpression, "if (!false) return true") {
		t.Fatal("static readiness expression unexpectedly required weather fields")
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
