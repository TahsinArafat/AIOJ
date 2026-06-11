# Admin Backup & Restore System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a comprehensive web-based backup and restore utility in the AIOJ admin dashboard that supports database dumps, test cases/uploaded media archives, or combined full backups, protected by admin password verification.

**Architecture:** The backend will run standard `pg_dump` and `pg_restore` commands (leveraging a newly installed `postgresql-client` in Alpine) for high fidelity, and `tar`/`gzip` for system files. Backups and JSON metadata will be written to `/app/backups`, which is persisted by a Docker volume. The frontend React UI provides a clean console for creating, downloading, uploading, and deleting backups, as well as a password-secured confirmation modal for database restoration.

**Tech Stack:** Go 1.26, PostgreSQL 18, React 19, Vite, Tailwind CSS, Lucide icons, `postgresql-client`, `tar`, `gzip`.

---

## Technical Mapping & Volume Persistence
*   **Docker Volume Mount**: Mount `/app/backups` inside the backend container to a persistent volume `backups` in `docker-compose.yml`.
*   **CLI Processes**: Go will spawn `pg_dump` and `psql` / `pg_restore` for database tasks. It will spawn `tar` for archiving `./testdata` and `./media` directories.

---

### Task 1: Add PostgreSQL CLI client and Tar to Docker Image

**Files:**
- Modify: `Dockerfile`

- [ ] **Step 1: Edit Dockerfile to install postgresql-client and tar**
  Modify line 12-13 in `Dockerfile` to include `postgresql-client` and `tar` packages.

  ```dockerfile
  # Stage 2: Run
  FROM alpine:3.19
  RUN apk add --no-cache ca-certificates tzdata \
      g++ gcc make musl-dev python3 openjdk21-jdk rust cargo nodejs npm bash postgresql-client tar
  ```

- [ ] **Step 2: Build the Docker image locally to verify package installation**
  Run: `docker compose build backend`
  Expected: Build finishes with exit code 0.

- [ ] **Step 3: Run check inside container to verify postgresql-client and tar are present**
  Run: `docker compose run --rm backend sh -c "pg_dump --version && tar --version"`
  Expected: Outputs the versions of pg_dump and tar (exit code 0).

- [ ] **Step 4: Commit**
  Run: `git add Dockerfile && git commit -m "build: install postgresql-client and tar in backend image"`

---

### Task 2: Implement Admin Backup & Restore Handler (Go)

**Files:**
- Create: `internal/api/handler/admin_backup.go`

- [ ] **Step 1: Write the AdminBackupHandler boilerplate and metadata structures**
  Create the file `/Users/tahsinarafat/App_Dev/AIOJ/internal/api/handler/admin_backup.go` and implement the struct and initialization.

  ```go
  package handler

  import (
  	"context"
  	"encoding/json"
  	"fmt"
  	"io"
  	"math/rand"
  	"net/http"
  	"os"
  	"os/exec"
  	"path/filepath"
  	"time"

  	"github.com/go-chi/chi/v5"
  	"github.com/tahsinarafat/aioj/internal/api/middleware"
  	"github.com/tahsinarafat/aioj/internal/config"
  	"github.com/tahsinarafat/aioj/internal/store"
  	"golang.org/x/crypto/bcrypt"
  )

  type BackupMeta struct {
  	Filename  string    `json:"filename"`
  	Type      string    `json:"type"` // "db", "files", "full"
  	CreatedAt time.Time `json:"created_at"`
  	Size      int64     `json:"size"`
  	Version   string    `json:"version"`
  	CreatedBy string    `json:"created_by"`
  }

  type AdminBackupHandler struct {
  	userStore store.UserStore
  	dbConfig  config.DatabaseConfig
  	backupDir string
  }

  func NewAdminBackupHandler(u store.UserStore, dbCfg config.DatabaseConfig, dir string) *AdminBackupHandler {
  	if dir == "" {
  		dir = "./backups"
  	}
  	return &AdminBackupHandler{
  		userStore: u,
  		dbConfig:  dbCfg,
  		backupDir: dir,
  	}
  }
  ```

- [ ] **Step 2: Add List and Delete methods**
  Implement the `List` and `Delete` endpoints in `internal/api/handler/admin_backup.go`.

  ```go
  func (h *AdminBackupHandler) List(w http.ResponseWriter, r *http.Request) {
  	if err := os.MkdirAll(h.backupDir, 0755); err != nil {
  		http.Error(w, "failed to create backup directory", http.StatusInternalServerError)
  		return
  	}

  	entries, err := os.ReadDir(h.backupDir)
  	if err != nil {
  		http.Error(w, "failed to read backup directory", http.StatusInternalServerError)
  		return
  	}

  	var backups []BackupMeta
  	for _, entry := range entries {
  		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
  			continue
  		}
  		metaPath := filepath.Join(h.backupDir, entry.Name())
  		data, err := os.ReadFile(metaPath)
  		if err != nil {
  			continue
  		}
  		var meta BackupMeta
  		if err := json.Unmarshal(data, &meta); err == nil {
  			// Verify tarball file actually exists
  			archivePath := filepath.Join(h.backupDir, meta.Filename)
  			if _, err := os.Stat(archivePath); err == nil {
  				backups = append(backups, meta)
  			}
  		}
  	}

  	respondJSON(w, http.StatusOK, map[string]interface{}{"data": backups})
  }

  func (h *AdminBackupHandler) Delete(w http.ResponseWriter, r *http.Request) {
  	filename := chi.URLParam(r, "filename")
  	if filename == "" {
  		http.Error(w, "filename is required", http.StatusBadRequest)
  		return
  	}

  	// Prevent directory traversal
  	cleanName := filepath.Base(filename)
  	archivePath := filepath.Join(h.backupDir, cleanName)
  	metaPath := filepath.Join(h.backupDir, cleanName[:len(cleanName)-len(filepath.Ext(cleanName))]+".json")

  	_ = os.Remove(archivePath)
  	_ = os.Remove(metaPath)

  	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
  }
  ```

