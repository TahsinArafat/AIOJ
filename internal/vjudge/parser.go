package vjudge

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/tahsinarafat/aioj/internal/model"
	"golang.org/x/net/html"
)

type ProblemParser struct {
	fetcher func(ctx context.Context, url string) (string, error)
}

func NewProblemParser(fetcher func(ctx context.Context, url string) (string, error)) *ProblemParser {
	return &ProblemParser{fetcher: fetcher}
}

func (p *ProblemParser) ParseCodeforcesProblem(ctx context.Context, contestID, problemIndex string) (*model.Problem, error) {
	problemURL := fmt.Sprintf("https://codeforces.com/contest/%s/problem/%s", contestID, problemIndex)
	body, err := p.fetcher(ctx, problemURL)
	if err != nil {
		return nil, fmt.Errorf("fetch problem page: %w", err)
	}

	prob := &model.Problem{
		Source:   "codeforces",
		RemoteID: contestID + "/" + problemIndex,
	}

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	// Find the problem-statement div (the main container)
	problemDiv := findNode(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "problem-statement")
	})

	if problemDiv == nil {
		// Fallback: try ttypography
		tstat := findNode(doc, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "ttypography")
		})
		if tstat == nil {
			return nil, fmt.Errorf("problem statement not found on page")
		}
		problemDiv = tstat
	}

	// Extract title from the .title div inside problem-statement
	titleNode := findNode(problemDiv, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "title")
	})
	if titleNode != nil {
		prob.Title = extractText(titleNode)
		prob.Title = strings.TrimSpace(strings.TrimPrefix(prob.Title, problemIndex+"."))
		prob.Title = strings.TrimSpace(prob.Title)
	}

	// Extract time limit from .time-limit div
	// CF text format: "time limit per test1 second" or "time limit per test2 seconds"
	tl := findNode(problemDiv, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "time-limit")
	})
	if tl != nil {
		prob.TimeLimit = parseTimeLimit(extractText(tl))
	}

	// Extract memory limit from .memory-limit div
	// CF text format: "memory limit per test256 megabytes"
	ml := findNode(problemDiv, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "memory-limit")
	})
	if ml != nil {
		prob.MemoryLimit = parseMemoryLimit(extractText(ml))
	}

	descriptionDiv := findNode(problemDiv, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "ttypography")
	})
	if descriptionDiv != nil {
		prob.Description = renderNodeToMarkdown(descriptionDiv)
	} else {
		prob.Description = renderNodeToMarkdown(problemDiv)
	}

	sampleDivs := findAllNodes(problemDiv, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "sample-tests")
	})
	if len(sampleDivs) > 0 {
		prob.SampleCases = extractSampleCases(sampleDivs[0])
	}

	prob.InputFormat = extractSectionFromDescription(prob.Description, "Input")
	prob.OutputFormat = extractSectionFromDescription(prob.Description, "Output")
	prob.Description = removeExamplesFromDescription(prob.Description)
	prob.Description = removeSectionFromDescription(prob.Description, "Input")
	prob.Description = removeSectionFromDescription(prob.Description, "Output")

	// Extract tags (including difficulty rating like *800)
	// Tags are in the SIDEBAR, not inside problem-statement, so search the whole doc
	tags := []string{}
	tagBoxes := findAllNodes(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "span" && hasClass(n, "tag-box")
	})
	for _, tb := range tagBoxes {
		tagText := strings.TrimSpace(extractText(tb))
		if strings.HasPrefix(tagText, "*") {
			// This is the difficulty rating tag like *800
			rating := strings.TrimPrefix(tagText, "*")
			rating = strings.TrimSpace(rating)
			prob.Tags = append(prob.Tags, "rating:"+rating)
		} else if tagText != "" && !strings.HasPrefix(tagText, "Difficulty") {
			tags = append(tags, strings.ToLower(tagText))
		}
	}
	if len(tags) > 0 {
		prob.Tags = append(prob.Tags, tags...)
	}

	// Set difficulty from rating tag
	if prob.Difficulty == "" {
		for _, tag := range prob.Tags {
			if strings.HasPrefix(tag, "rating:") {
				ratingStr := strings.TrimPrefix(tag, "rating:")
				var rating int
				fmt.Sscanf(ratingStr, "%d", &rating)
				switch {
				case rating < 1200:
					prob.Difficulty = "easy"
				case rating < 2000:
					prob.Difficulty = "medium"
				default:
					prob.Difficulty = "hard"
				}
				break
			}
		}
	}
	if prob.Difficulty == "" {
		prob.Difficulty = "medium"
	}

	if prob.Title == "" {
		prob.Title = fmt.Sprintf("CF %s %s", contestID, problemIndex)
	}
	if prob.TimeLimit == 0 {
		prob.TimeLimit = 2000
	}
	if prob.MemoryLimit == 0 {
		prob.MemoryLimit = 262144
	}

	return prob, nil
}

