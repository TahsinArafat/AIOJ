package vjudge

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQOJBot(t *testing.T) {
	t.Run("Login", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/login":
				if r.Method == "GET" {
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<head><title>Login</title></head>
<body>
<form method="POST" action="/login">
    <input type="hidden" name="_token" value="csrf_token_12345">
    <input type="text" name="user_name" />
    <input type="password" name="password" />
    <button type="submit">Login</button>
</form>
</body>
</html>`)
					return
				}
				if r.Method == "POST" {
					if err := r.ParseForm(); err != nil {
						http.Error(w, "bad form", http.StatusBadRequest)
						return
					}
					username := r.FormValue("user_name")
					password := r.FormValue("password")
					csrf := r.FormValue("_token")

					if csrf != "csrf_token_12345" {
						http.Error(w, "invalid csrf", http.StatusForbidden)
						return
					}
					if username == "testuser" && password == "testpass" {
						http.SetCookie(w, &http.Cookie{Name: "UOJSESSID", Value: "session_abc123", Path: "/"})
						w.Header().Set("Location", "/submissions")
						w.WriteHeader(http.StatusFound)
						return
					}
					http.Error(w, "login failed", http.StatusUnauthorized)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		bot := NewQOJBot(BotConfig{
			Username: "testuser",
			Password: "testpass",
			BaseURL:  server.URL,
		})

		ctx := context.Background()

		_, err := bot.Login(ctx)
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}
		if !bot.IsLoggedIn(ctx) {
			t.Error("IsLoggedIn should return true after successful login")
		}
	})

	t.Run("LoginInvalidCredentials", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/login":
				if r.Method == "GET" {
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<body>
<form method="POST" action="/login">
    <input type="hidden" name="_token" value="csrf_token_12345">
    <input type="text" name="user_name" />
    <input type="password" name="password" />
</form>
</body>
</html>`)
					return
				}
				if r.Method == "POST" {
					http.Error(w, "Invalid username or password", http.StatusUnauthorized)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		bot := NewQOJBot(BotConfig{
			Username: "wronguser",
			Password: "wrongpass",
			BaseURL:  server.URL,
		})

		ctx := context.Background()
		_, err := bot.Login(ctx)
		if err == nil {
			t.Error("Login should fail with invalid credentials")
		}
		if bot.IsLoggedIn(ctx) {
			t.Error("IsLoggedIn should return false after failed login")
		}
	})

	t.Run("LoginNoCSRFToken", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><body><form>No token here</form></body></html>`)
		}))
		defer server.Close()

		bot := NewQOJBot(BotConfig{
			Username: "user",
			Password: "pass",
			BaseURL:  server.URL,
		})

		ctx := context.Background()
		_, err := bot.Login(ctx)
		if err == nil {
			t.Error("Login should fail when no CSRF token found")
		}
		if !strings.Contains(err.Error(), "no CSRF token") {
			t.Errorf("Expected CSRF token error, got: %v", err)
		}
	})

	t.Run("LoginFetchError", func(t *testing.T) {
		bot := NewQOJBot(BotConfig{
			Username: "user",
			Password: "pass",
			BaseURL:  "http://127.0.0.1:1", // unreachable
		})

		ctx := context.Background()
		_, err := bot.Login(ctx)
		if err == nil {
			t.Error("Login should fail when server is unreachable")
		}
	})

	t.Run("Submit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/problem/1001":
				if r.Method == "GET" {
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<body>
<a href="/logout">Logout</a>
<form method="POST" action="/problem/1001/submit">
    <input type="hidden" name="_token" value="csrf_token_67890">
    <textarea name="answer"></textarea>
    <select name="language">
        <option value="0">C++</option>
        <option value="1">C</option>
    </select>
    <button type="submit">Submit</button>
</form>
</body>
</html>`)
					return
				}
			case "/problem/1001/submit":
				if r.Method == "POST" {
					if err := r.ParseForm(); err != nil {
						http.Error(w, "bad form", http.StatusBadRequest)
						return
					}
					csrf := r.FormValue("_token")
					if csrf != "csrf_token_67890" {
						http.Error(w, "invalid csrf", http.StatusForbidden)
						return
					}
					answer := r.FormValue("answer")
					lang := r.FormValue("language")
					if answer == "" || lang == "" {
						http.Error(w, "missing fields", http.StatusBadRequest)
						return
					}
					w.Header().Set("Location", "/submission/12345")
					w.WriteHeader(http.StatusFound)
					return
				}
			case "/submission/12345":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<body>
