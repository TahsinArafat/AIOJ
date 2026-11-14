package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/config"
	"github.com/tahsinarafat/aioj/internal/store"
)

// BackupMeta holds metadata about a backup file.
type BackupMeta struct {
	Filename  string    `json:"filename"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Size      int64     `json:"size"`
	Version   string    `json:"version"`
	CreatedBy string    `json:"created_by"`
}

// AdminBackupHandler manages backup and restore operations.
type AdminBackupHandler struct {
	userStore store.UserReader
	dbCfg     config.DatabaseConfig
	dir       string
}

// NewAdminBackupHandler creates a new AdminBackupHandler.
func NewAdminBackupHandler(u store.UserReader, dbCfg config.DatabaseConfig, dir string) *AdminBackupHandler {
	return &AdminBackupHandler{
		userStore: u,
		dbCfg:     dbCfg,
		dir:       dir,
	}
}

// sanitizeFilename reduces a client-supplied filename to a safe base name,
// rejecting empty names, traversal segments, and path separators so the value
// can never escape the backup directory.
func sanitizeFilename(filename string) (string, error) {
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid filename %q", filename)
	}
	return name, nil
}

// List returns all backup files in the backup directory.
func (h *AdminBackupHandler) List(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(h.dir)
	if err != nil {
		if os.IsNotExist(err) {
			respondJSON(w, http.StatusOK, []BackupMeta{})
			return
		}
		http.Error(w, "failed to read backup directory", http.StatusInternalServerError)
		return
	}

	var backups []BackupMeta
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, BackupMeta{
			Filename:  entry.Name(),
			Type:      guessBackupType(entry.Name()),
			CreatedAt: info.ModTime(),
			Size:      info.Size(),
			Version:   "1.0",
			CreatedBy: "system",
		})
	}

	if backups == nil {
		backups = []BackupMeta{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": backups})
}

// Delete removes a backup file by filename.
func (h *AdminBackupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	filename, err := sanitizeFilename(chi.URLParam(r, "filename"))
	if err != nil {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	path := filepath.Join(h.dir, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(w, "backup not found", http.StatusNotFound)
		return
	}

	if err := os.Remove(path); err != nil {
		http.Error(w, "failed to delete backup", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Download serves a backup file for download.
func (h *AdminBackupHandler) Download(w http.ResponseWriter, r *http.Request) {
	filename, err := sanitizeFilename(chi.URLParam(r, "filename"))
	if err != nil {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	path := filepath.Join(h.dir, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(w, "backup not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	http.ServeFile(w, r, path)
}

// Upload accepts a multipart file upload and saves it as a backup.
func (h *AdminBackupHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<20)
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if err := os.MkdirAll(h.dir, 0755); err != nil {
		http.Error(w, "failed to create backup directory", http.StatusInternalServerError)
		return
	}

	filename, err := sanitizeFilename(header.Filename)
	if err != nil {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".sql") && !strings.HasSuffix(strings.ToLower(filename), ".tar.gz") {
		http.Error(w, "backup must be a .sql or .tar.gz file", http.StatusBadRequest)
		return
	}

	dstPath := filepath.Join(h.dir, filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		dst.Close()
		_ = os.Remove(dstPath)
		http.Error(w, "failed to write file", http.StatusInternalServerError)
		return
	}
	if written == 0 {
		dst.Close()
		_ = os.Remove(dstPath)
		http.Error(w, "backup file is empty", http.StatusBadRequest)
		return
	}

	fi, err := dst.Stat()
	if err != nil {
		http.Error(w, "failed to stat saved file", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, BackupMeta{
		Filename:  filename,
		Type:      guessBackupType(filename),
		CreatedAt: fi.ModTime(),
		Size:      fi.Size(),
		Version:   "1.0",
		CreatedBy: "upload",
	})
}

// Create creates a new backup. Valid types: "db", "files", "full".
func (h *AdminBackupHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type     string `json:"type"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	validTypes := map[string]bool{"db": true, "files": true, "full": true}
	if !validTypes[req.Type] {
		http.Error(w, "invalid type: must be db, files, or full", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(h.dir, 0755); err != nil {
		http.Error(w, "failed to create backup directory", http.StatusInternalServerError)
		return
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("backup_%s_%s.tar.gz", req.Type, timestamp)

	switch req.Type {
	case "db":
		filename = fmt.Sprintf("backup_db_%s.sql", timestamp)
		if err := h.runPgDump(filename); err != nil {
			http.Error(w, fmt.Sprintf("database backup failed: %v", err), http.StatusInternalServerError)
			return
		}
	case "files":
		if err := h.runTarCreate(filename, "./testdata", "./media"); err != nil {
			http.Error(w, fmt.Sprintf("files backup failed: %v", err), http.StatusInternalServerError)
			return
		}
	case "full":
		// Create database dump first
		dbFilename := fmt.Sprintf("backup_db_%s.sql", timestamp)
		if err := h.runPgDump(dbFilename); err != nil {
			http.Error(w, fmt.Sprintf("database backup failed: %v", err), http.StatusInternalServerError)
			return
		}
		// Create tar.gz archive containing the db dump, testdata, and media
		dbPath := filepath.Join(h.dir, dbFilename)
		if err := h.runTarCreate(filename, dbPath, "./testdata", "./media"); err != nil {
			os.Remove(dbPath)
			http.Error(w, fmt.Sprintf("full backup failed: %v", err), http.StatusInternalServerError)
			return
		}
		os.Remove(dbPath)
	}

	path := filepath.Join(h.dir, filename)
	fi, err := os.Stat(path)
	if err != nil {
		http.Error(w, "backup file not found after creation", http.StatusInternalServerError)
		return
	}

	claims := middleware.GetUserClaims(r)
	createdBy := "admin"
	if claims != nil {
		createdBy = claims.UserID
	}

	respondJSON(w, http.StatusCreated, BackupMeta{
		Filename:  filename,
		Type:      req.Type,
		CreatedAt: fi.ModTime(),
		Size:      fi.Size(),
		Version:   "1.0",
		CreatedBy: createdBy,
	})
}

// Restore restores from a backup file. Requires password confirmation.
func (h *AdminBackupHandler) Restore(w http.ResponseWriter, r *http.Request) {
	filename, err := sanitizeFilename(chi.URLParam(r, "filename"))
	if err != nil {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	var req struct {
		Password string `json:"password"`
		Type     string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Verify admin password
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.userStore.GetByID(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "failed to verify user", http.StatusInternalServerError)
		return
	}
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		http.Error(w, "invalid password", http.StatusUnauthorized)
		return
	}

	validTypes := map[string]bool{"db": true, "files": true, "full": true}
	if !validTypes[req.Type] {
		http.Error(w, "invalid type: must be db, files, or full", http.StatusBadRequest)
		return
	}

	path := filepath.Join(h.dir, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(w, "backup not found", http.StatusNotFound)
		return
	}

	tmpDir, err := os.MkdirTemp("", "aioj-restore-*")
	if err != nil {
		http.Error(w, "failed to create temp directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	if strings.HasSuffix(strings.ToLower(filename), ".tar.gz") {
		if err := h.runTarExtract(path, tmpDir); err != nil {
			http.Error(w, fmt.Sprintf("restore extract failed: %v", err), http.StatusBadRequest)
			return
		}
	} else if req.Type == "db" && strings.HasSuffix(strings.ToLower(filename), ".sql") {
		if err := copyFile(path, filepath.Join(tmpDir, filename)); err != nil {
			http.Error(w, "failed to prepare database dump", 500)
			return
		}
	} else {
		http.Error(w, "backup file format does not match restore type", http.StatusBadRequest)
		return
	}

	switch req.Type {
	case "db":
		var sqlFile string
		filepath.Walk(tmpDir, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".sql") {
				sqlFile = p
			}
			return nil
		})

		if sqlFile == "" {
			http.Error(w, "no database dump found in backup archive", http.StatusBadRequest)
			return
		}

		if err := h.runPsql(sqlFile); err != nil {
			http.Error(w, fmt.Sprintf("database restore failed: %v", err), http.StatusInternalServerError)
			return
		}
	case "files":
		extractedTestData := filepath.Join(tmpDir, "testdata")
		extractedMedia := filepath.Join(tmpDir, "media")
		if err := restoreDirectories(map[string]string{extractedTestData: "./testdata", extractedMedia: "./media"}); err != nil {
			http.Error(w, fmt.Sprintf("files restore failed: %v", err), 500)
			return
		}
	case "full":
		var sqlFile string
		filepath.Walk(tmpDir, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".sql") {
				sqlFile = p
			}
			return nil
		})

		if sqlFile == "" {
			http.Error(w, "no database dump found in backup archive", http.StatusBadRequest)
			return
		}

		if err := h.runPsql(sqlFile); err != nil {
			http.Error(w, fmt.Sprintf("database restore failed: %v", err), http.StatusInternalServerError)
			return
		}

		extractedTestData := filepath.Join(tmpDir, "testdata")
		extractedMedia := filepath.Join(tmpDir, "media")
		if err := restoreDirectories(map[string]string{extractedTestData: "./testdata", extractedMedia: "./media"}); err != nil {
			http.Error(w, fmt.Sprintf("files restore failed: %v", err), 500)
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

// runPgDump executes pg_dump to create a database SQL dump.
func (h *AdminBackupHandler) runPgDump(filename string) error {
	dstPath := filepath.Join(h.dir, filename)

	cmd := exec.Command("pg_dump",
		"-h", h.dbCfg.Host,
		"-p", fmt.Sprintf("%d", h.dbCfg.Port),
		"-U", h.dbCfg.User,
		"-d", h.dbCfg.Name,
		"--clean",
		"--if-exists",
		"--no-owner",
		"--no-privileges",
		"-f", dstPath,
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", h.dbCfg.Password))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_dump failed: %v, output: %s", err, string(output))
	}
	return nil
}

// runPsql executes psql to restore a database from a SQL dump.
func (h *AdminBackupHandler) runPsql(sqlFile string) error {
	cmd := exec.Command("psql",
		"-h", h.dbCfg.Host,
		"-p", fmt.Sprintf("%d", h.dbCfg.Port),
		"-U", h.dbCfg.User,
		"-d", h.dbCfg.Name,
		"--set", "ON_ERROR_STOP=1",
		"--single-transaction",
		"-f", sqlFile,
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", h.dbCfg.Password))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql failed: %v, output: %s", err, string(output))
	}
	return nil
}

// runTarCreate creates a tar.gz archive of the given paths.
func (h *AdminBackupHandler) runTarCreate(filename string, paths ...string) error {
	dstPath := filepath.Join(h.dir, filename)

	existing := []string{}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			existing = append(existing, p)
		}
	}
	if len(existing) == 0 {
		return fmt.Errorf("no backup source paths exist")
	}
	args := append([]string{"-czf", dstPath, "--"}, existing...)
	cmd := exec.Command("tar", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar failed: %v, output: %s", err, string(output))
	}
	return nil
}