func (p *ProblemParser) ParseCSESProblem(ctx context.Context, problemID string) (*model.Problem, error) {
	problemURL := fmt.Sprintf("https://cses.fi/problemset/task/%s", problemID)
	body, err := p.fetcher(ctx, problemURL)
	if err != nil {
		return nil, fmt.Errorf("fetch CSES problem page: %w", err)
	}

	prob := &model.Problem{
		Source:     "cses",
		RemoteID:   problemID,
		Difficulty: "medium",
	}

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	titleNode := findNode(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "h1"
	})
	if titleNode != nil {
		prob.Title = strings.TrimSpace(extractText(titleNode))
	}
	if prob.Title == "" {
		prob.Title = fmt.Sprintf("CSES %s", problemID)
	}

	prob.TimeLimit = 1000
	prob.MemoryLimit = 524288

	contentDiv := findNode(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "content")
	})
	if contentDiv != nil {
		stripScriptAndStyle(contentDiv)
		prob.SampleCases = extractCSESSampleCases(doc)
		stripSampleCases(contentDiv)
		mdDiv := findNode(contentDiv, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "md")
		})
		if mdDiv != nil {
			prob.Description = renderNodeToMarkdown(mdDiv)
		} else {
			prob.Description = renderNodeToMarkdown(contentDiv)
		}
	}

	prob.Hint = extractConstraintsFromDescription(prob.Description)
	prob.Description = removeConstraintsFromDescription(prob.Description)

	prob.InputFormat = extractSectionFromDescription(prob.Description, "Input")
	prob.Description = removeSectionFromDescription(prob.Description, "Input")

	prob.OutputFormat = extractSectionFromDescription(prob.Description, "Output")
	prob.Description = removeSectionFromDescription(prob.Description, "Output")

	return prob, nil
}

func stripScriptAndStyle(n *html.Node) {
	var removeNodes []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "script" || c.Data == "style") {
			removeNodes = append(removeNodes, c)
		} else {
			stripScriptAndStyle(c)
		}
	}
	for _, node := range removeNodes {
		n.RemoveChild(node)
	}
}

func stripSampleCases(n *html.Node) {
	var removeNodes []*html.Node
	inExample := false
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "h1" && strings.TrimSpace(extractText(c)) == "Example" {
			inExample = true
			removeNodes = append(removeNodes, c)
		} else if inExample {
			removeNodes = append(removeNodes, c)
		} else if c.Type == html.ElementNode {
			stripSampleCases(c)
		}
	}
	for _, node := range removeNodes {
		n.RemoveChild(node)
	}
}

func stripConstraints(n *html.Node) {
	var removeNodes []*html.Node
	inConstraints := false
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "h1" && strings.TrimSpace(extractText(c)) == "Constraints" {
			inConstraints = true
			removeNodes = append(removeNodes, c)
		} else if inConstraints {
			removeNodes = append(removeNodes, c)
		} else if c.Type == html.ElementNode {
			stripConstraints(c)
		}
	}
	for _, node := range removeNodes {
		n.RemoveChild(node)
	}
}

func extractConstraintsFromDescription(description string) string {
	return extractSectionFromDescription(description, "Constraints")
}

func removeConstraintsFromDescription(description string) string {
	return removeSectionFromDescription(description, "Constraints")
}

func extractSectionFromDescription(description string, sectionName string) string {
	lines := strings.Split(description, "\n")
	var section []string
	inSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# "+sectionName || trimmed == "## "+sectionName || trimmed == sectionName {
			inSection = true
			continue
		}
		if inSection {
			if (strings.HasPrefix(trimmed, "#") && trimmed != "# "+sectionName) || 
			   (trimmed != "" && !strings.HasPrefix(trimmed, "#") && isSectionHeader(trimmed)) {
				break
			}
			if trimmed != "" {
				section = append(section, trimmed)
			}
		}
	}

	return strings.Join(section, "\n")
}

