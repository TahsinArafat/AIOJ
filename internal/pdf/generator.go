package pdf

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"github.com/tahsinarafat/aioj/internal/model"
)

var imgRegex = regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

type problemData struct {
	Index        string
	Title        string
	TimeLimit    int
	MemoryLimit  int
	Description  template.HTML
	InputFormat  template.HTML
	OutputFormat template.HTML
	Samples      []sampleData
	Hint         string
	HasHint      bool
}

type sampleData struct {
	Number      int
	Input       string
	Output      string
	Explanation string
	HasExplain  bool
}

type templateData struct {
	Title        string
	Duration     string
	ProblemCount int
	Problems     []problemData
	Year         string
}

// stripIOSections removes Input/Output/Hint sections from the description
// since they're rendered separately from dedicated fields.
func stripIOSections(md string) string {
	lines := strings.Split(md, "\n")
	var out []string
	skip := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for section headers that we render separately
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			header := strings.ToLower(strings.TrimLeft(trimmed, "# "))
			header = strings.TrimSpace(header)
			if header == "input" || header == "output" || header == "hint" || header == "hints" {
				skip = true
				continue
			}
			// Another section header stops skipping
			skip = false
		}

		if !skip {
			out = append(out, line)
		}
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}

func mdToHTML(md string, stripIO bool) string {
	// Strip Input/Output/Hint sections if they're rendered separately
	if stripIO {
		md = stripIOSections(md)
	}
	lines := strings.Split(md, "\n")
	var out strings.Builder
	inCode := false
	inList := false
	// para accumulates consecutive prose lines into ONE <p>. Emitting a <p> per
	// source line split a wrapped sentence into fragments, which is why statements
	// looked shredded across the page: Markdown soft-wraps prose, it does not
	// start a new paragraph on every newline.
	var para []string

	// flushPara writes the pending prose as a single paragraph.
	flushPara := func() {
		if len(para) == 0 {
			return
		}
		joined := strings.Join(para, " ")
		para = para[:0]
		out.WriteString(fmt.Sprintf("<p>%s</p>\n", processInline(joined)))
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Code blocks
		if strings.HasPrefix(trimmed, "```") {
			flushPara()
			if inCode {
				out.WriteString("</code></pre>\n")
				inCode = false
			} else {
				out.WriteString("<pre class=\"code-block\"><code>")
				inCode = true
			}
			continue
		}
		if inCode {
			out.WriteString(template.HTMLEscapeString(line) + "\n")
			continue
		}

		// Headings
		if strings.HasPrefix(trimmed, "### ") {
			flushPara()
			if inList {
				out.WriteString("</ul>\n")
				inList = false
			}
			out.WriteString(fmt.Sprintf("<h4>%s</h4>\n", strings.TrimPrefix(trimmed, "### ")))
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			flushPara()
			if inList {
				out.WriteString("</ul>\n")
				inList = false
			}
			out.WriteString(fmt.Sprintf("<h3>%s</h3>\n", strings.TrimPrefix(trimmed, "## ")))
			continue
		}

		// Bullet lists
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			flushPara()
			if !inList {
				out.WriteString("<ul>\n")
				inList = true
			}
			item := strings.TrimPrefix(trimmed, "- ")
			item = strings.TrimPrefix(item, "* ")
			out.WriteString(fmt.Sprintf("<li>%s</li>\n", processInline(item)))
			continue
		}
		if inList && trimmed == "" {
			out.WriteString("</ul>\n")
			inList = false
		}

		// Paragraphs
		if trimmed == "" {
			flushPara()
			if inList {
				out.WriteString("</ul>\n")
				inList = false
			}
			continue
		}

		para = append(para, trimmed)
	}

	flushPara()
	if inList {
		out.WriteString("</ul>\n")
	}
	if inCode {
		out.WriteString("</code></pre>\n")
	}

	return out.String()
}