// runTarExtract extracts a tar.gz archive to the destination directory.
func (h *AdminBackupHandler) runTarExtract(archivePath, destDir string) error {
	if err := validateTarArchive(archivePath); err != nil {
		return err
	}
	cmd := exec.Command("tar", "-xzf", archivePath, "-C", destDir, "--no-same-owner", "--no-same-permissions")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar extract failed: %v, output: %s", err, string(output))
	}
	return nil
}

// guessBackupType infers backup type from filename.
func guessBackupType(filename string) string {
	if strings.HasSuffix(filename, ".sql") || strings.Contains(filename, "_db_") {
		return "db"
	}
	if strings.Contains(filename, "_files_") {
		return "files"
	}
	return "full"
}

// copyDir copies a directory tree from src to dst, preferring an atomic rename
// when possible and falling back to a recursive copy via cp -r.
func copyDir(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	cmd := exec.Command("cp", "-r", src, dst)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("copy %s -> %s failed: %w, output: %s", src, dst, err, string(output))
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func validateTarArchive(path string) error {
	out, err := exec.Command("tar", "-tzf", path).Output()
	if err != nil {
		return fmt.Errorf("invalid tar archive")
	}
	if len(out) > 8<<20 {
		return fmt.Errorf("archive manifest is too large")
	}
	for _, entry := range strings.Split(string(out), "\n") {
		if entry == "" {
			continue
		}
		clean := filepath.Clean(entry)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("archive contains unsafe path %q", entry)
		}
	}
	return nil
}

