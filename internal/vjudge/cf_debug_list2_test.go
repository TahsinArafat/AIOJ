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

func TestDebugCF2232C2ListHTML(t *testing.T) {
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

	html, _ := fetcher(context.Background(), "https://codeforces.com/contest/2232/problem/C2")

	// Find the ttypography section and the list inside it
	idx := strings.Index(html, `class="ttypography"`)
	if idx > 0 {
		ttypographyHTML := html[idx:]
		ulIdx := strings.Index(ttypographyHTML, "<ul>")
		if ulIdx > 0 {
			end := ulIdx + 800
			if end > len(ttypographyHTML) {
				end = len(ttypographyHTML)
			}
			fmt.Println("=== UL INSIDE TTYPOGRAPHY ===")
			fmt.Println(ttypographyHTML[ulIdx:end])
		}
	}

	// Also check for any other list-like structures
	if strings.Contains(html, "<li>") {
		liIdx := strings.Index(html, "<li>")
		fmt.Println("\n=== FIRST <li> in page ===")
		fmt.Println(html[liIdx-100 : liIdx+200])
	}
}
