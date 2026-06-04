package vjudge

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestParser(t *testing.T) {
	// --- AtCoder Parser Tests ---
	t.Run("ParseAtCoderProblem/BasicProblem", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			if !strings.Contains(url, "atcoder.jp/contests/abc300/tasks/a") {
				return "", fmt.Errorf("unexpected URL: %s", url)
			}
			return `<html><body>
<h2><a href="/contests/abc300/tasks/abc300_a">A - Not Found</a></h2>
<div class="part">
<section>
<h3>Problem Statement</h3>
<p>Given two integers A and B, find the integer C (1<=C<=999) such that C is not equal to A and C is not equal to B.</p>
</section>
</div>
<div class="part">
<section>
<h3>Constraints</h3>
<ul><li>1 <= A, B <= 999</li></ul>
</section>
</div>
<div class="part">
<section>
<h3>Input</h3>
<p>Input is given from Standard Input in the following format:</p>
<pre>A B</pre>
</section>
</div>
<div class="part">
<section>
<h3>Output</h3>
<p>Print the answer.</p>
<pre>100</pre>
</section>
</div>
<span class="h2">Time Limit: 2 sec</span>
<span class="h2">Memory Limit: 1024 MB</span>
</body></html>`, nil
		}

		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseAtCoderProblem(context.Background(), "abc300", "a")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prob.Source != "atcoder" {
			t.Errorf("expected source 'atcoder', got %q", prob.Source)
		}
		if prob.RemoteID != "abc300_a" {
			t.Errorf("expected remote ID 'abc300_a', got %q", prob.RemoteID)
		}
		if !strings.Contains(prob.Title, "Not Found") {
			t.Errorf("expected title to contain 'Not Found', got %q", prob.Title)
		}
		if !strings.Contains(prob.Description, "Given two integers") {
			t.Errorf("expected description to contain problem text, got %q", prob.Description)
		}
		if prob.TimeLimit != 2000 {
			t.Errorf("expected time limit 2000ms, got %d", prob.TimeLimit)
		}
		if prob.MemoryLimit != 1048576 {
			t.Errorf("expected memory limit 1048576KB (1024MB), got %d", prob.MemoryLimit)
		}
	})

	t.Run("ParseAtCoderProblem/FetchError", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			return "", fmt.Errorf("network error")
		}
		parser := NewProblemParser(fetcher)
		_, err := parser.ParseAtCoderProblem(context.Background(), "abc300", "a")
		if err == nil {
			t.Fatal("expected error on fetch failure")
		}
	})

	t.Run("ParseAtCoderProblem/DefaultsWhenFieldsMissing", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			return `<html><body>
<div class="part">
<section>
<p>Some problem text</p>
</section>
</div>
</body></html>`, nil
		}
		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseAtCoderProblem(context.Background(), "abc300", "a")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prob.TimeLimit == 0 {
			t.Error("expected non-zero default time limit")
		}
		if prob.MemoryLimit == 0 {
			t.Error("expected non-zero default memory limit")
		}
	})

	// --- Toph Parser Tests ---
	t.Run("ParseTophProblem/BasicProblem", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			if !strings.Contains(url, "toph.co/p/1001") {
				return "", fmt.Errorf("unexpected URL: %s", url)
			}
			return `<html><body>
<h1 class="problem-title">Sum of Two Numbers</h1>
<div class="problem-body">
<p>Given two integers A and B, print A + B.</p>
</div>
<div class="problem-info">
<span class="time-limit">Time Limit: 1s</span>
<span class="memory-limit">Memory Limit: 256MB</span>
</div>
<div class="problem-section">
<h3>Input</h3>
<p>Two integers A and B.</p>
</div>
<div class="problem-section">
<h3>Output</h3>
<p>Print A + B.</p>
</div>
<pre class="sample-input">1 2</pre>
<pre class="sample-output">3</pre>
</body></html>`, nil
		}

		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseTophProblem(context.Background(), "1001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prob.Source != "toph" {
			t.Errorf("expected source 'toph', got %q", prob.Source)
		}
		if prob.RemoteID != "1001" {
			t.Errorf("expected remote ID '1001', got %q", prob.RemoteID)
		}
		if !strings.Contains(prob.Title, "Sum of Two Numbers") {
			t.Errorf("expected title to contain 'Sum of Two Numbers', got %q", prob.Title)
		}
		if !strings.Contains(prob.Description, "Given two integers") {
			t.Errorf("expected description to contain problem text, got %q", prob.Description)
		}
		if prob.TimeLimit != 1000 {
			t.Errorf("expected time limit 1000ms, got %d", prob.TimeLimit)
		}
		if prob.MemoryLimit != 262144 {
			t.Errorf("expected memory limit 262144KB (256MB), got %d", prob.MemoryLimit)
		}
	})

	t.Run("ParseTophProblem/FetchError", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			return "", fmt.Errorf("network error")
		}
		parser := NewProblemParser(fetcher)
		_, err := parser.ParseTophProblem(context.Background(), "1001")
		if err == nil {
			t.Fatal("expected error on fetch failure")
		}
	})

	t.Run("ParseTophProblem/DefaultsWhenFieldsMissing", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			return `<html><body>
<h1 class="problem-title">Minimal Problem</h1>
<div class="problem-body">
<p>No limits specified.</p>
</div>
</body></html>`, nil
		}
		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseTophProblem(context.Background(), "1001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prob.TimeLimit == 0 {
			t.Error("expected non-zero default time limit")
		}
		if prob.MemoryLimit == 0 {
			t.Error("expected non-zero default memory limit")
		}
	})

	// --- QOJ Parser Tests ---
	t.Run("ParseQOJProblem/PDFProblem", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			if !strings.Contains(url, "qoj.ac/problem/123") {
				return "", fmt.Errorf("unexpected URL: %s", url)
			}
			return `<html><body>
<h1>QOJ Problem 123</h1>
<div class="problem-content">
<a href="/problems/files/123/problem.pdf">Problem Statement (PDF)</a>
</div>
</body></html>`, nil
		}

		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseQOJProblem(context.Background(), "123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prob.Source != "qoj" {
			t.Errorf("expected source 'qoj', got %q", prob.Source)
		}
		if prob.RemoteID != "123" {
			t.Errorf("expected remote ID '123', got %q", prob.RemoteID)
		}
		if prob.Description != "https://qoj.ac/problems/files/123/problem.pdf" {
			t.Errorf("expected description to be PDF URL, got %q", prob.Description)
		}
	})

	t.Run("ParseQOJProblem/AbsolutePDFUrl", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			return `<html><body>
<div>
<a href="https://qoj.ac/problems/files/456/problem.pdf">PDF</a>
</div>
</body></html>`, nil
		}
		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseQOJProblem(context.Background(), "456")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prob.Description != "https://qoj.ac/problems/files/456/problem.pdf" {
			t.Errorf("expected absolute PDF URL preserved, got %q", prob.Description)
		}
	})

	t.Run("ParseQOJProblem/NoPDFLink", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			return `<html><body>
<h1>Problem Title</h1>
<div class="problem-content">
<p>Some problem description text without PDF.</p>
</div>
</body></html>`, nil
		}
		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseQOJProblem(context.Background(), "789")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prob.Description == "" {
			t.Error("expected non-empty description from page text")
		}
	})

	t.Run("ParseQOJProblem/FetchError", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			return "", fmt.Errorf("network error")
		}
		parser := NewProblemParser(fetcher)
		_, err := parser.ParseQOJProblem(context.Background(), "123")
		if err == nil {
			t.Fatal("expected error on fetch failure")
		}
	})
}