- [ ] **Step 3: Add Download and Upload methods**
  Implement `Download` and `Upload` endpoints in `internal/api/handler/admin_backup.go`.

  ```go
  func (h *AdminBackupHandler) Download(w http.ResponseWriter, r *http.Request) {
  	filename := chi.URLParam(r, "filename")
  	if filename == "" {
  		http.Error(w, "filename is required", http.StatusBadRequest)
  		return
  	}

  	cleanName := filepath.Base(filename)
  	archivePath := filepath.Join(h.backupDir, cleanName)

  	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
  		http.Error(w, "backup file not found", http.StatusNotFound)
  		return
  	}

  	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", cleanName))
  	w.Header().Set("Content-Type", "application/gzip")
  	http.ServeFile(w, r, archivePath)
  }

  func (h *AdminBackupHandler) Upload(w http.ResponseWriter, r *http.Request) {
  	claims := middleware.GetUserClaims(r)
  	if claims == nil {
  		http.Error(w, "unauthorized", http.StatusUnauthorized)
  		return
  	}

  	// Parse multipart form (max 200MB)
  	if err := r.ParseMultipartForm(200 << 20); err != nil {
  		http.Error(w, "file too large or invalid multipart form", http.StatusBadRequest)
  		return
  	}

  	file, header, err := r.FormFile("file")
  	if err != nil {
  		http.Error(w, "file is required", http.StatusBadRequest)
  		return
  	}
  	defer file.Close()

  	if err := os.MkdirAll(h.backupDir, 0755); err != nil {
  		http.Error(w, "failed to create backup directory", http.StatusInternalServerError)
  		return
  	}

  	cleanName := filepath.Base(header.Filename)
  	archivePath := filepath.Join(h.backupDir, cleanName)

  	dst, err := os.Create(archivePath)
  	if err != nil {
  		http.Error(w, "failed to create destination file", http.StatusInternalServerError)
  		return
  	}
  	defer dst.Close()

  	if _, err := io.Copy(dst, file); err != nil {
  		http.Error(w, "failed to save uploaded file", http.StatusInternalServerError)
  		return
  	}

  	// Create corresponding JSON metadata
  	bType := "db"
  	if filepath.Ext(cleanName) == ".gz" && filepath.Base(cleanName)[:12] == "aioj_backup_" {
  		// Try parsing type from name aioj_backup_<type>_...
  		parts := filepath.Base(cleanName)
  		if len(parts) > 12 {
  			sub := parts[12:]
  			if len(sub) >= 2 && sub[:2] == "db" {
  				bType = "db"
  			} else if len(sub) >= 5 && sub[:5] == "files" {
  				bType = "files"
  			} else if len(sub) >= 4 && sub[:4] == "full" {
  				bType = "full"
  			}
  		}
  	}

  	stat, _ := dst.Stat()
  	meta := BackupMeta{
  		Filename:  cleanName,
  		Type:      bType,
  		CreatedAt: time.Now(),
  		Size:      stat.Size(),
  		Version:   "51",
  		CreatedBy: claims.Username,
  	}

  	metaPath := filepath.Join(h.backupDir, cleanName[:len(cleanName)-len(filepath.Ext(cleanName))]+".json")
  	metaData, _ := json.MarshalIndent(meta, "", "  ")
  	_ = os.WriteFile(metaPath, metaData, 0644)

  	respondJSON(w, http.StatusCreated, map[string]string{"status": "uploaded", "filename": cleanName})
  }
  ```