<table>
<tr><td>12345</td><td>Accepted</td><td>0.5s</td><td>32768KB</td></tr>
</table>
</body>
</html>`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		bot := NewQOJBot(BotConfig{
			Username: "testuser",
			Password: "testpass",
			BaseURL:  server.URL,
		})

		ctx := context.Background()

		remoteID, err := bot.Submit(ctx, "1001", "#include <iostream>\nint main() { return 0; }", "0")
		if err != nil {
			t.Fatalf("Submit failed: %v", err)
		}
		if remoteID == "" {
			t.Error("Submit returned empty remote ID")
		}
		if !strings.Contains(remoteID, "qoj") && remoteID != "" {
			t.Logf("Remote ID: %s", remoteID)
		}
	})

	t.Run("SubmitFetchError", func(t *testing.T) {
		bot := NewQOJBot(BotConfig{
			Username: "user",
			Password: "pass",
			BaseURL:  "http://127.0.0.1:1",
		})

		ctx := context.Background()
		_, err := bot.Submit(ctx, "1001", "code", "0")
		if err == nil {
			t.Error("Submit should fail when server is unreachable")
		}
	})

	t.Run("Poll", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/submission/12345":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<body>
<table>
<tr><td>12345</td><td>Accepted</td><td>0.25s</td><td>16384KB</td></tr>
</table>
</body>
</html>`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		bot := NewQOJBot(BotConfig{
			BaseURL: server.URL,
		})

		ctx := context.Background()

		result, err := bot.Poll(ctx, "12345")
		if err != nil {
			t.Fatalf("Poll failed: %v", err)
		}
		if result == nil {
			t.Fatal("Poll returned nil result")
		}
		if !result.Done {
			t.Error("Poll should return Done=true for judged submission")
		}
		if result.Verdict != "AC" {
			t.Errorf("Expected verdict AC, got %s", result.Verdict)
		}
		if result.TimeUsed != 250 {
			t.Errorf("Expected time 250ms, got %d", result.TimeUsed)
		}
		if result.MemoryUsed != 16384 {
			t.Errorf("Expected memory 16384KB, got %d", result.MemoryUsed)
		}
	})

	t.Run("PollPending", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/submission/99999":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<body>
<table>
<tr><td>99999</td><td>Running</td><td>-</td><td>-</td></tr>
</table>
</body>
</html>`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		bot := NewQOJBot(BotConfig{
			BaseURL: server.URL,
		})

		ctx := context.Background()

		result, err := bot.Poll(ctx, "99999")
		if err != nil {
			t.Fatalf("Poll failed: %v", err)
		}
		if result == nil {
			t.Fatal("Poll returned nil result")
		}
		if result.Done {
			t.Error("Poll should return Done=false for pending submission")
		}
		if result.Verdict != "PENDING" {
			t.Errorf("Expected verdict PENDING, got %s", result.Verdict)
		}
	})

	t.Run("PollFetchError", func(t *testing.T) {
		bot := NewQOJBot(BotConfig{
			BaseURL: "http://127.0.0.1:1",
		})

		ctx := context.Background()
		result, err := bot.Poll(ctx, "12345")
		if err == nil {
			t.Error("Poll should return error when server is unreachable")
		}
		if result != nil {
			t.Error("Poll should return nil result on error")
		}
	})

	t.Run("PollServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}))
		defer server.Close()

		bot := NewQOJBot(BotConfig{
			BaseURL: server.URL,
		})

		ctx := context.Background()
		result, err := bot.Poll(ctx, "12345")
		if err == nil {
			t.Error("Poll should return error for non-2xx response")
		}
		if result != nil {
			t.Error("Poll should return nil result on error")
		}
	})

	t.Run("PollLocalID", func(t *testing.T) {
		bot := NewQOJBot(BotConfig{})
		ctx := context.Background()

		result, err := bot.Poll(ctx, "qoj-123456")
		if err != nil {
			t.Fatalf("Poll failed: %v", err)
		}
		if result == nil {
			t.Fatal("Poll returned nil result")
		}
		if result.Done {
			t.Error("Local qoj- IDs should not be done")
		}
		if result.Verdict != "PENDING" {
			t.Errorf("Expected PENDING, got %s", result.Verdict)
		}
	})

	t.Run("Name", func(t *testing.T) {
		bot := NewQOJBot(BotConfig{})
		if bot.Name() != "qoj" {
			t.Errorf("Expected name 'qoj', got %s", bot.Name())
		}
	})

	t.Run("State", func(t *testing.T) {
		bot := NewQOJBot(BotConfig{})
		if bot.State() != StateIdle {
			t.Errorf("Expected state idle, got %s", bot.State())
		}
	})
}
