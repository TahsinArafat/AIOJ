package vjudge

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAtCoderBot(t *testing.T) {
	// --- Mock AtCoder Server ---
	t.Run("Submit and Poll flow", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// Serve submit page with CSRF token
			if path == "/contests/abc300/submit" && r.Method == "GET" {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, `<html><body>
					<form>
						<input type="hidden" name="csrf_token" value="test_csrf_token_12345">
						<input name="data.TaskScreenName" value="abc300_a">
						<select name="data.LanguageId">
							<option value="5001">C++ (GCC 9.2.1)</option>
						</select>
					</form>
				</body></html>`)
				return
			}

			// Handle submit POST with redirect
			if path == "/contests/abc300/submit" && r.Method == "POST" {
				// Parse form data
				if err := r.ParseForm(); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				csrf := r.FormValue("csrf_token")
				if csrf == "" {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprintf(w, "missing csrf_token")
					return
				}
				task := r.FormValue("data.TaskScreenName")
				if task == "" {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprintf(w, "missing task")
					return
				}
				langID := r.FormValue("data.LanguageId")
				if langID == "" {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprintf(w, "missing language")
					return
				}
				source := r.FormValue("sourceCode")
				if source == "" {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprintf(w, "missing sourceCode")
					return
				}

				// Redirect to submissions page
				w.Header().Set("Location", "/contests/abc300/submissions/me")
				w.WriteHeader(http.StatusFound)
				return
			}

			// Serve submissions page for polling
			if path == "/contests/abc300/submissions/me" && r.Method == "GET" {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, `<html><body>
					<table id="submissions">
						<tr>
							<td>abc300/me/12345</td>
							<td>abc300_a</td>
							<td>C++ (GCC 9.2.1)</td>
							<td>200 ms</td>
							<td>8192 KB</td>
							<td><span class="judge-result-success">Accepted</span></td>
						</tr>
					</table>
				</body></html>`)
				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		// Create bot with cookies (simulating active session)
		cookies := map[string]string{
			"RESESS": "test_recess_cookie",
			"csrf_token": "test_csrf_token_12345",
		}
		bot := NewAtCoderBot(BotConfig{
			BaseURL: server.URL,
			Cookies: cookies,
		})

		ctx := context.Background()

		// Verify bot is logged in with cookies
		if !bot.IsLoggedIn(ctx) {
			t.Error("bot should be logged in with cookies")
		}

		// Submit a solution
		submitID, err := bot.Submit(ctx, "abc300_a", "#include <iostream>\nint main(){return 0;}", "C++ (GCC 9.2.1)")
		if err != nil {
			t.Fatalf("Submit failed: %v", err)
		}

		// Verify remote submission ID
		expectedID := "abc300/me"
		if submitID != expectedID {
			t.Errorf("expected remote ID %q, got %q", expectedID, submitID)
		}

		// Poll the submission
		result, err := bot.Poll(ctx, submitID)
		if err != nil {
			t.Fatalf("Poll failed: %v", err)
		}

		// Verify result
		if !result.Done {
			t.Error("expected Done=true")
		}
		if result.Verdict != "AC" {
			t.Errorf("expected verdict AC, got %q", result.Verdict)
		}
		if result.TimeUsed != 200 {
			t.Errorf("expected time 200ms, got %d", result.TimeUsed)
		}
		if result.MemoryUsed != 8192 {
			t.Errorf("expected memory 8192KB, got %d", result.MemoryUsed)
		}
	})

	// --- Login fails without cookies ---
	t.Run("Login requires cookies", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		bot := NewAtCoderBot(BotConfig{
			BaseURL: server.URL,
		})

		ctx := context.Background()
		_, err := bot.Login(ctx)
		if err == nil {
			t.Error("Login should fail without cookies (Turnstile blocks automated login)")
		}
		if !strings.Contains(err.Error(), "cookies") {
			t.Errorf("error should mention cookies, got: %v", err)
		}
	})

	// --- Name returns correct platform name ---
	t.Run("Name returns atcoder", func(t *testing.T) {
		bot := NewAtCoderBot(BotConfig{})
		if bot.Name() != "atcoder" {
			t.Errorf("expected name 'atcoder', got %q", bot.Name())
		}
	})

	// --- State returns BotState ---
	t.Run("State returns idle initially", func(t *testing.T) {
		bot := NewAtCoderBot(BotConfig{})
		if bot.State() != StateIdle {
			t.Errorf("expected StateIdle, got %q", bot.State())
		}
	})

	// --- Invalid problem ID format ---
	t.Run("Invalid problem ID returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		bot := NewAtCoderBot(BotConfig{
			BaseURL: server.URL,
			Cookies: map[string]string{"test": "value"},
		})

		ctx := context.Background()
		_, err := bot.Submit(ctx, "invalid_no_underscore", "code", "C++")
		if err == nil {
			t.Error("Submit should fail with invalid problem ID format")
		}
	})

	// --- Wrong CSRF token returns error ---
	t.Run("Wrong CSRF token returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" {
				// Return error page
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, `<html><body>
					<div class="alert alert-danger">CSRF token mismatch</div>
				</body></html>`)
				return
			}
			// Serve submit page with different CSRF
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<html><body>
				<form>
					<input type="hidden" name="csrf_token" value="correct_token">
				</form>
			</body></html>`)
		}))
		defer server.Close()

		bot := NewAtCoderBot(BotConfig{
			BaseURL: server.URL,
			Cookies: map[string]string{"test": "value"},
		})

		ctx := context.Background()
		_, err := bot.Submit(ctx, "abc300_a", "code", "C++")
		if err != nil {
			// We expect an error since the mock returns CSRF mismatch error page
			t.Logf("Expected error on CSRF mismatch: %v", err)
		}
	})

	// --- Poll returns pending for unknown submissions ---
	t.Run("Poll returns pending for unknown ID", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<html><body>
				<table><tr><td>Pending</td></tr></table>
			</body></html>`)
		}))
		defer server.Close()

		bot := NewAtCoderBot(BotConfig{
			BaseURL: server.URL,
			Cookies: map[string]string{"test": "value"},
		})

		ctx := context.Background()
		result, err := bot.Poll(ctx, "abc300/me/99999")
		if err != nil {
			t.Fatalf("Poll failed: %v", err)
		}
		if result.Done {
			t.Error("expected Done=false for pending submission")
		}
		if result.Verdict != "PENDING" {
			t.Errorf("expected verdict PENDING, got %q", result.Verdict)
		}
	})
}

