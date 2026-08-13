package mcp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func requireLiveToolTests(t *testing.T) {
	t.Helper()
	if os.Getenv("DKST_LIVE_TOOL_TESTS") != "1" {
		t.Skip("set DKST_LIVE_TOOL_TESTS=1 to run network/browser tool smoke tests")
	}
}

func TestLiveSearchWeb(t *testing.T) {
	requireLiveToolTests(t)
	result, err := SearchWeb("OpenAI official site")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Title:") || !strings.Contains(result, "Link:") {
		t.Fatalf("web search returned no usable result cards:\n%s", result)
	}
}

func TestLiveSearchWebMulti(t *testing.T) {
	requireLiveToolTests(t)
	result, err := SearchWebMulti([]string{"OpenAI official site", "LM Studio official site"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Query 1:") || !strings.Contains(result, "Query 2:") || strings.Count(result, "Link:") < 2 {
		t.Fatalf("multi web search did not preserve both result sets:\n%s", result)
	}
}

func TestLiveSearchNaver(t *testing.T) {
	requireLiveToolTests(t)
	result, err := SearchNaver("오늘 주요 뉴스")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result) == "" {
		t.Fatal("Naver search returned empty content")
	}
}

func TestLiveReadPageWithDedicatedChromium(t *testing.T) {
	requireLiveToolTests(t)
	browserPath := strings.TrimSpace(os.Getenv("DKST_BROWSER_PATH"))
	if browserPath == "" {
		t.Skip("set DKST_BROWSER_PATH to the dedicated Chromium executable")
	}
	SetBrowserExecutablePath(browserPath)
	t.Cleanup(func() { SetBrowserExecutablePath("") })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html><html><body><script>
document.body.innerText = "DKST_DYNAMIC_BROWSER_SENTINEL " + "rendered ".repeat(80);
</script></body></html>`)
	}))
	defer server.Close()

	result, err := ReadPage(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "DKST_DYNAMIC_BROWSER_SENTINEL") {
		t.Fatalf("dedicated Chromium did not render the dynamic page:\n%s", result)
	}
}

func TestLiveSearchNamuwiki(t *testing.T) {
	requireLiveToolTests(t)
	result, err := SearchNamuwiki("대한민국")
	if err != nil {
		t.Fatal(err)
	}
	if len([]rune(strings.TrimSpace(result))) < 100 {
		t.Fatalf("Namuwiki returned too little content: %q", result)
	}
}