func removeSectionFromDescription(description string, sectionName string) string {
	lines := strings.Split(description, "\n")
	var result []string
	inSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# "+sectionName || trimmed == "## "+sectionName || trimmed == sectionName {
			inSection = true
			continue
		}
		if inSection {
			if (strings.HasPrefix(trimmed, "#") && trimmed != "# "+sectionName) || 
			   (trimmed != "" && !strings.HasPrefix(trimmed, "#") && isSectionHeader(trimmed)) {
				inSection = false
				result = append(result, line)
			}
			continue
		}
		result = append(result, line)
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

func isSectionHeader(line string) bool {
	headers := []string{"Input", "Output", "Constraints", "Examples", "Example", "Note"}
	for _, h := range headers {
		if line == h {
			return true
		}
	}
	return false
}

func removeExamplesFromDescription(description string) string {
	lines := strings.Split(description, "\n")
	var result []string
	inExamples := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Example") || strings.HasPrefix(trimmed, "Examples") ||
			strings.HasPrefix(trimmed, "# Example") || strings.HasPrefix(trimmed, "## Example") ||
			strings.HasPrefix(trimmed, "# Examples") || strings.HasPrefix(trimmed, "## Examples") {
			inExamples = true
			continue
		}
		if inExamples {
			if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "# Example") && !strings.HasPrefix(trimmed, "# Examples") {
				inExamples = false
				result = append(result, line)
			}
			continue
		}
		result = append(result, line)
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

func extractCSESSampleCases(doc *html.Node) []model.SampleCase {
	var cases []model.SampleCase
	preNodes := findAllNodes(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "pre"
	})
	for i := 0; i+1 < len(preNodes); i += 2 {
		input := strings.TrimSpace(extractText(preNodes[i]))
		output := strings.TrimSpace(extractText(preNodes[i+1]))
		if input != "" && output != "" && !strings.HasPrefix(input, "<") {
			cases = append(cases, model.SampleCase{
				Input:  input + "\n",
				Output: output + "\n",
			})
		}
	}
	return cases
}

func (p *ProblemParser) ListCodeforcesProblems(ctx context.Context, contestID string) ([]model.ProblemListItem, error) {
	url := fmt.Sprintf("https://codeforces.com/api/contest.standings?contestId=%s&from=1&count=1", contestID)
	result, err := p.fetcher(ctx, url)
	if err != nil {
		return nil, err
	}
	_ = result
	return nil, fmt.Errorf("not implemented: use CF API directly for problem listing")
}

func findNode(n *html.Node, matcher func(*html.Node) bool) *html.Node {
	if matcher(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findNode(c, matcher); found != nil {
			return found
		}
	}
	return nil
}

func findAllNodes(n *html.Node, matcher func(*html.Node) bool) []*html.Node {
	var results []*html.Node
	if matcher(n) {
		results = append(results, n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		results = append(results, findAllNodes(c, matcher)...)
	}
	return results
}

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func extractText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := n.Data
			if sb.Len() > 0 {
				text = strings.TrimLeft(text, "\n")
			}
			sb.WriteString(text)
			return
		}
		if n.Type != html.ElementNode {
			return
		}
		switch n.Data {
		case "br":
			sb.WriteString("\n")
		case "div", "p", "li", "h1", "h2", "h3", "h4", "h5", "h6", "tr":
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
				sb.WriteString("\n")
			}
		default:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
	walk(n)
	return sb.String()
}

// skipHeaderClasses are div classes that should NOT be rendered into the description
var skipHeaderClasses = map[string]bool{
	"time-limit":   true,
	"memory-limit": true,
	"input-file":   true,
	"output-file":  true,
	"title":        true,
	"header":       true,
}

func shouldSkipNode(n *html.Node) bool {
	if n.Type != html.ElementNode || n.Data != "div" {
		return false
	}
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if skipHeaderClasses[c] {
					return true
				}
			}
		}
	}
	return false
}