func processInline(s string) string {
	// Inline code
	result := ""
	for {
		idx := strings.Index(s, "`")
		if idx == -1 {
			result += escapeAndProcessMath(s)
			break
		}
		result += escapeAndProcessMath(s[:idx])
		s = s[idx+1:]
		end := strings.Index(s, "`")
		if end == -1 {
			result += "<code>" + template.HTMLEscapeString(s) + "</code>"
			break
		}
		result += "<code>" + template.HTMLEscapeString(s[:end]) + "</code>"
		s = s[end+1:]
	}

	// Bold **text**
	res := ""
	for {
		idx := strings.Index(result, "**")
		if idx == -1 {
			res += result
			break
		}
		res += result[:idx]
		result = result[idx+2:]
		end := strings.Index(result, "**")
		if end == -1 {
			res += "**" + result
			break
		}
		res += "<strong>" + result[:end] + "</strong>"
		result = result[end+2:]
	}

	res = imgRegex.ReplaceAllStringFunc(res, func(match string) string {
		submatches := imgRegex.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		alt := submatches[1]
		url := submatches[2]
		url = strings.ReplaceAll(url, "&#x2F;", "/")
		url = strings.ReplaceAll(url, "&#x3A;", ":")
		return fmt.Sprintf(`<img src="%s" alt="%s" style="max-width: 100%%; height: auto; display: block; margin: 10px 0;" />`, url, alt)
	})

	return res
}

func escapeAndProcessMath(s string) string {
	// Process display math $$...$$ first
	result := ""
	for {
		idx := strings.Index(s, "$$")
		if idx == -1 {
			result += processInlineMath(s)
			break
		}
		result += processInlineMath(s[:idx])
		s = s[idx+2:]
		end := strings.Index(s, "$$")
		if end == -1 {
			result += "$$" + s
			break
		}
		result += `<span class="katex-display">$$` + s[:end] + `$$</span>`
		s = s[end+2:]
	}
	return result
}

func processInlineMath(s string) string {
	// Process inline math $...$
	result := ""
	for {
		idx := strings.Index(s, "$")
		if idx == -1 {
			result += template.HTMLEscapeString(s)
			break
		}
		result += template.HTMLEscapeString(s[:idx])
		s = s[idx+1:]
		end := strings.Index(s, "$")
		if end == -1 {
			result += "$" + template.HTMLEscapeString(s)
			break
		}
		// Inline math keeps single-$ delimiters. It previously emitted $$...$$,
		// which KaTeX renders as display math — so every mid-sentence $t$ became
		// a centred block on its own line, breaking the paragraph apart.
		result += `<span class="katex-inline">$` + s[:end] + `$</span>`
		s = s[end+1:]
	}
	return result
}

