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
		if prob.Difficulty != "medium" {
			t.Errorf("expected difficulty 'medium', got %q", prob.Difficulty)
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
		if prob.Difficulty != "medium" {
			t.Errorf("expected difficulty 'medium', got %q", prob.Difficulty)
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
		if prob.Difficulty != "medium" {
			t.Errorf("expected difficulty 'medium', got %q", prob.Difficulty)
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

	// --- Codeforces Parser Tests ---
	t.Run("ParseCodeforcesProblem/MultilineSampleCases", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			if !strings.Contains(url, "codeforces.com/contest/1234/problem/A") {
				return "", fmt.Errorf("unexpected URL: %s", url)
			}
			// Simulate Codeforces HTML with <br> tags in sample testcases
			return `<html><body>
<div class="problem-statement">
<div class="title">A. Multiline Problem</div>
<div class="time-limit">time limit per test2 seconds</div>
<div class="memory-limit">memory limit per test256 megabytes</div>
<div class="ttypography">
<p>Given an array of integers, find the maximum sum.</p>
<p><strong>Input:</strong> The first line contains an integer n (1 ≤ n ≤ 100).</p>
<p><em>Output:</em> Print the maximum sum.</p>
</div>
<div class="sample-tests">
<div class="input">
<div class="title">Input</div>
<pre>4
1 2 3 4
5 6 7 8
9 10 11 12
13 14 15 16</pre>
</div>
<div class="output">
<div class="title">Output</div>
<pre>136</pre>
</div>
</div>
</div>
</body></html>`, nil
		}

		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseCodeforcesProblem(context.Background(), "1234", "A")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prob.Title != "Multiline Problem" {
			t.Errorf("expected title 'Multiline Problem', got %q", prob.Title)
		}
		// Check that description preserves bold and italic formatting
		if !strings.Contains(prob.Description, "**Input:**") {
			t.Errorf("expected description to contain bold 'Input:', got %q", prob.Description)
		}
		if !strings.Contains(prob.Description, "*Output:*") {
			t.Errorf("expected description to contain italic 'Output:', got %q", prob.Description)
		}
		// Check that sample cases preserve newlines
		if len(prob.SampleCases) != 1 {
			t.Fatalf("expected 1 sample case, got %d", len(prob.SampleCases))
		}
		expectedInput := "4\n1 2 3 4\n5 6 7 8\n9 10 11 12\n13 14 15 16\n"
		if prob.SampleCases[0].Input != expectedInput {
			t.Errorf("expected sample input to preserve newlines:\ngot:      %q\nexpected: %q", prob.SampleCases[0].Input, expectedInput)
		}
	})

	t.Run("ParseCodeforcesProblem/SampleCasesWithBRTags", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			return `<html><body>
<div class="problem-statement">
<div class="title">B. BR Test</div>
<div class="time-limit">time limit per test1 second</div>
<div class="memory-limit">memory limit per test256 megabytes</div>
<div class="ttypography">
<p>Print each number on a separate line.</p>
</div>
<div class="sample-tests">
<div class="input">
<div class="title">Input</div>
<pre>3<br>10<br>20<br>30</pre>
</div>
<div class="output">
<div class="title">Output</div>
<pre>10<br>20<br>30</pre>
</div>
</div>
</div>
</body></html>`, nil
		}

		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseCodeforcesProblem(context.Background(), "5678", "B")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(prob.SampleCases) != 1 {
			t.Fatalf("expected 1 sample case, got %d", len(prob.SampleCases))
		}
		expectedInput := "3\n10\n20\n30\n"
		expectedOutput := "10\n20\n30\n"
		if prob.SampleCases[0].Input != expectedInput {
			t.Errorf("sample input with <br> tags: got %q, expected %q", prob.SampleCases[0].Input, expectedInput)
		}
		if prob.SampleCases[0].Output != expectedOutput {
			t.Errorf("sample output with <br> tags: got %q, expected %q", prob.SampleCases[0].Output, expectedOutput)
		}
	})

	t.Run("ParseCodeforcesProblem/SampleCasesWithDivLines", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			return `<html><body>
<div class="problem-statement">
<div class="title">C. Div Test</div>
<div class="time-limit">time limit per test1 second</div>
<div class="memory-limit">memory limit per test256 megabytes</div>
<div class="ttypography">
<p>Process multiple test cases.</p>
</div>
<div class="sample-tests">
<div class="input">
<div class="title">Input</div>
<pre>
<div class="test-example-line test-example-line-0">3</div>
<div class="test-example-line test-example-line-1">5 2 2</div>
<div class="test-example-line test-example-line-2">EIAIE</div>
</pre>
</div>
<div class="output">
<div class="title">Output</div>
<pre>
<div class="test-example-line test-example-line-1">4</div>
</pre>
</div>
</div>
</div>
</body></html>`, nil
		}

		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseCodeforcesProblem(context.Background(), "9999", "C")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(prob.SampleCases) != 1 {
			t.Fatalf("expected 1 sample case, got %d", len(prob.SampleCases))
		}
		expectedInput := "3\n5 2 2\nEIAIE\n"
		expectedOutput := "4\n"
		if prob.SampleCases[0].Input != expectedInput {
			t.Errorf("sample input with div lines: got %q, expected %q", prob.SampleCases[0].Input, expectedInput)
		}
		if prob.SampleCases[0].Output != expectedOutput {
			t.Errorf("sample output with div lines: got %q, expected %q", prob.SampleCases[0].Output, expectedOutput)
		}
	})

	t.Run("ParseCodeforcesProblem/MathNotation", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			return `<html><body>
<div class="problem-statement">
<div class="title">D. Math Test</div>
<div class="time-limit">time limit per test1 second</div>
<div class="memory-limit">memory limit per test256 megabytes</div>
<div class="ttypography">
<p>Given $$$n$$$ integers, find the sum of $$$a_1 + a_2 + \ldots + a_n$$$.</p>
<p>The value of $$$x$$$ is at most $$$10^9$$$.</p>
</div>
<div class="sample-tests">
<div class="input">
<div class="title">Input</div>
<pre>1 2 3</pre>
</div>
<div class="output">
<div class="title">Output</div>
<pre>6</pre>
</div>
</div>
</div>
</body></html>`, nil
		}

		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseCodeforcesProblem(context.Background(), "8888", "D")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(prob.Description, "$n$") {
			t.Errorf("expected $$$ to be converted to $, got: %s", prob.Description)
		}
		if strings.Contains(prob.Description, "$$$") {
			t.Errorf("description should not contain $$$: %s", prob.Description)
		}
	})

	t.Run("ParseCodeforcesProblem/TexFontStyleTT", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			return `<html><body>
<div class="problem-statement">
<div class="title">E. Monospace Test</div>
<div class="time-limit">time limit per test1 second</div>
<div class="memory-limit">memory limit per test256 megabytes</div>
<div class="ttypography">
<p>The string <span class="tex-font-style-tt">hello</span> contains 5 characters.</p>
<p>Use <span class="tex-font-style-tt">printf</span> for output.</p>
</div>
<div class="sample-tests">
<div class="input">
<div class="title">Input</div>
<pre>test</pre>
</div>
<div class="output">
<div class="title">Output</div>
<pre>test</pre>
</div>
</div>
</div>
</body></html>`, nil
		}

		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseCodeforcesProblem(context.Background(), "7777", "E")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(prob.Description, "`hello`") {
			t.Errorf("expected tex-font-style-tt to be converted to backticks, got: %s", prob.Description)
		}
		if !strings.Contains(prob.Description, "`printf`") {
			t.Errorf("expected tex-font-style-tt to be converted to backticks, got: %s", prob.Description)
		}
	})

	// --- CSES Parser Tests ---
	t.Run("ParseCSESProblem/StripsExampleSection", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			if !strings.Contains(url, "cses.fi/problemset/task/1068") {
				return "", fmt.Errorf("unexpected URL: %s", url)
			}
			return `<html><body>
<div class="navigation">
<div class="title-block">
<h3><a href="/problemset/list/">CSES Problem Set</a></h3>
<h1>Weird Algorithm</h1>
</div>
</div>
<div class="content-wrapper">
<div class="content">
<link rel="stylesheet" href="/lib/katex/katex.min.css">
<script defer src="/lib/katex/katex.min.js"></script>
<style>.katex .base:last-child { display: inline; }</style>
<ul class="task-constraints">
<li><b>Time limit:</b> 1.00 s</li>
<li><b>Memory limit:</b> 512 MB</li>
</ul>
<div class="md"><p>Consider an algorithm that takes as input a positive integer <span class="math math-inline">n</span>. If <span class="math math-inline">n</span> is even, the algorithm divides it by two, and if <span class="math math-inline">n</span> is odd, the algorithm multiplies it by three and adds one. The algorithm repeats this, until <span class="math math-inline">n</span> is one. For example, the sequence for <span class="math math-inline">n=3</span> is as follows:
<span class="math math-display"> 3 \rightarrow 10 \rightarrow 5 \rightarrow 16 \rightarrow 8 \rightarrow 4 \rightarrow 2 \rightarrow 1</span>
Your task is to simulate the execution of the algorithm for a given value of <span class="math math-inline">n</span>.</p>
<h1 id="input">Input</h1>
<p>The only input line contains an integer <span class="math math-inline">n</span>.</p>
<h1 id="output">Output</h1>
<p>Print a line that contains all values of <span class="math math-inline">n</span> during the algorithm.</p>
<h1 id="constraints">Constraints</h1>
<ul>
<li><span class="math math-inline">1 \le n \le 10^6</span></li>
</ul>
<h1 id="example">Example</h1>
<p>Input:</p>
<pre>3
</pre>
<p>Output:</p>
<pre>3 10 5 16 8 4 2 1
</pre></div>
</div>
</div>
</body></html>`, nil
		}

		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseCSESProblem(context.Background(), "1068")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prob.Title != "Weird Algorithm" {
			t.Errorf("expected title 'Weird Algorithm', got %q", prob.Title)
		}
		if strings.Contains(prob.Description, "Example") {
			t.Errorf("description should NOT contain Example section, got:\n%s", prob.Description)
		}
		if !strings.Contains(prob.Description, "positive integer") {
			t.Errorf("description should contain problem text, got:\n%s", prob.Description)
		}
		if prob.InputFormat == "" {
			t.Error("expected non-empty input format")
		}
		if prob.OutputFormat == "" {
			t.Error("expected non-empty output format")
		}
		if len(prob.SampleCases) != 1 {
			t.Errorf("expected 1 sample case, got %d", len(prob.SampleCases))
		} else {
			if !strings.Contains(prob.SampleCases[0].Input, "3") {
				t.Errorf("expected sample input to contain '3', got %q", prob.SampleCases[0].Input)
			}
		}
		if prob.TimeLimit != 1000 {
			t.Errorf("expected time limit 1000, got %d", prob.TimeLimit)
		}
		if prob.MemoryLimit != 524288 {
			t.Errorf("expected memory limit 524288, got %d", prob.MemoryLimit)
		}
	})

	t.Run("ParseCSESProblem/MathRendering", func(t *testing.T) {
		fetcher := func(ctx context.Context, url string) (string, error) {
			return `<html><body>
<div class="content">
<div class="md"><p>Given <span class="math math-inline">x^2 + y^2 = z^2</span>, find all solutions.</p>
<p>Compute:
<span class="math math-display">\sum_{i=1}^{n} i = \frac{n(n+1)}{2}</span>
</p>
<h1 id="input">Input</h1>
<p>An integer.</p>
<h1 id="output">Output</h1>
<p>The result.</p>
<h1 id="example">Example</h1>
<p>Input:</p>
<pre>5
</pre>
<p>Output:</p>
<pre>15
</pre></div>
</div>
</body></html>`, nil
		}
		parser := NewProblemParser(fetcher)
		prob, err := parser.ParseCSESProblem(context.Background(), "999")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(prob.Description, "$x^2 + y^2 = z^2$") {
			t.Errorf("expected inline math to be wrapped in $, got:\n%s", prob.Description)
		}
		if !strings.Contains(prob.Description, "$$\\sum") {
			t.Errorf("expected display math to be wrapped in $$, got:\n%s", prob.Description)
		}
	})
}
