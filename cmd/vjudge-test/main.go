package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tahsinarafat/aioj/internal/vjudge"
)

func main() {
	if len(os.Args) < 2 {
		printUsageAndExit()
	}

	cmd := os.Args[1]
	switch cmd {
	case "parse":
		handleParse()
	case "submit":
		handleSubmit()
	default:
		printUsageAndExit()
	}
}

func printUsageAndExit() {
	fmt.Println("Usage:")
	fmt.Println("  vjudge-test parse <platform> <problem_id> [contest_id]")
	fmt.Println("  vjudge-test submit <platform> <username> <password> <problem_id> <source_file> <language> [cookies_comma_separated]")
	os.Exit(1)
}

func handleParse() {
	fs := flag.NewFlagSet("parse", flag.ExitOnError)
	fs.Parse(os.Args[2:])

	args := fs.Args()
	if len(args) < 2 {
		fmt.Println("Error: missing platform or problem_id")
		printUsageAndExit()
	}

	platform := args[0]
	problemID := args[1]

	parser := vjudge.NewProblemParser(func(ctx context.Context, url string) (string, error) {
		fmt.Printf("Fetching URL: %s\n", url)
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36")
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		fmt.Printf("Response Status: %d\n", resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	})

	ctx := context.Background()
	var prob interface{}
	var err error

	switch platform {
	case "atcoder":
		if len(args) < 3 {
			fmt.Println("Error: AtCoder requires contest_id as third argument (e.g. parse atcoder abc100_a abc100)")
			os.Exit(1)
		}
		contestID := args[2]
		prob, err = parser.ParseAtCoderProblem(ctx, contestID, problemID)
	case "toph":
		prob, err = parser.ParseTophProblem(ctx, problemID)
	case "qoj":
		prob, err = parser.ParseQOJProblem(ctx, problemID)
	default:
		fmt.Printf("Unknown platform: %s\n", platform)
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("Parse failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully parsed problem from %s:\n%+v\n", platform, prob)
}

func handleSubmit() {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)
	fs.Parse(os.Args[2:])

	args := fs.Args()
	if len(args) < 6 {
		fmt.Println("Error: insufficient arguments for submit")
		printUsageAndExit()
	}

	platform := args[0]
	username := args[1]
	password := args[2]
	problemID := args[3]
	sourceFile := args[4]
	language := args[5]

	codeBytes, err := os.ReadFile(sourceFile)
	if err != nil {
		fmt.Printf("Failed to read source file: %v\n", err)
		os.Exit(1)
	}
	sourceCode := string(codeBytes)

	cookies := make(map[string]string)
	if len(args) > 6 {
		cookieParts := strings.Split(args[6], ",")
		for _, part := range cookieParts {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				cookies[kv[0]] = kv[1]
			}
		}
	}

	cfg := vjudge.BotConfig{
		Username: username,
		Password: password,
		Cookies:  cookies,
	}

	var bot vjudge.Bot
	switch platform {
	case "atcoder":
		bot = vjudge.NewAtCoderBot(cfg)
	case "toph":
		bot = vjudge.NewTophBot(cfg)
	case "qoj":
		bot = vjudge.NewQOJBot(cfg)
	default:
		fmt.Printf("Unknown platform: %s\n", platform)
		os.Exit(1)
	}

	ctx := context.Background()

	// State check
	fmt.Printf("Logging in to %s...\n", platform)
	savedCookies, err := bot.Login(ctx)
	if err != nil {
		fmt.Printf("Login error (or cookies warning): %v\n", err)
	}
	if len(savedCookies) > 0 {
		fmt.Printf("Logged in! Session cookies: %v\n", savedCookies)
	}

	fmt.Printf("Submitting code to %s (problem %s)...\n", platform, problemID)
	remoteID, err := bot.Submit(ctx, problemID, sourceCode, language)
	if err != nil {
		fmt.Printf("Submit failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Submitted successfully. Remote ID: %s\n", remoteID)

	// Poll status
	fmt.Println("Polling status (will poll up to 60s)...")
	for i := 0; i < 12; i++ {
		time.Sleep(5 * time.Second)
		res, err := bot.Poll(ctx, remoteID)
		if err != nil {
			fmt.Printf("Poll error: %v\n", err)
			continue
		}
		fmt.Printf("Poll result: Verdict=%s, Done=%t, Time=%dms, Memory=%dKB\n", res.Verdict, res.Done, res.TimeUsed, res.MemoryUsed)
		if res.Done {
			break
		}
	}
}