func renderNodeToMarkdown(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	inPre := false
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
			return
		}
		if n.Type != html.ElementNode {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			return
		}
		// Skip header/metadata divs
		if shouldSkipNode(n) {
			return
		}
		switch n.Data {
		case "p":
			sb.WriteString("\n\n")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("\n\n")
		case "b", "strong":
			sb.WriteString("**")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("**")
		case "i", "em":
			sb.WriteString("*")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("*")
		case "code":
			sb.WriteString("`")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("`")
		case "var":
			if inPre {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
			} else {
				sb.WriteString("$")
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				sb.WriteString("$")
			}
		case "pre":
			inPre = true
			sb.WriteString("\n```\n")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("\n```\n")
			inPre = false
		case "ul":
			sb.WriteString("\n")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Data == "li" {
					sb.WriteString("- ")
					for lc := c.FirstChild; lc != nil; lc = lc.NextSibling {
						walk(lc)
					}
					sb.WriteString("\n")
				}
			}
		case "ol":
			sb.WriteString("\n")
			idx := 1
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Data == "li" {
					sb.WriteString(fmt.Sprintf("%d. ", idx))
					for lc := c.FirstChild; lc != nil; lc = lc.NextSibling {
						walk(lc)
					}
					sb.WriteString("\n")
					idx++
				}
			}
		case "math":
			sb.WriteString("$")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("$")
		case "annotation":
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		case "img":
			for _, attr := range n.Attr {
				if attr.Key == "src" {
					src := attr.Val
					if !strings.HasPrefix(src, "http") && !strings.HasPrefix(src, "data:") {
						src = "https://codeforces.com" + src
					}
					sb.WriteString(fmt.Sprintf("\n![image](%s)\n", src))
					break
				}
			}
		case "br":
			sb.WriteString("\n")
		case "hr":
			sb.WriteString("\n---\n")
		case "h1", "h2":
			prefix := strings.Repeat("#", len(n.Data))
			sb.WriteString(fmt.Sprintf("\n%s ", prefix))
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("\n\n")
		case "h3", "h4", "h5", "h6":
			// skip section labels entirely
		case "table":
			sb.WriteString("\n")
			rows := findAllNodes(n, func(node *html.Node) bool {
				return node.Type == html.ElementNode && node.Data == "tr"
			})
			for ri, row := range rows {
				cells := append(
					findAllNodes(row, func(node *html.Node) bool { return node.Type == html.ElementNode && node.Data == "th" }),
					findAllNodes(row, func(node *html.Node) bool { return node.Type == html.ElementNode && node.Data == "td" })...,
				)
				sb.WriteString("| ")
				for _, cell := range cells {
					sb.WriteString(extractText(cell))
					sb.WriteString(" | ")
				}
				sb.WriteString("\n")
				if ri == 0 {
					sb.WriteString("|")
					for range cells {
						sb.WriteString("---|")
					}
					sb.WriteString("\n")
				}
			}
			sb.WriteString("\n")
	case "span":
		if hasClass(n, "math") {
			if hasClass(n, "math-display") {
				sb.WriteString("$$")
			} else {
				sb.WriteString("$")
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			if hasClass(n, "math-display") {
				sb.WriteString("$$")
			} else {
				sb.WriteString("$")
			}
		} else if hasClass(n, "tex-font-style-bf") {
			sb.WriteString("**")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("**")
		} else if hasClass(n, "tex-font-style-it") {
			sb.WriteString("*")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("*")
		} else if hasClass(n, "tex-font-style-tt") {
			sb.WriteString("`")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("`")
		} else {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	default:
		if n.Data == "div" && hasClass(n, "math-display") {
			sb.WriteString("$$")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("$$")
		} else {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
}
	walk(n)
	result := sb.String()
	// Clean up excessive newlines
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")
	// Convert Codeforces $$$ math notation to standard $ notation
	result = strings.ReplaceAll(result, "$$$", "$")
	return strings.TrimSpace(result)
}

// parseTimeLimit extracts time limit in milliseconds from CF text.
// CF text format: "time limit per test1 second" or "time limit per test2 seconds"
// or "time limit per test500 milliseconds"
func parseTimeLimit(s string) int {
	s = strings.ToLower(s)
	// Remove prefix "time limit per test" or "time limit"
	s = strings.TrimPrefix(s, "time limit per test")
	s = strings.TrimPrefix(s, "time limit")
	s = strings.TrimSpace(s)

	// Try to extract number using regex
	re := regexp.MustCompile(`([\d.]+)\s*(milliseconds?|ms|seconds?|s)\b`)
	matches := re.FindStringSubmatch(s)
	if len(matches) >= 3 {
		var val float64
		fmt.Sscanf(matches[1], "%f", &val)
		unit := matches[2]
		if strings.HasPrefix(unit, "milli") || unit == "ms" {
			return int(val)
		}
		return int(val * 1000)
	}

	// Fallback: try to parse just a number
	var val float64
	fmt.Sscanf(s, "%f", &val)
	if val > 0 {
		if val < 100 {
			return int(val * 1000) // assume seconds
		}
		return int(val) // assume milliseconds
	}
	return 0
}

// parseMemoryLimit extracts memory limit in kilobytes from CF text.
// CF text format: "memory limit per test256 megabytes"
func parseMemoryLimit(s string) int {
	s = strings.ToLower(s)
	// Remove prefix "memory limit per test" or "memory limit"
	s = strings.TrimPrefix(s, "memory limit per test")
	s = strings.TrimPrefix(s, "memory limit")
	s = strings.TrimSpace(s)

	// Try to extract number using regex
	re := regexp.MustCompile(`([\d.]+)\s*(megabytes?|mb|kilobytes?|kb|gigabytes?|gb)\b`)
	matches := re.FindStringSubmatch(s)
	if len(matches) >= 3 {
		var val float64
		fmt.Sscanf(matches[1], "%f", &val)
		unit := matches[2]
		switch {
		case strings.HasPrefix(unit, "mega") || unit == "mb":
			return int(val * 1024)
		case strings.HasPrefix(unit, "giga") || unit == "gb":
			return int(val * 1024 * 1024)
		default: // kilobytes
			return int(val)
		}
	}

	// Fallback: try to parse just a number (assume MB)
	var val float64
	fmt.Sscanf(s, "%f", &val)
	if val > 0 {
		if val < 10000 {
			return int(val * 1024) // assume MB
		}
		return int(val) // assume KB
	}
	return 0
}

func (p *ProblemParser) ParseAtCoderProblem(ctx context.Context, contestID, taskID string) (*model.Problem, error) {
	problemURL := fmt.Sprintf("https://atcoder.jp/contests/%s/tasks/%s", contestID, taskID)
	body, err := p.fetcher(ctx, problemURL)
	if err != nil {
		return nil, fmt.Errorf("fetch AtCoder problem page: %w", err)
	}

	prob := &model.Problem{
		Source:     "atcoder",
		RemoteID:   contestID + "_" + taskID,
		Difficulty: "medium",
	}

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	titleNode := findNode(doc, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "h2" {
			return true
		}
		return n.Type == html.ElementNode && n.Data == "span" && hasClass(n, "h2")
	})
	if titleNode != nil {
		prob.Title = strings.TrimSpace(extractText(titleNode))
		if idx := strings.Index(prob.Title, "Editorial"); idx != -1 {
			prob.Title = strings.TrimSpace(prob.Title[:idx])
		}
		if idx := strings.Index(prob.Title, " - "); idx != -1 {
			prob.Title = strings.TrimSpace(prob.Title[idx+3:])
		}
		prob.Title = strings.Split(prob.Title, "\n")[0]
		prob.Title = strings.TrimSpace(prob.Title)
	}
	if prob.Title == "" {
		prob.Title = fmt.Sprintf("AtCoder %s %s", contestID, taskID)
	}

	tlNode := findNode(doc, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "span" && hasClass(n, "h2") {
			text := extractText(n)
			return strings.Contains(text, "Time Limit") || strings.Contains(text, "time limit")
		}
		return false
	})
	if tlNode != nil {
		prob.TimeLimit = parseAtCoderTimeLimit(extractText(tlNode))
	}
	if prob.TimeLimit == 0 {
		prob.TimeLimit = 2000
	}

	mlNode := findNode(doc, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "span" && hasClass(n, "h2") {
			text := extractText(n)
			return strings.Contains(text, "Memory Limit") || strings.Contains(text, "memory limit")
		}
		return false
	})
	if mlNode != nil {
		prob.MemoryLimit = parseAtCoderMemoryLimit(extractText(mlNode))
	}
	if prob.MemoryLimit == 0 {
		prob.MemoryLimit = 1048576
	}

	prob.Description, prob.InputFormat, prob.OutputFormat, prob.Hint, prob.SampleCases = extractAtCoderDescription(doc)

	return prob, nil
}