- [ ] **Step 4: Add Create (Backup Generation) implementation**
  Add backup generation to `internal/api/handler/admin_backup.go`.

  ```go
  func (h *AdminBackupHandler) Create(w http.ResponseWriter, r *http.Request) {
  	claims := middleware.GetUserClaims(r)
  	if claims == nil {
  		http.Error(w, "unauthorized", http.StatusUnauthorized)
  		return
  	}

  	var req struct {
  		Type string `json:"type"` // "db", "files", "full"
  	}
  	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
  		http.Error(w, "invalid body", http.StatusBadRequest)
  		return
  	}

  	if req.Type != "db" && req.Type != "files" && req.Type != "full" {
  		http.Error(w, "invalid type: must be db, files, or full", http.StatusBadRequest)
  		return
  	}

  	if err := os.MkdirAll(h.backupDir, 0755); err != nil {
  		http.Error(w, "failed to create backup directory", http.StatusInternalServerError)
  		return
  	}

  	timestamp := time.Now().Format("20060102_150405")
  	randID := fmt.Sprintf("%06d", rand.Intn(1000000))
  	baseName := fmt.Sprintf("aioj_backup_%s_%s_%s", req.Type, timestamp, randID)
  	archiveName := baseName + ".tar.gz"
  	archivePath := filepath.Join(h.backupDir, archiveName)

  	// Create temporary directory for gathering data
  	tempDir, err := os.MkdirTemp("", "aioj-backup-*")
  	if err != nil {
  		http.Error(w, "failed to create temp directory", http.StatusInternalServerError)
  		return
  	}
  	defer os.RemoveAll(tempDir)

  	// 1. Export database if required
  	if req.Type == "db" || req.Type == "full" {
  		sqlFile := filepath.Join(tempDir, "db.sql")
  		// Set PGPASSWORD env variable to communicate with postgres safely without prompt
  		cmd := exec.Command("pg_dump", "-h", h.dbConfig.Host, "-p", fmt.Sprintf("%d", h.dbConfig.Port),
  			"-U", h.dbConfig.User, "-d", h.dbConfig.Name, "--clean", "--if-exists", "--no-owner", "--no-privileges", "-f", sqlFile)
  		cmd.Env = append(os.Environ(), "PGPASSWORD="+h.dbConfig.Password)
  		if output, err := cmd.CombinedOutput(); err != nil {
  			http.Error(w, fmt.Sprintf("database backup failed: %v, output: %s", err, string(output)), http.StatusInternalServerError)
  			return
  		}
  	}

  	// 2. Compress files if required
  	if req.Type == "files" || req.Type == "full" {
  		filesTar := filepath.Join(tempDir, "files.tar.gz")
  		// tar cvfz files.tar.gz ./testdata ./media
  		cmd := exec.Command("tar", "-czf", filesTar, "./testdata", "./media")
  		if output, err := cmd.CombinedOutput(); err != nil {
  			// It's fine if directories don't exist yet, we create dummies or proceed
  			_ = os.MkdirAll("./testdata", 0755)
  			_ = os.MkdirAll("./media", 0755)
  			cmd2 := exec.Command("tar", "-czf", filesTar, "./testdata", "./media")
  			if output2, err := cmd2.CombinedOutput(); err != nil {
  				http.Error(w, fmt.Sprintf("files backup failed: %v, output: %s", err, string(output2)), http.StatusInternalServerError)
  				return
  			}
  		}
  	}

  	// 3. Compress the gathered data into the final archive
  	tarCmd := exec.Command("tar", "-czf", archivePath, "-C", tempDir, ".")
  	if output, err := tarCmd.CombinedOutput(); err != nil {
  		http.Error(w, fmt.Sprintf("failed to pack archive: %v, output: %s", err, string(output)), http.StatusInternalServerError)
  		return
  	}

  	stat, err := os.Stat(archivePath)
  	if err != nil {
  		http.Error(w, "failed to get archive file stats", http.StatusInternalServerError)
  		return
  	}

  	// Write metadata JSON
  	meta := BackupMeta{
  		Filename:  archiveName,
  		Type:      req.Type,
  		CreatedAt: time.Now(),
  		Size:      stat.Size(),
  		Version:   "51",
  		CreatedBy: claims.Username,
  	}
  	metaPath := filepath.Join(h.backupDir, baseName+".json")
  	metaData, _ := json.MarshalIndent(meta, "", "  ")
  	_ = os.WriteFile(metaPath, metaData, 0644)

  	respondJSON(w, http.StatusCreated, map[string]string{"status": "created", "filename": archiveName})
  }
  ```

- [ ] **Step 5: Add Restore (Restoration logic) implementation**
  Add restoration to `internal/api/handler/admin_backup.go`.

  ```go
  func (h *AdminBackupHandler) Restore(w http.ResponseWriter, r *http.Request) {
  	claims := middleware.GetUserClaims(r)
  	if claims == nil {
  		http.Error(w, "unauthorized", http.StatusUnauthorized)
  		return
  	}

  	filename := chi.URLParam(r, "filename")
  	if filename == "" {
  		http.Error(w, "filename is required", http.StatusBadRequest)
  		return
  	}

  	var req struct {
  		Password string `json:"password"`
  	}
  	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
  		http.Error(w, "invalid body", http.StatusBadRequest)
  		return
  	}

  	// Verify admin password
  	user, err := h.userStore.GetByID(r.Context(), claims.UserID)
  	if err != nil || user == nil {
  		http.Error(w, "admin user not found", http.StatusUnauthorized)
  		return
  	}

  	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
  		http.Error(w, "incorrect password", http.StatusUnauthorized)
  		return
  	}

  	cleanName := filepath.Base(filename)
  	archivePath := filepath.Join(h.backupDir, cleanName)

  	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
  		http.Error(w, "backup file not found", http.StatusNotFound)
  		return
  	}

  	// Create temp directory for extraction
  	tempDir, err := os.MkdirTemp("", "aioj-restore-*")
  	if err != nil {
  		http.Error(w, "failed to create temp directory", http.StatusInternalServerError)
  		return
  	}
  	defer os.RemoveAll(tempDir)

  	// Extract final archive
  	extractCmd := exec.Command("tar", "-xzf", archivePath, "-C", tempDir)
  	if output, err := extractCmd.CombinedOutput(); err != nil {
  		http.Error(w, fmt.Sprintf("failed to extract archive: %v, output: %s", err, string(output)), http.StatusInternalServerError)
  		return
  	}

  	// Restore Database (if present)
  	sqlFile := filepath.Join(tempDir, "db.sql")
  	if _, err := os.Stat(sqlFile); err == nil {
  		// Run restoring script using psql with clean flags
  		cmd := exec.Command("psql", "-h", h.dbConfig.Host, "-p", fmt.Sprintf("%d", h.dbConfig.Port),
  			"-U", h.dbConfig.User, "-d", h.dbConfig.Name, "-f", sqlFile)
  		cmd.Env = append(os.Environ(), "PGPASSWORD="+h.dbConfig.Password)
  		if output, err := cmd.CombinedOutput(); err != nil {
  			http.Error(w, fmt.Sprintf("database restore failed: %v, output: %s", err, string(output)), http.StatusInternalServerError)
  			return
  		}
  	}

  	// Restore Files (if present)
  	filesTar := filepath.Join(tempDir, "files.tar.gz")
  	if _, err := os.Stat(filesTar); err == nil {
  		// Clean target directories first to prevent leftovers
  		_ = os.RemoveAll("./testdata")
  		_ = os.RemoveAll("./media")
  		_ = os.MkdirAll("./testdata", 0755)
  		_ = os.MkdirAll("./media", 0755)

  		cmd := exec.Command("tar", "-xzf", filesTar, "-C", ".")
  		if output, err := cmd.CombinedOutput(); err != nil {
  			http.Error(w, fmt.Sprintf("files restore failed: %v, output: %s", err, string(output)), http.StatusInternalServerError)
  			return
  		}
  	}

  	respondJSON(w, http.StatusOK, map[string]string{"status": "restored"})
  }
  ```

