package pdf

import (
	"strings"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

func mustTime() time.Time { return time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC) }

/**
 * The booklet's math never typeset: KaTeX was loaded from jsdelivr while the site
 * CSP is `script-src 'self' blob:`, so the CDN script was blocked and
 * renderMathInElement never ran. `$...$` stayed literal, and because the print
 * dialog was triggered from that same blocked script, printing never opened
 * either. These pin the three properties that fix it.
 */

func bookletHTML(t *testing.T) string {
	t.Helper()
	c := &model.Contest{ID: "c1", Title: "Round", StartTime: mustTime(), EndTime: mustTime()}
	problems := []model.ProblemWithSamples{{
		Index: "A",
		Problem: model.Problem{
			Title:        "Sums",
			Description:  "Given $n \\le 10^5$, compute $\\sum_{i=1}^n a_i$.\n\n$$\\sum_{i=1}^{n} a_i = S$$",
			InputFormat:  "A single integer $n$.",
			OutputFormat: "The sum.",
		},
	}}
	out, err := NewGenerator().GenerateContestPDF(c, problems)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return string(out)
}

func TestBookletSelfHostsKaTeX(t *testing.T) {
	html := bookletHTML(t)

	// A CDN URL is the actual bug: the CSP forbids it, so the script never ran.
	if strings.Contains(html, "cdn.jsdelivr.net") {
		t.Fatal("the booklet still loads KaTeX from a CDN, which the CSP blocks")
	}
	for _, want := range []string{"/katex/katex.min.css", "/katex/katex.min.js", "/katex/auto-render.min.js"} {
		if !strings.Contains(html, want) {
			t.Fatalf("booklet is missing the self-hosted asset %q", want)
		}
	}
	// Same-origin paths are what CSP 'self' permits.
	if !strings.Contains(html, `href="/katex/katex.min.css"`) {
		t.Fatal("the stylesheet is not an absolute same-origin path")
	}
}

func TestBookletDoesNotAutoPrint(t *testing.T) {
	html := bookletHTML(t)

	// An unprompted print dialog is hostile, and it fired before KaTeX finished.
	if strings.Contains(html, "onload=\"renderMathInElement") {
		t.Fatal("the booklet still chains renderMathInElement and window.print in onload")
	}
	if strings.Contains(html, "window.print();\"") {
		t.Fatal("window.print is still invoked from the script load handler")
	}
	// Printing must remain available, just under the reader's control.
	if !strings.Contains(html, `id="aioj-print"`) {
		t.Fatal("no print control remains at all")
	}
	if !strings.Contains(html, "Print / Save as PDF") {
		t.Fatal("the print button is missing from the toolbar")
	}
}

func TestBookletTextIsSelectable(t *testing.T) {
	html := bookletHTML(t)

	// The reported symptom: text could not be copied.
	if !strings.Contains(html, "user-select: text") {
		t.Fatal("the booklet does not explicitly allow text selection")
	}
	// The toolbar must not print, or it lands on every page.
	if !strings.Contains(html, ".no-print, .toolbar { display: none !important; }") {
		t.Fatal("the screen-only toolbar is not excluded from print output")
	}
}

func TestBookletRendersMathMarkup(t *testing.T) {
	html := bookletHTML(t)

	// The statement's $...$ must survive into the page for auto-render to find.
	if !strings.Contains(html, "$$") && !strings.Contains(html, "$n") {
		t.Fatal("math delimiters were stripped, so there is nothing for KaTeX to typeset")
	}
	// The delimiter configuration lives in the external script now (inline JS is
	// blocked by CSP), so this only asserts the math reached the page at all.
	// booklet.js is checked separately by TestBookletHasNoInlineJavaScript.

}