func (p *ProblemParser) ParseTophProblem(ctx context.Context, problemID string) (*model.Problem, error) {
	problemURL := fmt.Sprintf("https://toph.co/p/%s", problemID)
	body, err := p.fetcher(ctx, problemURL)
	if err != nil {
		return nil, fmt.Errorf("fetch Toph problem page: %w", err)
	}

	prob := &model.Problem{
		Source:     "toph",
		RemoteID:   problemID,
		Difficulty: "medium",
	}

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	titleNode := findNode(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "h1" && hasClass(n, "problem-title")
	})
	if titleNode == nil {
		titleNode = findNode(doc, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "h1"
		})
	}
	if titleNode != nil {
		prob.Title = strings.TrimSpace(extractText(titleNode))
	}
	if prob.Title == "" {
		prob.Title = fmt.Sprintf("Toph %s", problemID)
	}

	tlNode := findNode(doc, func(n *html.Node) bool {
		if n.Type == html.ElementNode && (n.Data == "span" || n.Data == "div") && hasClass(n, "time-limit") {
			return true
		}
		if n.Type == html.ElementNode && (n.Data == "span" || n.Data == "div") {
			text := extractText(n)
			return strings.Contains(text, "Time Limit") || strings.Contains(text, "time limit")
		}
		return false
	})
	if tlNode != nil {
		prob.TimeLimit = parseTophProblemTimeLimit(extractText(tlNode))
	}
	if prob.TimeLimit == 0 {
		prob.TimeLimit = 1000
	}

	mlNode := findNode(doc, func(n *html.Node) bool {
		if n.Type == html.ElementNode && (n.Data == "span" || n.Data == "div") && hasClass(n, "memory-limit") {
			return true
		}
		if n.Type == html.ElementNode && (n.Data == "span" || n.Data == "div") {
			text := extractText(n)
			return strings.Contains(text, "Memory Limit") || strings.Contains(text, "memory limit")
		}
		return false
	})
	if mlNode != nil {
		prob.MemoryLimit = parseTophProblemMemoryLimit(extractText(mlNode))
	}
	if prob.MemoryLimit == 0 {
		prob.MemoryLimit = 262144
	}

	bodyDiv := findNode(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "problem-body")
	})
	if bodyDiv != nil {
		stripScriptAndStyle(bodyDiv)
		prob.Description = renderNodeToMarkdown(bodyDiv)
	} else {
		for _, div := range findAllNodes(doc, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "div"
		}) {
			text := strings.TrimSpace(extractText(div))
			if len(text) > 50 {
				stripScriptAndStyle(div)
				prob.Description = renderNodeToMarkdown(div)
				break
			}
		}
	}

	prob.InputFormat = extractSectionFromDescription(prob.Description, "Input")
	prob.OutputFormat = extractSectionFromDescription(prob.Description, "Output")

	return prob, nil
}