- [ ] **Step 6: Run go compiler syntax check to verify compile success**
  Run: `go build ./internal/api/handler/...`
  Expected: Command returns with exit code 0.

- [ ] **Step 7: Commit**
  Run: `git add internal/api/handler/admin_backup.go && git commit -m "feat: implement AdminBackupHandler in Go backend"`

---

### Task 3: Create Go Backend Unit Tests

**Files:**
- Create: `internal/api/handler/admin_backup_test.go`

- [ ] **Step 1: Write mock stores and backup handler unit tests**
  Create `/Users/tahsinarafat/App_Dev/AIOJ/internal/api/handler/admin_backup_test.go` to test access controls and methods.

  ```go
  package handler

  import (
  	"bytes"
  	"context"
  	"encoding/json"
  	"net/http"
  	"net/http/httptest"
  	"os"
  	"path/filepath"
  	"testing"

  	"github.com/go-chi/chi/v5"
  	"github.com/tahsinarafat/aioj/internal/api/middleware"
  	"github.com/tahsinarafat/aioj/internal/auth"
  	"github.com/tahsinarafat/aioj/internal/config"
  	"github.com/tahsinarafat/aioj/internal/model"
  )

  type mockUserStore struct {
  	user *model.User
  	err  error
  }

  func (m *mockUserStore) Create(ctx context.Context, user *model.User) error { return nil }
  func (m *mockUserStore) GetByID(ctx context.Context, id string) (*model.User, error) {
  	return m.user, m.err
  }
  func (m *mockUserStore) GetByUsername(ctx context.Context, username string) (*model.User, error) {
  	return m.user, m.err
  }
  func (m *mockUserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
  	return nil, nil
  }
  func (m *mockUserStore) GetPublicProfile(ctx context.Context, username string) (*model.PublicProfile, error) {
  	return nil, nil
  }
  func (m *mockUserStore) GetProfile(ctx context.Context, userID string) (*model.UserProfile, error) {
  	return nil, nil
  }
  func (m *mockUserStore) UpdateProfile(ctx context.Context, userID string, p *model.UserProfile) error {
  	return nil
  }
  func (m *mockUserStore) ListUsers(ctx context.Context, offset, limit int) ([]model.User, int, error) {
  	return nil, 0, nil
  }
  func (m *mockUserStore) UpdateRole(ctx context.Context, id, role string) error { return nil }
  func (m *mockUserStore) UpdatePassword(ctx context.Context, id, passwordHash string) error {
  	return nil
  }
  func (m *mockUserStore) UpdateRating(ctx context.Context, userID string, rating, maxRating, contestCount int) error {
  	return nil
  }

  func TestAdminBackupHandler_List(t *testing.T) {
  	tempDir, err := os.MkdirTemp("", "aioj-backup-test-*")
  	if err != nil {
  		t.Fatal(err)
  	}
  	defer os.RemoveAll(tempDir)

  	// Create a dummy metadata and backup file
  	archiveFile := filepath.Join(tempDir, "aioj_backup_db_test.tar.gz")
  	_ = os.WriteFile(archiveFile, []byte("dummy tar"), 0644)

  	metaFile := filepath.Join(tempDir, "aioj_backup_db_test.json")
  	meta := BackupMeta{
  		Filename: "aioj_backup_db_test.tar.gz",
  		Type:     "db",
  		Size:     9,
  	}
  	metaBytes, _ := json.Marshal(meta)
  	_ = os.WriteFile(metaFile, metaBytes, 0644)

  	h := NewAdminBackupHandler(&mockUserStore{}, config.DatabaseConfig{}, tempDir)

  	req := httptest.NewRequest("GET", "/api/admin/backups", nil)
  	rec := httptest.NewRecorder()

  	h.List(rec, req)

  	if rec.Code != http.StatusOK {
  		t.Errorf("expected status 200, got %d", rec.Code)
  	}

  	var resp struct {
  		Data []BackupMeta `json:"data"`
  	}
  	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
  		t.Fatal(err)
  	}

  	if len(resp.Data) != 1 {
  		t.Fatalf("expected 1 backup, got %d", len(resp.Data))
  	}

  	if resp.Data[0].Filename != "aioj_backup_db_test.tar.gz" {
  		t.Errorf("expected filename 'aioj_backup_db_test.tar.gz', got '%s'", resp.Data[0].Filename)
  	}
  }

  func TestAdminBackupHandler_Delete(t *testing.T) {
  	tempDir, err := os.MkdirTemp("", "aioj-backup-test-*")
  	if err != nil {
  		t.Fatal(err)
  	}
  	defer os.RemoveAll(tempDir)

  	archiveFile := filepath.Join(tempDir, "aioj_backup_db_test.tar.gz")
  	_ = os.WriteFile(archiveFile, []byte("dummy tar"), 0644)

  	h := NewAdminBackupHandler(&mockUserStore{}, config.DatabaseConfig{}, tempDir)

  	rctx := chi.NewRouteContext()
  	rctx.URLParams.Add("filename", "aioj_backup_db_test.tar.gz")

  	req := httptest.NewRequest("DELETE", "/api/admin/backups/aioj_backup_db_test.tar.gz", nil)
  	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
  	rec := httptest.NewRecorder()

  	h.Delete(rec, req)

  	if rec.Code != http.StatusOK {
  		t.Errorf("expected status 200, got %d", rec.Code)
  	}

  	if _, err := os.Stat(archiveFile); !os.IsNotExist(err) {
  		t.Errorf("expected file to be deleted")
  	}
  }
  ```