// The CSP is `script-src 'self' blob:`, which forbids inline <script> blocks and
// inline event handlers as well as external origins. A first attempt at this fix
// self-hosted KaTeX but kept the render call in an inline script, so the browser
// blocked it and math still did not render. This test is the guard against
// repeating that: no inline JavaScript of any kind may appear in the booklet.
func TestBookletHasNoInlineJavaScript(t *testing.T) {
	html := bookletHTML(t)

	forbidden := []struct {
		name string
		frag string
	}{
		{"inline event handler onclick=", "onclick="},
		{"inline event handler onload=", "onload="},
		{"inline event handler onerror=", "onerror="},
		{"javascript: URL", "javascript:"},
	}
	for _, f := range forbidden {
		if strings.Contains(html, f.frag) {
			t.Fatalf("booklet contains %s, which CSP 'self' blocks (no unsafe-inline)", f.name)
		}
	}

	// A bare <script> with no src is an inline block, which is also blocked.
	for _, chunk := range strings.Split(html, "<script")[1:] {
		head := chunk
		if i := strings.Index(head, ">"); i >= 0 {
			head = head[:i]
		}
		if !strings.Contains(head, "src=") {
			t.Fatalf("booklet contains an inline <script> block (no src): script-src 'self' blocks it")
		}
	}

	// The behaviour must live in a same-origin file instead.
	if !strings.Contains(html, `src="/booklet/booklet.js"`) {
		t.Fatal("the booklet behaviour script is not referenced from /booklet/booklet.js")
	}
}

// Inline math must stay inline. The generator emitted $...$ wrapped in $$...$$,
// so KaTeX rendered a mid-sentence $t$ as display math: its own centred line,
// which shredded the paragraph and produced the reported layout. Display math
// still uses $$...$$, so the two must be distinguishable in the output.
func TestInlineMathKeepsInlineDelimiters(t *testing.T) {
	html := bookletHTML(t)

	if strings.Contains(html, `class="katex-inline">$$`) {
		t.Fatal("inline math is emitted with display delimiters, so KaTeX breaks the line")
	}
	if !strings.Contains(html, `class="katex-inline">$`) {
		t.Fatal("inline math is not emitted as inline span with single-dollar delimiters")
	}
	// Display math keeps its own delimiters and block wrapper.
	if !strings.Contains(html, `class="katex-display">$$`) {
		t.Fatal("display math lost its display delimiters")
	}
	// The wrappers must carry the layout distinction, not APIC-style margins.
	if !strings.Contains(html, ".katex-inline { display: inline; }") {
		t.Fatal("inline math has no inline CSS rule, so it may inherit block layout")
	}
}

// Prose soft-wraps in Markdown: a newline inside a sentence is not a paragraph
// break. Emitting one <p> per source line fragmented every statement, which is
// why the booklet looked shredded rather than like the problem page.
func TestSoftWrappedProseStaysOneParagraph(t *testing.T) {
	c := &model.Contest{ID: "c1", Title: "R", StartTime: mustTime(), EndTime: mustTime()}
	desc := "The first line contains $t$, the number of queries.\n" +
		"Each of the next $t$ lines contains two integers $n$ and $k$."
	out, err := NewGenerator().GenerateContestPDF(c, []model.ProblemWithSamples{{
		Index: "A", Problem: model.Problem{Title: "D", Description: desc},
	}})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	html := string(out)

	// One paragraph, not two: the sentence spans a source newline.
	if n := strings.Count(html, "<p>"); n != 1 {
		t.Fatalf("statement produced %d <p> elements, want 1 (soft wrap must not split a paragraph)", n)
	}
	if !strings.Contains(html, "queries. Each of the next") {
		t.Fatal("the wrapped sentence was not joined; prose fragments across lines")
	}
}

// A blank line still separates paragraphs.
func TestBlankLineStartsANewParagraph(t *testing.T) {
	c := &model.Contest{ID: "c1", Title: "R", StartTime: mustTime(), EndTime: mustTime()}
	desc := "First paragraph.\n\nSecond paragraph."
	out, err := NewGenerator().GenerateContestPDF(c, []model.ProblemWithSamples{{
		Index: "A", Problem: model.Problem{Title: "D", Description: desc},
	}})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if n := strings.Count(string(out), "<p>"); n != 2 {
		t.Fatalf("blank line produced %d paragraphs, want 2", n)
	}
}