func (p *ProblemParser) ParseQOJProblem(ctx context.Context, problemID string) (*model.Problem, error) {
	problemURL := fmt.Sprintf("https://qoj.ac/problem/%s", problemID)
	body, err := p.fetcher(ctx, problemURL)
	if err != nil {
		return nil, fmt.Errorf("fetch QOJ problem page: %w", err)
	}

	prob := &model.Problem{
		Source:     "qoj",
		RemoteID:   problemID,
		Difficulty: "medium",
	}

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	h1s := findAllNodes(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "h1"
	})
	for _, n := range h1s {
		txt := strings.TrimSpace(extractText(n))
		if txt != "QOJ.ac" && txt != "QOJ" && txt != "" {
			prob.Title = txt
			break
		}
	}
	if prob.Title == "" {
		prob.Title = fmt.Sprintf("QOJ %s", problemID)
	}

	pdfURL := findQOJPDFLink(doc, problemID)
	if pdfURL != "" {
		prob.Description = pdfURL
		return prob, nil
	}

	contentDiv := findNode(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "problem-content")
	})
	if contentDiv == nil {
		contentDiv = findNode(doc, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "div"
		})
	}
	if contentDiv != nil {
		stripScriptAndStyle(contentDiv)
		prob.Description = renderNodeToMarkdown(contentDiv)
	} else {
		prob.Description = renderNodeToMarkdown(doc)
	}

	prob.TimeLimit = 1000
	prob.MemoryLimit = 262144

	return prob, nil
}