- [ ] **Step 2: Run tests to verify they pass**
  Run: `go test -v ./internal/api/handler -run TestAdminBackupHandler`
  Expected: PASS

- [ ] **Step 3: Commit**
  Run: `git add internal/api/handler/admin_backup_test.go && git commit -m "test: add unit tests for AdminBackupHandler"`

---

### Task 4: Register Handler, Router, and main Dependency Injection

**Files:**
- Modify: `internal/api/deps.go`
- Modify: `internal/api/router.go`
- Modify: `cmd/aioj/main.go`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Add handler field to Dependency injection list**
  Modify `/Users/tahsinarafat/App_Dev/AIOJ/internal/api/deps.go` line 52 to include the new handler:
  ```go
  type Deps struct {
  	// ... existing fields ...
  	AdminSub       *handler.AdminSubmissionHandler
  	Backup         *handler.AdminBackupHandler // <-- ADD THIS
  }
  ```

- [ ] **Step 2: Define route map inside router setup**
  Modify `/Users/tahsinarafat/App_Dev/AIOJ/internal/api/router.go` inside `api.NewRouter` to map endpoints.
  Add backupH declaration around line 55:
  ```go
  	adminSubH := d.AdminSub
  	backupH := d.Backup // <-- ADD THIS
  ```
  Add routes under `r.Route("/api/admin", func(r chi.Router) { ... })` around line 245:
  ```go
  		r.Route("/backups", func(r chi.Router) {
  			r.Get("/", backupH.List)
  			r.Post("/", backupH.Create)
  			r.Post("/upload", backupH.Upload)
  			r.Get("/{filename}", backupH.Download)
  			r.Delete("/{filename}", backupH.Delete)
  			r.Post("/{filename}/restore", backupH.Restore)
  		})
  ```

- [ ] **Step 3: Wire up backup handler inside server main.go**
  Modify `/Users/tahsinarafat/App_Dev/AIOJ/cmd/aioj/main.go` around line 222:
  ```go
  	remoteLangH := handler.NewRemoteLanguageHandler(remoteLangStore, vjService)
  	adminSubH := handler.NewAdminSubmissionHandler(submissionStore, problemStore, vjService)
  	backupH := handler.NewAdminBackupHandler(userStore, cfg.Database, "./backups") // <-- ADD THIS
  ```
  And add it to `api.Deps` mapping around line 265:
  ```go
  		LangAdmin:      langAdminH,
  		RemoteLang:     remoteLangH,
  		AdminSub:       adminSubH,
  		Backup:         backupH, // <-- ADD THIS
  	}, jwtManager)
  ```

- [ ] **Step 4: Mount backup folder volume mapping in docker-compose.yml**
  Modify `docker-compose.yml` to support persistence.
  Under the `backend` service volumes mapping, add:
  ```yaml
      volumes:
        - testdata:/app/testdata
        - ./lang:/app/lang
        - backups:/app/backups # <-- ADD THIS
  ```
  And under the root level `volumes` mapping block, add:
  ```yaml
  volumes:
    pgdata:
    redisdata:
    testdata:
    promdata:
    grafdata:
    caddy_data:
    caddy_config:
    backups: # <-- ADD THIS
  ```

- [ ] **Step 5: Run compiler to check server compiles fine**
  Run: `go build ./cmd/aioj`
  Expected: Command compiles successfully with exit code 0.

