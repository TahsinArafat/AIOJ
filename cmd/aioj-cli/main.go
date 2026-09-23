// Command aioj-cli is a thin HTTP client for the AIOJ API.
//
// Usage:
//
//	aioj-cli login -url https://aioj.com -user alice -pass secret
//	aioj-cli submit -url https://aioj.com -problem two-sum -lang cpp-gpp-64 -file sol.cpp
//	aioj-cli submit -url https://aioj.com -problem two-sum -lang python -file sol.py -contest 1
//
// Tokens are stored under ~/.config/aioj/token (0600).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "login":
		cmdLogin(os.Args[2:])
	case "submit":
		cmdSubmit(os.Args[2:])
	case "health":
		cmdHealth(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `aioj-cli — AIOJ command-line client

Commands:
  login   -url URL -user NAME -pass PASSWORD
  submit  -url URL -problem SLUG -lang KEY -file PATH [-contest ID]
  health  -url URL
`)
}

func cmdLogin(args []string) {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	url := fs.String("url", "http://localhost:8081", "API base URL")
	user := fs.String("user", "", "username")
	pass := fs.String("pass", "", "password")
	_ = fs.Parse(args)
	if *user == "" || *pass == "" {
		fmt.Fprintln(os.Stderr, "-user and -pass required")
		os.Exit(2)
	}
	body, _ := json.Marshal(map[string]string{"username": *user, "password": *pass})
	resp, err := http.Post(strings.TrimRight(*url, "/")+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		fatal(fmt.Errorf("login %d: %s", resp.StatusCode, truncate(string(raw), 300)))
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Token        string `json:"token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		fatal(err)
	}
	tok := out.AccessToken
	if tok == "" {
		tok = out.Token
	}
	if tok == "" {
		fatal(fmt.Errorf("no access_token in response"))
	}
	if err := saveToken(tok); err != nil {
		fatal(err)
	}
	fmt.Println("ok — token saved to", tokenPath())
}

func cmdSubmit(args []string) {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)
	url := fs.String("url", "http://localhost:8081", "API base URL")
	problem := fs.String("problem", "", "problem id or slug")
	lang := fs.String("lang", "", "language key, e.g. cpp-gpp-64")
	file := fs.String("file", "", "source file path")
	contest := fs.String("contest", "", "optional contest id")
	_ = fs.Parse(args)
	if *problem == "" || *lang == "" || *file == "" {
		fmt.Fprintln(os.Stderr, "-problem, -lang, -file required")
		os.Exit(2)
	}
	src, err := os.ReadFile(*file)
	if err != nil {
		fatal(err)
	}
	tok, err := loadToken()
	if err != nil {
		fatal(err)
	}
	payload := map[string]string{
		"problem_id":  *problem,
		"language":    *lang,
		"source_code": string(src),
	}
	if *contest != "" {
		payload["contest_id"] = *contest
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, strings.TrimRight(*url, "/")+"/api/submissions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		fatal(fmt.Errorf("submit %d: %s", resp.StatusCode, truncate(string(raw), 400)))
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	if out.ID == "" {
		fmt.Println(string(raw))
		return
	}
	fmt.Println("submitted", out.ID)
}

func cmdHealth(args []string) {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	url := fs.String("url", "http://localhost:8081", "API base URL")
	_ = fs.Parse(args)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(strings.TrimRight(*url, "/") + "/api/health")
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	fmt.Printf("HTTP %d %s\n", resp.StatusCode, truncate(string(raw), 400))
	if resp.StatusCode >= 300 {
		os.Exit(1)
	}
}

func tokenPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "aioj-token")
	}
	return filepath.Join(dir, "aioj", "token")
}

func saveToken(tok string) error {
	p := tokenPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(tok+"\n"), 0o600)
}

func loadToken() (string, error) {
	b, err := os.ReadFile(tokenPath())
	if err != nil {
		return "", fmt.Errorf("not logged in (run aioj-cli login): %w", err)
	}
	t := strings.TrimSpace(string(b))
	if t == "" {
		return "", fmt.Errorf("empty token")
	}
	return t, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