func findQOJPDFLink(doc *html.Node, problemID string) string {
	linkNodes := findAllNodes(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "a" {
			return false
		}
		for _, attr := range n.Attr {
			if attr.Key == "href" {
				href := strings.ToLower(attr.Val)
				return strings.HasSuffix(href, ".pdf") || strings.Contains(href, "/problem.pdf")
			}
		}
		return false
	})

	for _, node := range linkNodes {
		for _, attr := range node.Attr {
			if attr.Key == "href" {
				href := attr.Val
				if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
					return href
				}
				if strings.HasPrefix(href, "//") {
					return "https:" + href
				}
				if strings.HasPrefix(href, "/") {
					return "https://qoj.ac" + href
				}
				return fmt.Sprintf("https://qoj.ac/problems/files/%s/problem.pdf", problemID)
			}
		}
	}
	return ""
}

func parseAtCoderTimeLimit(s string) int {
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "time limit:")
	s = strings.TrimPrefix(s, "time limit")
	s = strings.TrimSpace(s)

	re := regexp.MustCompile(`([\d.]+)\s*(milliseconds?|ms|seconds?|s)\b`)
	matches := re.FindStringSubmatch(s)
	if len(matches) >= 3 {
		var val float64
		fmt.Sscanf(matches[1], "%f", &val)
		unit := matches[2]
		if strings.HasPrefix(unit, "milli") || unit == "ms" {
			return int(val)
		}
		return int(val * 1000)
	}

	var val float64
	fmt.Sscanf(s, "%f", &val)
	if val > 0 {
		if val < 100 {
			return int(val * 1000)
		}
		return int(val)
	}
	return 0
}

func parseAtCoderMemoryLimit(s string) int {
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "memory limit:")
	s = strings.TrimPrefix(s, "memory limit")
	s = strings.TrimSpace(s)

	re := regexp.MustCompile(`([\d.]+)\s*(megabytes?|mb|kilobytes?|kb|gigabytes?|gb)\b`)
	matches := re.FindStringSubmatch(s)
	if len(matches) >= 3 {
		var val float64
		fmt.Sscanf(matches[1], "%f", &val)
		unit := matches[2]
		switch {
		case strings.HasPrefix(unit, "mega") || unit == "mb":
			return int(val * 1024)
		case strings.HasPrefix(unit, "giga") || unit == "gb":
			return int(val * 1024 * 1024)
		default:
			return int(val)
		}
	}

	var val float64
	fmt.Sscanf(s, "%f", &val)
	if val > 0 {
		if val < 10000 {
			return int(val * 1024)
		}
		return int(val)
	}
	return 0
}