- [ ] **Step 6: Commit**
  Run: `git add internal/api/deps.go internal/api/router.go cmd/aioj/main.go docker-compose.yml && git commit -m "feat: wire up backup routes and configure backups volume mount"`

---

### Task 5: Update Frontend HTTP API Client (TypeScript)

**Files:**
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Allow FormData file uploads in web request client**
  Modify `/Users/tahsinarafat/App_Dev/AIOJ/web/src/lib/api.ts` around line 66:
  ```typescript
  async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
      const headers: Record<string, string> = {
          ...(opts.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
          ...(opts.headers as Record<string, string> || {}),
      }
  ```

- [ ] **Step 2: Add backups methods in admin object of api namespace**
  Modify `/Users/tahsinarafat/App_Dev/AIOJ/web/src/lib/api.ts` around line 360 (inside `admin: { ... }`):
  ```typescript
          backups: {
              list: () =>
                  request<{ data: any[] }>('/admin/backups'),
              create: (type: string) =>
                  request<any>('/admin/backups', { method: 'POST', body: JSON.stringify({ type }) }),
              delete: (filename: string) =>
                  request<any>(`/admin/backups/${encodeURIComponent(filename)}`, { method: 'DELETE' }),
              restore: (filename: string, password: string) =>
                  request<any>(`/admin/backups/${encodeURIComponent(filename)}/restore`, { method: 'POST', body: JSON.stringify({ password }) }),
              upload: (file: File) => {
                  const formData = new FormData()
                  formData.append('file', file)
                  return request<any>('/admin/backups/upload', {
                      method: 'POST',
                      body: formData,
                  })
              },
          },
  ```

- [ ] **Step 3: Run typescript check in web to verify compilation**
  Run: `npm run tsc` in `web/` or check build status.
  Expected: TSC check passes with no errors.

- [ ] **Step 4: Commit**
  Run: `git add web/src/lib/api.ts && git commit -m "feat: add backups endpoints to frontend api client"`

---

### Task 6: Build Backups Admin Dashboard Panel (React Component)

**Files:**
- Create: `web/src/pages/admin/BackupsPanel.tsx`

