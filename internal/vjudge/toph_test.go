package vjudge

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTophBot(t *testing.T) {
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
    <input type="text" name="handle" />
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
					handle := r.FormValue("handle")
					password := r.FormValue("password")

					if handle == "testuser" && password == "testpass" {
						http.SetCookie(w, &http.Cookie{Name: "session", Value: "session_abc123", Path: "/"})
						w.Header().Set("Location", "/")
						w.WriteHeader(http.StatusFound)
						return
					}
					http.Error(w, "Invalid handle or password", http.StatusUnauthorized)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		bot := NewTophBot(BotConfig{
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
    <input type="text" name="handle" />
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

		bot := NewTophBot(BotConfig{
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

	t.Run("LoginEmptyCredentials", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/login":
				if r.Method == "GET" {
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, `<html><body>Login page</body></html>`)
					return
				}
				if r.Method == "POST" {
					http.Error(w, "invalid", http.StatusUnauthorized)
				}
			}
		}))
		defer server.Close()

		bot := NewTophBot(BotConfig{
			Username: "",
			Password: "",
			BaseURL:  server.URL,
		})

		ctx := context.Background()
		_, err := bot.Login(ctx)
		if err == nil {
			t.Error("Login should fail when credentials are empty")
		}
	})

	t.Run("LoginFetchError", func(t *testing.T) {
		bot := NewTophBot(BotConfig{
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
			case "/p/1001":
				if r.Method == "GET" {
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<body>
<a href="/logout">Logout</a>
<form method="POST" action="/submit">
    <input type="hidden" name="csrf" value="csrf_token_67890">
    <textarea name="source_code"></textarea>
    <select name="language">
        <option value="cpp17">C++ 17</option>
        <option value="python3">Python 3</option>
    </select>
    <button type="submit">Submit</button>
</form>
</body>
</html>`)
					return
				}
			case "/p/1001/submit":
				if r.Method == "POST" {
					if err := r.ParseMultipartForm(10 << 20); err != nil {
						http.Error(w, "bad form", http.StatusBadRequest)
						return
					}
					langID := r.FormValue("languageId")
					file, _, err := r.FormFile("source")
					if err != nil || langID == "" || file == nil {
						http.Error(w, "missing fields", http.StatusBadRequest)
						return
					}
					w.Header().Set("Location", "/status/12345")
					w.WriteHeader(http.StatusFound)
					return
				}
			case "/status/12345":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<body>
<div class="verdict">Accepted</div>
<div class="time">0.5s</div>
<div class="memory">32768 KB</div>
</body>
</html>`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		bot := NewTophBot(BotConfig{
			Username: "testuser",
			Password: "testpass",
			BaseURL:  server.URL,
		})

		ctx := context.Background()

		remoteID, err := bot.Submit(ctx, "1001", "#include <iostream>\nint main() { return 0; }", "cpp17")
		if err != nil {
			t.Fatalf("Submit failed: %v", err)
		}
		if remoteID == "" {
			t.Error("Submit returned empty remote ID")
		}
		if remoteID != "12345" {
			t.Errorf("Expected remote ID '12345', got '%s'", remoteID)
		}
	})

	t.Run("SubmitFetchError", func(t *testing.T) {
		bot := NewTophBot(BotConfig{
			Username: "user",
			Password: "pass",
			BaseURL:  "http://127.0.0.1:1",
		})

		ctx := context.Background()
		_, err := bot.Submit(ctx, "1001", "code", "cpp17")
		if err == nil {
			t.Error("Submit should fail when server is unreachable")
		}
	})

	t.Run("Poll", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/s/12345":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<body>
<div class="verdict">Accepted</div>
<div class="time">0.25s</div>
<div class="memory">16384 KB</div>
</body>
</html>`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		bot := NewTophBot(BotConfig{
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
			case "/s/99999":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<body>
<div class="verdict">Running</div>
<div class="time">-</div>
<div class="memory">-</div>
</body>
</html>`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		bot := NewTophBot(BotConfig{
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
		bot := NewTophBot(BotConfig{
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

		bot := NewTophBot(BotConfig{
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
		bot := NewTophBot(BotConfig{})
		ctx := context.Background()

		result, err := bot.Poll(ctx, "toph-123456")
		if err != nil {
			t.Fatalf("Poll failed: %v", err)
		}
		if result == nil {
			t.Fatal("Poll returned nil result")
		}
		if result.Done {
			t.Error("Local toph- IDs should not be done")
		}
		if result.Verdict != "PENDING" {
			t.Errorf("Expected PENDING, got %s", result.Verdict)
		}
	})

	t.Run("Name", func(t *testing.T) {
		bot := NewTophBot(BotConfig{})
		if bot.Name() != "toph" {
			t.Errorf("Expected name 'toph', got %s", bot.Name())
		}
	})

	t.Run("State", func(t *testing.T) {
		bot := NewTophBot(BotConfig{})
		if bot.State() != StateIdle {
			t.Errorf("Expected state idle, got %s", bot.State())
		}
	})
}