func extractAtCoderDescription(doc *html.Node) (string, string, string, string, []model.SampleCase) {
	langSpan := findNode(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "span" && hasClass(n, "lang-en")
	})
	searchRoot := doc
	if langSpan != nil {
		searchRoot = langSpan
	}

	parts := findAllNodes(searchRoot, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "part")
	})

	var descParts, constraints []string
	var inputFormat, outputFormat string
	var sampleCases []model.SampleCase
	for _, part := range parts {
		stripScriptAndStyle(part)
		h3 := findNode(part, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "h3"
		})
		heading := ""
		if h3 != nil {
			heading = strings.TrimSpace(extractText(h3))
		}

		switch {
		case strings.HasPrefix(heading, "Sample Input") || strings.HasPrefix(heading, "Sample Output") ||
			strings.HasPrefix(heading, "入力例") || strings.HasPrefix(heading, "出力例"):
			pre := findNode(part, func(n *html.Node) bool {
				return n.Type == html.ElementNode && n.Data == "pre"
			})
			if pre != nil {
				content := strings.TrimSpace(extractText(pre))
				if strings.HasPrefix(heading, "Sample Input") || strings.HasPrefix(heading, "入力例") {
					for len(sampleCases) > 0 && sampleCases[len(sampleCases)-1].Input != "" {
						sampleCases = append(sampleCases, model.SampleCase{})
					}
					if len(sampleCases) == 0 || sampleCases[len(sampleCases)-1].Input != "" {
						sampleCases = append(sampleCases, model.SampleCase{})
					}
					sampleCases[len(sampleCases)-1].Input = content + "\n"
				} else {
					if len(sampleCases) == 0 || sampleCases[len(sampleCases)-1].Output != "" {
						sampleCases = append(sampleCases, model.SampleCase{})
					}
					sampleCases[len(sampleCases)-1].Output = content + "\n"
				}
			}
		case heading == "Constraints" || heading == "制約":
			md := renderNodeToMarkdown(part)
			constraints = append(constraints, md)
		case heading == "Input" || heading == "入力":
			inputFormat = renderNodeToMarkdown(part)
			inputFormat = strings.TrimSpace(inputFormat)
		case heading == "Output" || heading == "出力":
			outputFormat = renderNodeToMarkdown(part)
			outputFormat = strings.TrimSpace(outputFormat)
		default:
			md := renderNodeToMarkdown(part)
			if md != "" {
				descParts = append(descParts, md)
			}
		}
	}

	description := strings.TrimSpace(strings.Join(descParts, "\n\n"))
	hint := strings.TrimSpace(strings.Join(constraints, "\n"))
	return description, inputFormat, outputFormat, hint, sampleCases
}

func parseTophProblemTimeLimit(s string) int {
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "time limit:")
	s = strings.TrimPrefix(s, "time limit")
	s = strings.TrimSpace(s)

	re := regexp.MustCompile(`([\d.]+)\s*(milliseconds?|ms|seconds?|s)\b`)
	matches := re.FindStringSubmatch(s)
	if len(matches) >= 3 {
		var val float64
		fmt.Sscanf(matches[1], "%f", &val)
		unit := matches[2]
		if strings.HasPrefix(unit, "milli") || unit == "ms" {
			return int(val)
		}
		return int(val * 1000)
	}

	var val float64
	fmt.Sscanf(s, "%f", &val)
	if val > 0 {
		if val < 100 {
			return int(val * 1000)
		}
		return int(val)
	}
	return 0
}

func parseTophProblemMemoryLimit(s string) int {
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "memory limit:")
	s = strings.TrimPrefix(s, "memory limit")
	s = strings.TrimSpace(s)

	re := regexp.MustCompile(`([\d.]+)\s*(megabytes?|mb|kilobytes?|kb|gigabytes?|gb)\b`)
	matches := re.FindStringSubmatch(s)
	if len(matches) >= 3 {
		var val float64
		fmt.Sscanf(matches[1], "%f", &val)
		unit := matches[2]
		switch {
		case strings.HasPrefix(unit, "mega") || unit == "mb":
			return int(val * 1024)
		case strings.HasPrefix(unit, "giga") || unit == "gb":
			return int(val * 1024 * 1024)
		default:
			return int(val)
		}
	}

	var val float64
	fmt.Sscanf(s, "%f", &val)
	if val > 0 {
		if val < 10000 {
			return int(val * 1024)
		}
		return int(val)
	}
	return 0
}

func extractSampleCases(n *html.Node) []model.SampleCase {
	var samples []model.SampleCase
	inputDivs := findAllNodes(n, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "div" && hasClass(node, "input")
	})
	outputDivs := findAllNodes(n, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "div" && hasClass(node, "output")
	})

	count := len(inputDivs)
	if len(outputDivs) < count {
		count = len(outputDivs)
	}

	for i := 0; i < count; i++ {
		inputPre := findNode(inputDivs[i], func(node *html.Node) bool {
			return node.Type == html.ElementNode && node.Data == "pre"
		})
		outputPre := findNode(outputDivs[i], func(node *html.Node) bool {
			return node.Type == html.ElementNode && node.Data == "pre"
		})

		if inputPre != nil && outputPre != nil {
			samples = append(samples, model.SampleCase{
				Input:  strings.TrimSpace(extractText(inputPre)) + "\n",
				Output: strings.TrimSpace(extractText(outputPre)) + "\n",
			})
		}
	}
	return samples
}