- [ ] **Step 1: Create BackupsPanel layout with upload, list, generate, and restore warning dialog**
  Create the file `/Users/tahsinarafat/App_Dev/AIOJ/web/src/pages/admin/BackupsPanel.tsx`.

  ```tsx
  import React, { useEffect, useState } from 'react'
  import { api } from '../../lib/api'
  import { Database, Download, Trash2, RotateCcw, Upload, Play, RefreshCw, AlertTriangle } from 'lucide-react'

  interface Backup {
      filename: string
      type: 'db' | 'files' | 'full'
      created_at: string
      size: number
      version: string
      created_by: string
  }

  export default function BackupsPanel() {
      const [backups, setBackups] = useState<Backup[]>([])
      const [loading, setLoading] = useState(true)
      const [creating, setCreating] = useState(false)
      const [uploading, setUploading] = useState(false)
      const [backupType, setBackupType] = useState<'db' | 'files' | 'full'>('db')
      const [restoreTarget, setRestoreTarget] = useState<string | null>(null)
      const [password, setPassword] = useState('')
      const [restoring, setRestoring] = useState(false)
      const [error, setError] = useState('')

      const fetchBackups = () => {
          setLoading(true)
          api.admin.backups.list()
              .then(d => setBackups(d.data || []))
              .catch(err => setError(err.message || 'Failed to fetch backups'))
              .finally(() => setLoading(false))
      }

      useEffect(() => {
          fetchBackups()
      }, [])

      const handleCreate = async () => {
          setCreating(true)
          setError('')
          try {
              await api.admin.backups.create(backupType)
              fetchBackups()
          } catch (err: any) {
              setError(err.message || 'Failed to trigger backup creation')
          } finally {
              setCreating(false)
          }
      }

      const handleDelete = async (filename: string) => {
          if (!confirm('Are you sure you want to delete this backup file off the server?')) return
          setError('')
          try {
              await api.admin.backups.delete(filename)
              fetchBackups()
          } catch (err: any) {
              setError(err.message || 'Failed to delete backup')
          }
      }

      const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
          const file = e.target.files?.[0]
          if (!file) return
          setUploading(true)
          setError('')
          try {
              await api.admin.backups.upload(file)
              fetchBackups()
          } catch (err: any) {
              setError(err.message || 'Failed to upload backup file')
          } finally {
              setUploading(false)
          }
      }

      const handleRestore = async () => {
          if (!restoreTarget) return
          if (!password) {
              alert('Please enter your account password to confirm.')
              return
          }
          setRestoring(true)
          setError('')
          try {
              await api.admin.backups.restore(restoreTarget, password)
              alert('System successfully restored!')
              setRestoreTarget(null)
              setPassword('')
              fetchBackups()
          } catch (err: any) {
              setError(err.message || 'Restoration failed. Verify password is correct.')
          } finally {
              setRestoring(false)
          }
      }

      const formatSize = (bytes: number) => {
          if (bytes < 1024) return bytes + ' B'
          if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
          return (bytes / 1048576).toFixed(1) + ' MB'
      }

      return (
          <div className="space-y-6">
              <div className="flex items-center justify-between mb-2">
                  <div>
                      <h2 className="text-lg font-semibold">Backup & Restore</h2>
                      <p className="text-sm text-gray-500 mt-1">
                          Create and restore database dumps, system file uploads, and test data cases
                      </p>
                  </div>
                  <button onClick={fetchBackups} className="flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100">
                      <RefreshCw className="w-4 h-4" /> Refresh
                  </button>
              </div>

              {error && (
                  <div className="bg-red-50 dark:bg-red-950/20 text-red-700 dark:text-red-300 p-3 rounded-lg text-sm">
                      {error}
                  </div>
              )}

              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  {/* Generate Backup */}
                  <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-5">
                      <h3 className="text-sm font-semibold mb-4 flex items-center gap-2">
                          <Play className="w-4 h-4 text-blue-600" /> Generate Backup Archive
                      </h3>
                      <div className="space-y-4">
                          <div>
                              <label className="block text-xs text-gray-500 mb-1.5">Backup Scope</label>
                              <div className="flex flex-col gap-2">
                                  <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                                      <input type="radio" name="btype" checked={backupType === 'db'} onChange={() => setBackupType('db')} />
                                      Database Dump Only (PostgreSQL)
                                  </label>
                                  <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                                      <input type="radio" name="btype" checked={backupType === 'files'} onChange={() => setBackupType('files')} />
                                      Test Cases & Uploaded Files (./testdata & ./media)
                                  </label>
                                  <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                                      <input type="radio" name="btype" checked={backupType === 'full'} onChange={() => setBackupType('full')} />
                                      Full Backup (Database + Files)
                                  </label>
                              </div>
                          </div>
                          <button
                              disabled={creating}
                              onClick={handleCreate}
                              className="w-full bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white py-2 px-4 rounded-lg text-sm font-medium transition-colors"
                          >
                              {creating ? 'Generating Archive...' : 'Create Backup'}
                          </button>
                      </div>
                  </div>

                  {/* Upload Backup */}
                  <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-5">
                      <h3 className="text-sm font-semibold mb-4 flex items-center gap-2">
                          <Upload className="w-4 h-4 text-green-600" /> Upload Backup File
                      </h3>
                      <div className="flex flex-col items-center justify-center border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg p-6 text-center">
                          <Upload className="w-8 h-8 text-gray-400 mb-2" />
                          <p className="text-xs text-gray-500 mb-3">Upload a previously generated .tar.gz backup package</p>
                          <label className="bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-800 dark:text-gray-200 px-4 py-2 rounded-lg text-sm font-medium cursor-pointer transition-colors">
                              {uploading ? 'Uploading...' : 'Choose Archive File'}
                              <input type="file" accept=".tar.gz" className="hidden" disabled={uploading} onChange={handleUpload} />
                          </label>
                      </div>
                  </div>
              </div>

              {/* Backup List Table */}
              <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                  <div className="px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
                      <h3 className="text-sm font-semibold">Saved Archives</h3>
                      <span className="text-xs text-gray-400">{backups.length} file(s) available</span>
                  </div>

                  {loading ? (
                      <div className="text-center py-8 text-gray-400">Loading archives...</div>
                  ) : backups.length === 0 ? (
                      <div className="text-center py-8 text-gray-400">No backup files found on the server.</div>
                  ) : (
                      <div className="overflow-x-auto">
                          <table className="w-full text-left border-collapse text-sm">
                              <thead>
                                  <tr className="bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700 text-gray-500">
                                      <th className="px-5 py-3 font-semibold">Name</th>
                                      <th className="px-5 py-3 font-semibold">Scope</th>
                                      <th className="px-5 py-3 font-semibold">Size</th>
                                      <th className="px-5 py-3 font-semibold">Created At</th>
                                      <th className="px-5 py-3 font-semibold text-right">Actions</th>
                                  </tr>
                              </thead>
                              <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                                  {backups.map(b => (
                                      <tr key={b.filename} className="hover:bg-gray-50 dark:hover:bg-gray-800/40">
                                          <td className="px-5 py-3 font-mono text-xs">{b.filename}</td>
                                          <td className="px-5 py-3">
                                              <span className={`px-2 py-0.5 rounded text-xs font-medium uppercase ${
                                                  b.type === 'db' ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300' :
                                                  b.type === 'files' ? 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-300' :
                                                  'bg-purple-50 text-purple-700 dark:bg-purple-900/20 dark:text-purple-300'
                                              }`}>
                                                  {b.type === 'full' ? 'Combined' : b.type === 'db' ? 'Database' : 'Files'}
                                              </span>
                                          </td>
                                          <td className="px-5 py-3">{formatSize(b.size)}</td>
                                          <td className="px-5 py-3 text-xs text-gray-500">
                                              {new Date(b.created_at).toLocaleString()}
                                          </td>
                                          <td className="px-5 py-3 text-right">
                                              <div className="inline-flex gap-2">
                                                  <a
                                                      href={`/api/admin/backups/${encodeURIComponent(b.filename)}`}
                                                      download
                                                      className="p-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 rounded text-gray-600 dark:text-gray-400"
                                                      title="Download File"
                                                  >
                                                      <Download className="w-4 h-4" />
                                                  </a>
                                                  <button
                                                      onClick={() => setRestoreTarget(b.filename)}
                                                      className="p-1.5 hover:bg-orange-50 dark:hover:bg-orange-950/20 rounded text-orange-600 dark:text-orange-400"
                                                      title="Restore from Backup"
                                                  >
                                                      <RotateCcw className="w-4 h-4" />
                                                  </button>
                                                  <button
                                                      onClick={() => handleDelete(b.filename)}
                                                      className="p-1.5 hover:bg-red-50 dark:hover:bg-red-950/20 rounded text-red-600 dark:text-red-400"
                                                      title="Delete Backup"
                                                  >
                                                      <Trash2 className="w-4 h-4" />
                                                  </button>
                                              </div>
                                          </td>
                                      </tr>
                                  ))}
                              </tbody>
                          </table>
                      </div>
                  )}
              </div>

              {/* Destructive Restore Modal */}
              {restoreTarget && (
                  <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
                      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl max-w-md w-full p-6 space-y-4 relative">
                          <div className="flex items-center gap-3 text-red-600">
                              <AlertTriangle className="w-6 h-6 flex-shrink-0" />
                              <h3 className="text-lg font-bold">Destructive System Restore</h3>
                          </div>
                          <p className="text-sm text-gray-600 dark:text-gray-300">
                              You are about to restore AIOJ using the archive: <strong className="font-mono text-xs">{restoreTarget}</strong>.
                              <br /><br />
                              <span className="text-red-600 dark:text-red-400 font-medium">
                                  WARNING: This action is destructive and will overwrite all current data and configurations. Active sessions and queues will be lost.
                              </span>
                          </p>

                          <div className="space-y-2">
                              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                                  Confirm Admin Password
                              </label>
                              <input
                                  type="password"
                                  placeholder="Enter your password to confirm"
                                  value={password}
                                  onChange={e => setPassword(e.target.value)}
                                  className="w-full border rounded-lg px-3 py-2 text-sm bg-white dark:bg-gray-900 border-gray-300 dark:border-gray-600 focus:outline-none focus:ring-1 focus:ring-red-500"
                              />
                          </div>

                          <div className="flex justify-end gap-3 pt-2">
                              <button
                                  disabled={restoring}
                                  onClick={() => {
                                      setRestoreTarget(null)
                                      setPassword('')
                                  }}
                                  className="px-4 py-2 border rounded-lg text-sm text-gray-600 dark:text-gray-400 border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                              >
                                  Cancel
                              </button>
                              <button
                                  disabled={restoring || !password}
                                  onClick={handleRestore}
                                  className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg text-sm font-medium disabled:opacity-50 transition-colors"
                              >
                                  {restoring ? 'Restoring System...' : 'Confirm Restore'}
                              </button>
                          </div>
                      </div>
                  </div>
              )}
          </div>
      )
  }
  ```