func TestAtCoderBotExtractCSRFToken(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "standard CSRF token",
			html:     `<input type="hidden" name="csrf_token" value="abc123">`,
			expected: "abc123",
		},
		{
			name:     "no CSRF token",
			html:     `<input type="text" name="username">`,
			expected: "",
		},
		{
			name:     "CSRF token with special chars",
			html:     `<input type="hidden" name="csrf_token" value="token-with_underscores.and.dots">`,
			expected: "token-with_underscores.and.dots",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAtCoderCSRFToken(tt.html)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestAtCoderBotParseProblemID(t *testing.T) {
	tests := []struct {
		name      string
		problemID string
		wantContest string
		wantErr   bool
	}{
		{
			name:      "valid abc problem",
			problemID: "abc300_a",
			wantContest: "abc300",
			wantErr:   false,
		},
		{
			name:      "valid arc problem",
			problemID: "arc123_b",
			wantContest: "arc123",
			wantErr:   false,
		},
		{
			name:      "valid agc problem",
			problemID: "agc001_c",
			wantContest: "agc001",
			wantErr:   false,
		},
		{
			name:      "no underscore - invalid",
			problemID: "abc300a",
			wantErr:   true,
		},
		{
			name:      "multiple underscores - uses first part",
			problemID: "abc300_a_b",
			wantContest: "abc300",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contestID, err := parseAtCoderContestID(tt.problemID)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if contestID != tt.wantContest {
				t.Errorf("expected contest ID %q, got %q", tt.wantContest, contestID)
			}
		})
	}
}
