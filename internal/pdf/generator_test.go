package pdf

import (
	"regexp"
	"strconv"
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

/**
 * The booklet printed with no page numbers at all. `counter(page)` and
 * `counter(pages)` resolve to zero anywhere in the document flow — they only
 * have values inside a page-margin box, which is why the counters must be
 * declared in a @bottom-center block nested in @page rather than in an element
 * or a body::after rule.
 *
 * Measured in Chrome 155 against the real booklet: three problems print as four
 * sheets (cover + 3), and the footers read "Page 2 of 4", "Page 3 of 4",
 * "Page 4 of 4" with the cover carrying no number. `@page :first` is what
 * suppresses the cover; `counter-reset: page` is ignored, so the cover still
 * counts toward the total and the numbers match the printed sheet count.
 */

// pageRule returns the text of the @page block from the booklet stylesheet.
func pageRule(t *testing.T, html string) string {
	t.Helper()
	i := strings.Index(html, "@page")
	if i < 0 {
		t.Fatal("the booklet has no @page rule, so nothing can print in the margin")
	}
	depth, j := 0, strings.Index(html[i:], "{")
	if j < 0 {
		t.Fatal("@page has no block")
	}
	j += i
	for k := j; k < len(html); k++ {
		switch html[k] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return html[j : k+1]
			}
		}
	}
	t.Fatal("@page block is unterminated")
	return ""
}

func TestBookletPrintsPageNumbers(t *testing.T) {
	html := bookletHTML(t)
	rule := pageRule(t, html)

	// The counters only mean anything inside a margin box. A counter(page) in
	// the body, or a footer built as an <div>, prints nothing.
	for _, want := range []string{"@bottom-center", "counter(page)", "counter(pages)"} {
		if !strings.Contains(rule, want) {
			t.Fatalf("the @page rule is missing %q, so pages print unnumbered", want)
		}
	}

	// Both counters must sit in the same margin-box content declaration: split
	// apart, one of the two silently prints as an empty string.
	content := rule[strings.Index(rule, "content:"):]
	if !strings.Contains(content[:strings.Index(content, ";")], "counter(page)") {
		t.Fatal("counter(page) is outside the margin box content declaration")
	}
}

// The cover is a title page. A number on it is the one thing a reader notices,
// and it was not in the request.
func TestBookletCoverCarriesNoPageNumber(t *testing.T) {
	html := bookletHTML(t)

	if !strings.Contains(html, "@page :first") {
		t.Fatal("no @page :first rule, so the cover is numbered like a problem page")
	}
	// The margin box must still be declared for the other pages, or suppressing
	// the cover also suppressed every footer.
	if !strings.Contains(html, "@bottom-center") {
		t.Fatal("suppressing the cover removed the footer for every page")
	}
}

// The page-number band lives in the page margin, so the bottom margin has to be
// deep enough to hold it without colliding with the last line of a statement.
// Measured in the printed booklet: the 9pt footer baseline lands ~11mm from the
// sheet edge while the text block ends at the bottom margin, so the margin must
// comfortably exceed one 9pt line (~3.2mm) plus padding.
func TestBookletReservesSpaceForTheFooter(t *testing.T) {
	rule := pageRule(t, bookletHTML(t))

	m := regexp.MustCompile(`margin:\s*([\d.]+)mm\s+([\d.]+)mm\s+([\d.]+)mm\s+([\d.]+)mm`).FindStringSubmatch(rule)
	if m == nil {
		t.Fatal("@page margin is not the 4-value form, so the bottom band is unverified")
	}
	bottom, _ := strconv.ParseFloat(m[3], 64)
	if bottom < 15 {
		t.Fatalf("bottom margin is only %gmm; a 9pt footer plus padding needs at least 15mm", bottom)
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
