package vjudge

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDebugCF2232C2List(t *testing.T) {
	fetcher := func(ctx context.Context, url string) (string, error) {
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body), nil
	}

	// This test scrapes a live Codeforces page and asserts on its markup, so it
	// depends on a remote site that changes without notice (and is blocked from
	// most CI runners). Skip rather than fail when the page cannot be fetched:
	// a network or remote-markup change is not a defect in this repository.
	html, fetchErr := fetcher(context.Background(), "https://codeforces.com/contest/2232/problem/C2")
	if fetchErr != nil || !strings.Contains(html, "problem-statement") {
		t.Skipf("codeforces unreachable or markup changed; skipping live scrape test (err=%v)", fetchErr)
	}

	// Find the list in raw HTML
	idx := strings.Index(html, "<ul>")
	if idx > 0 {
		end := idx + 500
		if end > len(html) {
			end = len(html)
		}
		fmt.Println("=== RAW HTML UL ===")
		fmt.Println(html[idx:end])
	}

	parser := NewProblemParser(fetcher)
	prob, err := parser.ParseCodeforcesProblem(context.Background(), "2232", "C2")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	fmt.Println("\n=== PARSED DESCRIPTION (full) ===")
	fmt.Println(prob.Description)

	// Check for list markers
	lines := strings.Split(prob.Description, "\n")
	fmt.Println("\n=== LINES WITH - OR * ===")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "-") || strings.HasPrefix(strings.TrimSpace(line), "*") {
			fmt.Printf("Line %d: %q\n", i, line)
		}
	}
}