- [ ] **Step 2: Commit**
  Run: `git add web/src/pages/admin/BackupsPanel.tsx && git commit -m "feat: build BackupsPanel component with restore and upload features"`

---

### Task 7: Wire up Backups Panel Tab in Admin Dashboard

**Files:**
- Modify: `web/src/pages/AdminDashboard.tsx`

- [ ] **Step 1: Add backups key to tabs list and display panel component conditionally**
  Modify `/Users/tahsinarafat/App_Dev/AIOJ/web/src/pages/AdminDashboard.tsx`.
  Import `Database` from `lucide-react` on line 2:
  ```typescript
  import { Users, FileText, Bot, Settings, Code, Globe, Send, Database } from 'lucide-react'
  import BackupsPanel from './admin/BackupsPanel' // <-- ADD THIS
  ```
  Add tab key to `AdminTab` and tabs list around line 11-13:
  ```typescript
  type AdminTab = 'users' | 'applications' | 'bots' | 'languages' | 'remote-languages' | 'submissions' | 'settings' | 'backups'

  const tabs: { key: AdminTab; label: string; icon: typeof Users }[] = [
      { key: 'users', label: 'Users', icon: Users },
      { key: 'applications', label: 'Applications', icon: FileText },
      { key: 'bots', label: 'Bot Accounts', icon: Bot },
      { key: 'languages', label: 'Languages', icon: Code },
      { key: 'remote-languages', label: 'Remote Languages', icon: Globe },
      { key: 'submissions', label: 'Remote Subs', icon: Send },
      { key: 'settings', label: 'Settings', icon: Settings },
      { key: 'backups', label: 'Backup & Restore', icon: Database }, // <-- ADD THIS
  ]
  ```
  Add matching condition inside activeTab check around line 26:
  ```typescript
          const validTabs: AdminTab[] = ['users', 'applications', 'bots', 'languages', 'remote-languages', 'submissions', 'settings', 'backups']
  ```
  Render the backups panel around line 67:
  ```typescript
                      {activeTab === 'submissions' && <SubmissionsPanel />}
                      {activeTab === 'settings' && <SystemSettingsPanel />}
                      {activeTab === 'backups' && <BackupsPanel />} {/* <-- ADD THIS */}
  ```

- [ ] **Step 2: Run frontend build check in web directory**
  Run: `npm run build` in `/Users/tahsinarafat/App_Dev/AIOJ/web`
  Expected: React application builds without any TS compilation or styling issues (exit code 0).

- [ ] **Step 3: Commit**
  Run: `git add web/src/pages/AdminDashboard.tsx && git commit -m "feat: add Backup & Restore tab to admin dashboard"`