func (g *Generator) GenerateContestPDF(contest *model.Contest, problems []model.ProblemWithSamples) ([]byte, error) {
	var probList []problemData
	for i, p := range problems {
		var samples []sampleData
		for j, sc := range p.SampleCases {
			samples = append(samples, sampleData{
				Number:      j + 1,
				Input:       sc.Input,
				Output:      sc.Output,
				Explanation: sc.Explanation,
				HasExplain:  sc.Explanation != "",
			})
		}

		descHTML := mdToHTML(p.Description, true)
		inputHTML := ""
		if p.InputFormat != "" {
			inputHTML = mdToHTML(p.InputFormat, false)
		}
		outputHTML := ""
		if p.OutputFormat != "" {
			outputHTML = mdToHTML(p.OutputFormat, false)
		}

		memLimit := p.MemoryLimit / 1024

		probList = append(probList, problemData{
			Index:        string(rune('A' + i)),
			Title:        p.Title,
			TimeLimit:    p.TimeLimit,
			MemoryLimit:  memLimit,
			Description:  template.HTML(descHTML),
			InputFormat:  template.HTML(inputHTML),
			OutputFormat: template.HTML(outputHTML),
			Samples:      samples,
			Hint:         p.Hint,
			HasHint:      p.Hint != "",
		})
	}

	dur := contest.EndTime.Sub(contest.StartTime)
	hours := int(dur.Hours())
	mins := int(dur.Minutes()) % 60
	durationStr := fmt.Sprintf("%dh %02dm", hours, mins)

	data := templateData{
		Title:        contest.Title,
		Duration:     durationStr,
		ProblemCount: len(problems),
		Problems:     probList,
	}

	tmpl, err := template.New("pdf").Parse(pdfTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return buf.Bytes(), nil
}

var pdfTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>{{.Title}}</title>
<!-- KaTeX is self-hosted (web/scripts/copy-katex.mjs copies it to dist/katex)
     and every script is an external file. Both matter: the CSP is
     script-src 'self' blob:, which permits no CDN origin AND no inline script
     or inline event handler. Loading KaTeX from jsdelivr meant it never ran;
     putting the render call in an inline <script> meant it still never ran. -->
<link rel="stylesheet" href="/katex/katex.min.css">
<script defer src="/katex/katex.min.js"></script>
<script defer src="/katex/auto-render.min.js"></script>
<script defer src="/booklet/booklet.js"></script>

<style>
  @page {
    size: A4;
    margin: 22mm 20mm 24mm 20mm;
    /* Page numbers can only be printed from a page-margin box: counter(page) and
       counter(pages) resolve nowhere in the document flow, only inside @page. */
    @bottom-center {
      content: "Page " counter(page) " of " counter(pages);
      font-family: "Latin Modern Roman", "Computer Modern Roman", "Times New Roman", Georgia, serif;
      font-size: 9pt;
      color: #555;
    }
  }
  /* The cover is a title page — a number on it reads as a mistake. The count
     still includes it, so the numbers match the printed sheet count. */
  @page :first {
    @bottom-center { content: none; }
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  /* Text must be selectable: the booklet is read on screen and quoted from, and
     a page that cannot be copied is a documented complaint. */
  html, body, .problem-page, .cover, .sample-box, code, pre, .katex {
    -webkit-user-select: text;
    user-select: text;
  }
  /* Screen-only toolbar: never printed, never part of the booklet itself. */
  .toolbar {
    position: sticky;
    top: 0;
    z-index: 10;
    display: flex;
    align-items: center;
    gap: 4mm;
    padding: 3mm 20mm;
    background: #f4f4f4;
    border-bottom: 1px solid #ddd;
    font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
  }
  .toolbar button {
    font: inherit;
    font-weight: 600;
    padding: 2mm 5mm;
    border: 1px solid #333;
    border-radius: 2mm;
    background: #1a1a1a;
    color: #fff;
    cursor: pointer;
  }
  .toolbar-hint { font-size: 9pt; color: #666; }
  body {
    font-family: "Latin Modern Roman", "Computer Modern Roman", "Times New Roman", Georgia, serif;
    font-size: 11pt;
    line-height: 1.5;
    color: #1a1a1a;
    background: #fff;
  }

  /* ── Cover Page ── */
  .cover {
    page-break-after: always;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    min-height: 85vh;
    text-align: center;
  }
  .cover-title {
    font-size: 28pt;
    font-weight: 700;
    letter-spacing: 0.5pt;
    margin-bottom: 10mm;
    border-bottom: 2.5pt solid #1a1a1a;
    padding-bottom: 6mm;
  }
  .cover-sub {
    font-size: 13pt;
    color: #444;
    margin-bottom: 3mm;
  }
  .cover-table {
    margin-top: 12mm;
    border-collapse: collapse;
    width: 70%;
    font-size: 10.5pt;
  }
  .cover-table th, .cover-table td {
    border: 1pt solid #999;
    padding: 2.5mm 5mm;
    text-align: center;
  }
  .cover-table th {
    background: #f0f0f0;
    font-weight: 600;
  }
  .cover-footer {
    margin-top: 15mm;
    font-size: 9pt;
    color: #888;
  }

  /* ── Problem Pages ── */
  .problem-page { page-break-before: always; }
  .problem-header {
    border-bottom: 2pt solid #333;
    padding-bottom: 3mm;
    margin-bottom: 5mm;
  }
  .problem-label {
    font-size: 20pt;
    font-weight: 700;
    margin-bottom: 1mm;
  }
  .problem-title {
    font-size: 14pt;
    font-weight: 400;
    color: #333;
  }
  .problem-limits {
    font-size: 9pt;
    color: #666;
    margin-top: 2mm;
    letter-spacing: 0.3pt;
  }
  .problem-limits strong { color: #333; }

  h3 {
    font-size: 12pt;
    font-weight: 700;
    margin: 5mm 0 2mm 0;
    text-transform: uppercase;
    letter-spacing: 0.5pt;
    color: #222;
  }
  h4 {
    font-size: 11pt;
    font-weight: 600;
    margin: 4mm 0 1.5mm 0;
    color: #333;
  }

  p { margin: 1.5mm 0; }
  br + br { display: block; content: ""; margin-top: 2mm; }

  ul { margin: 2mm 0 2mm 6mm; }
  li { margin: 1mm 0; }

  /* ── Sample I/O Boxes ── */
  .samples { margin-top: 4mm; }
  .sample-pair {
    margin-bottom: 5mm;
    page-break-inside: avoid;
  }
  .sample-label {
    font-size: 10pt;
    font-weight: 700;
    margin-bottom: 1.5mm;
    color: #444;
    text-transform: uppercase;
    letter-spacing: 0.3pt;
  }
  .sample-box {
    border: 1pt solid #bbb;
    background: #fafafa;
    padding: 3mm 4mm;
    font-family: "Courier New", "Consolas", "Liberation Mono", monospace;
    font-size: 9.5pt;
    line-height: 1.45;
    white-space: pre-wrap;
    word-break: break-all;
    margin-bottom: 2mm;
    border-radius: 1pt;
  }
  .sample-box-label {
    font-size: 8pt;
    font-weight: 600;
    color: #777;
    text-transform: uppercase;
    letter-spacing: 0.5pt;
    margin-bottom: 1mm;
  }

  .explanation {
    font-style: italic;
    font-size: 10pt;
    color: #555;
    margin: 2mm 0 2mm 4mm;
  }

  /* ── Code / Math ── */
  code {
    font-family: "Courier New", "Consolas", monospace;
    background: #f4f4f4;
    padding: 0.3mm 1.5mm;
    font-size: 9.5pt;
    border-radius: 1pt;
  }
  .code-block {
    border: 1pt solid #ddd;
    background: #f8f8f8;
    padding: 3mm 4mm;
    font-size: 9pt;
    line-height: 1.4;
    overflow-x: auto;
    margin: 2mm 0;
    page-break-inside: avoid;
  }
  .code-block code {
    background: none;
    padding: 0;
  }

  /* KaTeX owns the layout of its own output. These wrappers only carry the
     inline/display distinction, so inline math must NOT inherit any margin or
     centring — doing that is what turned a mid-sentence $t$ into a block. */
  .katex { font-size: 1.05em; }
  .katex-inline { display: inline; }
  .katex-display { display: block; margin: 2mm 0; text-align: center; }

  strong { font-weight: 600; }

  /* ── Hint ── */
  .hint-box {
    margin-top: 4mm;
    padding: 3mm 4mm;
    border-left: 3pt solid #aaa;
    background: #f9f9f9;
    font-size: 10pt;
    color: #555;
    page-break-inside: avoid;
  }

  @media print {
    .no-print, .toolbar { display: none !important; }
    .problem-page { page-break-before: always; }
    body { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
  }
</style>
</head>
<body>

<!-- Screen-only toolbar. The booklet used to call window.print() from an onload
     handler, which opened the dialog unprompted and (when the CSP blocked the
     script) not at all. The reader decides when to print now. -->
<div class="toolbar no-print">
  <button type="button" id="aioj-print">Print / Save as PDF</button>
  <span class="toolbar-hint">Math may take a moment to typeset &mdash; print after it settles.</span>
</div>

<!-- COVER PAGE -->
<div class="cover">
  <div class="cover-title">{{.Title}}</div>
  <div class="cover-sub">Problem Set</div>
  <div class="cover-sub">Duration: {{.Duration}} &nbsp;|&nbsp; {{.ProblemCount}} Problems</div>
  <table class="cover-table">
    <tr><th>#</th><th>Problem Name</th><th>Time Limit</th><th>Memory Limit</th></tr>
    {{range .Problems}}<tr><td>{{.Index}}</td><td>{{.Title}}</td><td>{{.TimeLimit}} ms</td><td>{{.MemoryLimit}} MB</td></tr>{{end}}
  </table>
  <div class="cover-footer">AIOJ Online Judge</div>
</div>

<!-- PROBLEM PAGES -->
{{range .Problems}}
<div class="problem-page">
  <div class="problem-header">
    <div class="problem-label">Problem {{.Index}}</div>
    <div class="problem-title">{{.Title}}</div>
    <div class="problem-limits">
      <strong>Time Limit:</strong> {{.TimeLimit}} ms &nbsp;&nbsp;|&nbsp;&nbsp;
      <strong>Memory Limit:</strong> {{.MemoryLimit}} MB
    </div>
  </div>

  <h3>Problem Statement</h3>
  {{.Description}}

  {{if .InputFormat}}
  <h3>Input</h3>
  {{.InputFormat}}
  {{end}}

  {{if .OutputFormat}}
  <h3>Output</h3>
  {{.OutputFormat}}
  {{end}}

  <div class="samples">
  {{range .Samples}}
    <div class="sample-pair">
      <div class="sample-label">Sample {{.Number}}</div>
      <div class="sample-box-label">Input</div>
      <div class="sample-box">{{.Input}}</div>
      <div class="sample-box-label">Output</div>
      <div class="sample-box">{{.Output}}</div>
      {{if .HasExplain}}<div class="explanation">{{.Explanation}}</div>{{end}}
    </div>
  {{end}}
  </div>

  {{if .HasHint}}
  <div class="hint-box"><strong>Hint:</strong> {{.Hint}}</div>
  {{end}}
</div>
{{end}}

</body>
</html>`