// restoreDirectories stages every source beside its destination, then swaps
// them in. Existing data is retained until all staging copies succeed.
func restoreDirectories(paths map[string]string) error {
	type swap struct{ stage, dst, old string }
	var swaps []swap
	for src, dst := range paths {
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		stage := dst + ".restore-new"
		old := dst + ".restore-old"
		_ = os.RemoveAll(stage)
		_ = os.RemoveAll(old)
		if err := copyDir(src, stage); err != nil {
			return err
		}
		swaps = append(swaps, swap{stage, dst, old})
	}
	if len(swaps) == 0 {
		return fmt.Errorf("archive contains no testdata or media directory")
	}
	for i, s := range swaps {
		if _, err := os.Stat(s.dst); err == nil {
			if err := os.Rename(s.dst, s.old); err != nil {
				return err
			}
		}
		if err := os.Rename(s.stage, s.dst); err != nil {
			if _, oldErr := os.Stat(s.old); oldErr == nil {
				_ = os.Rename(s.old, s.dst)
			}
			for _, done := range swaps[:i] {
				_ = os.RemoveAll(done.dst)
				_ = os.Rename(done.old, done.dst)
			}
			return err
		}
	}
	for _, s := range swaps {
		_ = os.RemoveAll(s.old)
	}
	return nil
}
