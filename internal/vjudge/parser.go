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

	// Extract input/output file info from .input-file and .output-file divs
	// CF text format: "inputstdin" or "inputstandard input"
	//                 "outputstdout" or "outputstandard output"
	inputFileDiv := findNode(problemDiv, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "input-file")
	})
	outputFileDiv := findNode(problemDiv, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "output-file")
	})

	ioDescription := ""
	if inputFileDiv != nil {
		inputText := strings.TrimSpace(extractText(inputFileDiv))
		inputText = strings.TrimPrefix(inputText, "input")
		inputText = strings.TrimSpace(inputText)
		ioDescription += "**Input:** " + inputText + "\n"
	}
	if outputFileDiv != nil {
		outputText := strings.TrimSpace(extractText(outputFileDiv))
		outputText = strings.TrimPrefix(outputText, "output")
		outputText = strings.TrimSpace(outputText)
		ioDescription += "**Output:** " + outputText + "\n"
	}

	// Extract description: render ONLY the actual statement content, skipping header divs
	descriptionDiv := findNode(problemDiv, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "ttypography")
	})
	if descriptionDiv != nil {
		prob.Description = renderNodeToMarkdown(descriptionDiv)
	} else {
		prob.Description = renderNodeToMarkdown(problemDiv)
	}

	// Prepend I/O description if available
	if ioDescription != "" {
		prob.Description = ioDescription + "\n" + prob.Description
	}

	// Extract sample test cases
	sampleDivs := findAllNodes(problemDiv, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "sample-tests")
	})
	if len(sampleDivs) > 0 {
		prob.SampleCases = extractSampleCases(sampleDivs[0])
	}

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
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
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
		case "pre":
			sb.WriteString("\n```\n")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("\n```\n")
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
					if !strings.HasPrefix(src, "http") {
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
		case "h1", "h2", "h3":
			prefix := strings.Repeat("#", len(n.Data)-1)
			sb.WriteString(fmt.Sprintf("\n%s ", prefix))
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			sb.WriteString("\n\n")
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
			// Check if this is a MathJax inline math span (class "mjx-chtml" or similar)
			// CF uses $$$...$$$ for inline math which gets rendered as <span class="mjx-chtml">
			// We just walk children to get the text content
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		default:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
	walk(n)
	result := sb.String()
	// Clean up excessive newlines
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")
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
