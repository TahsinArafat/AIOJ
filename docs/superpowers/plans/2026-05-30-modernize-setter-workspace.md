# AIOJ Polygon Workspace: UI Overhaul & Feature Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Overhaul AIOJ's problem setting experience to achieve feature parity with Codeforces Polygon:
1. Rewrite all `/setter` routes (`/setter`, `/setter/create`, `/setter/:slug`, `/setter/contest/create`) into a premium, responsive, developer-grade workspace with analytics, dynamic card grids, live markdown previewers, and split views.
2. Fix all critical/high swallowed database and filesystem errors inside problem handlers.
3. Secure the ZIP extraction logic from decompression bombs (Zip Bombs) and traversal.
4. Build an **Automated Testcase Pair Matching and Score Generator** on ZIP upload (scanning directory contents, matching `.in`/`.out` files, and automatically populating/persisting points to the problem's database record), removing the painful manual typing bottleneck.

**Architecture:**
- **Backend (Go)**:
  - Enhance `internal/api/handler/problem.go` to log and return HTTP 500s on all failed DB updates/deletions.
  - Upgrade `internal/fps/zip.go` with `io.LimitReader` (10MB limit per file) and a maximum of 500 files.
  - Enhance `internal/api/handler/testcase.go` to scan, pair inputs/outputs automatically, create default `TestCaseScore` structures, and persist them.
- **Frontend (React)**:
  - Redesign `SetterPanel.tsx`, `ProblemCreate.tsx`, `SetterProblemWorkspace.tsx`, and `ContestCreate.tsx` with high-fidelity Tailwind components and SVG icons.

**Tech Stack:** React 19, TypeScript, React Router v6, Tailwind CSS, Go (chi router), PostgreSQL 18.

---

### Task 1: Fix Swallowed Backend Errors & Secure Zip Extraction

**Files:**
- Modify: `internal/api/handler/problem.go`
- Modify: `internal/fps/zip.go`

- [ ] **Step 1: Fix ignored store/system errors in problem.go**

Modify `internal/api/handler/problem.go` to check and return errors from `Update`, `Delete`, `AddPermission`, and `RemovePermission`:

```go
// In Update handler:
err = h.store.Update(r.Context(), p.ID, p)
if err != nil {
    http.Error(w, "failed to update problem: "+err.Error(), http.StatusInternalServerError)
    return
}
w.WriteHeader(http.StatusOK)

// In Delete handler:
if err := os.RemoveAll(p.TestdataPath); err != nil {
    // Log error, but proceed or optionally return error
}
err = h.store.Delete(r.Context(), p.ID)
if err != nil {
    http.Error(w, "failed to delete problem: "+err.Error(), http.StatusInternalServerError)
    return
}
w.WriteHeader(http.StatusNoContent)

// In AddPermission handler:
err = h.store.AddPermission(r.Context(), p.ID, req.UserID, req.Level)
if err != nil {
    http.Error(w, "failed to add permission: "+err.Error(), http.StatusInternalServerError)
    return
}
w.WriteHeader(http.StatusOK)

// In RemovePermission handler:
err = h.store.RemovePermission(r.Context(), p.ID, targetUserID)
if err != nil {
    http.Error(w, "failed to remove permission: "+err.Error(), http.StatusInternalServerError)
    return
}
w.WriteHeader(http.StatusOK)
```

- [ ] **Step 2: Secure Zip extraction logic against decompression bombs**

Modify `internal/fps/zip.go` to add entry limit, entry uncompressed size cap, and symlink protection:

```go
func ExtractZip(zipData []byte, targetDir string) (xmlContent []byte, err error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("failed to read zip archive: %w", err)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	const maxFiles = 500
	const maxFileSize = 10 * 1024 * 1024 // 10MB per file max
	
	if len(reader.File) > maxFiles {
		return nil, fmt.Errorf("archive contains too many files (maximum %d)", maxFiles)
	}

	for _, file := range reader.File {
		name := filepath.Base(file.Name)

		if strings.Contains(file.Name, "..") {
			continue
		}

		if file.FileInfo().IsDir() {
			continue
		}
		
		// Skip symlinks
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open entry %s: %w", file.Name, err)
		}

		// Protect against decompression bomb via LimitReader
		limitedReader := io.LimitReader(rc, maxFileSize)
		data, err := io.ReadAll(limitedReader)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read entry %s: %w", file.Name, err)
		}

		if name == "problem.xml" || name == "fps.xml" {
			xmlContent = data
			continue
		}

		destPath := filepath.Join(targetDir, name)
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", name, err)
		}
	}

	return xmlContent, nil
}
```

- [ ] **Step 3: Run Go tests to verify ZIP extraction integrity**

Run: `go test -v ./internal/fps/...`
Expected: PASS

---

### Task 2: Automated Testcase Pair Scanner & Persistence

**Files:**
- Modify: `internal/api/handler/testcase.go`

- [ ] **Step 1: Implement automatic pairing and DB update in Upload handler**

In `internal/api/handler/testcase.go`, add an auto-discover scanner. When a ZIP is extracted successfully, automatically pair inputs/outputs, allocate a default 10-point score per testcase, and update `p.TestCaseScore` in the database!

Modify the `Upload` handler:

```go
package handler

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/fps"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type TestcaseHandler struct {
	store   store.ProblemStore
	dataDir string
}

func NewTestcaseHandler(s store.ProblemStore, dataDir string) *TestcaseHandler {
	return &TestcaseHandler{store: s, dataDir: dataDir}
}

func (h *TestcaseHandler) Upload(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	slug := chi.URLParam(r, "slug")
	p, err := h.store.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), p.ID, claims.UserID, "owner", "co_author") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB max
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		http.Error(w, "no files provided", http.StatusBadRequest)
		return
	}

	probDir := filepath.Join(h.dataDir, p.ID)
	if err := os.MkdirAll(probDir, 0755); err != nil {
		http.Error(w, "failed to create directory", http.StatusInternalServerError)
		return
	}

	zipUploaded := false
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "failed to open file: "+err.Error(), http.StatusBadRequest)
			return
		}

		filename := fileHeader.Filename

		if strings.HasSuffix(strings.ToLower(filename), ".zip") {
			zipUploaded = true
			zipBytes, err := io.ReadAll(file)
			file.Close()
			if err != nil {
				http.Error(w, "failed to read zip file: "+err.Error(), http.StatusBadRequest)
				return
			}

			_, err = fps.ExtractZip(zipBytes, probDir)
			if err != nil {
				http.Error(w, "failed to extract zip: "+err.Error(), http.StatusBadRequest)
				return
			}
		} else {
			err = h.saveFile(probDir, fileHeader)
			file.Close()
			if err != nil {
				http.Error(w, "failed to save file: "+fileHeader.Filename, http.StatusInternalServerError)
				return
			}
		}
	}

	p.TestdataPath = probDir

	// Automated Discovery & Pairing Logic for ZIP uploads
	if zipUploaded {
		discoveredScores := autoDiscoverTestCases(probDir)
		if len(discoveredScores) > 0 {
			p.TestCaseScore = discoveredScores
		}
	}

	if err := h.store.Update(r.Context(), p.ID, p); err != nil {
		http.Error(w, "failed to update problem model: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func autoDiscoverTestCases(targetDir string) []model.TestCaseScore {
	files, err := os.ReadDir(targetDir)
	if err != nil {
		return nil
	}

	inputs := make(map[string]bool)
	outputs := make(map[string]bool)

	inputExts := []string{".in", ".input", ".txt"}
	outputExts := []string{".out", ".output", ".ans", ".sol"}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		lowerName := strings.ToLower(name)

		isInput := false
		for _, ext := range inputExts {
			if strings.HasSuffix(lowerName, ext) {
				inputs[name] = true
				isInput = true
				break
			}
		}

		if !isInput {
			for _, ext := range outputExts {
				if strings.HasSuffix(lowerName, ext) {
					outputs[name] = true
					break
				}
			}
		}
	}

	var testCases []model.TestCaseScore

	for inputName := range inputs {
		var inputPrefix string
		for _, ext := range inputExts {
			if strings.HasSuffix(strings.ToLower(inputName), ext) {
				inputPrefix = inputName[:len(inputName)-len(ext)]
				break
			}
		}

		var matchedOutput string
		for outputName := range outputs {
			for _, ext := range outputExts {
				if strings.HasSuffix(strings.ToLower(outputName), ext) {
					outputPrefix := outputName[:len(outputName)-len(ext)]
					if inputPrefix == outputPrefix {
						matchedOutput = outputName
						break
					}
				}
			}
			if matchedOutput != "" {
				break
			}
		}

		if matchedOutput != "" {
			testCases = append(testCases, model.TestCaseScore{
				InputName:  inputName,
				OutputName: matchedOutput,
				Score:      10, // Default 10 points
			})
			delete(outputs, matchedOutput)
		}
	}

	sort.Slice(testCases, func(i, j int) bool {
		return testCases[i].InputName < testCases[j].InputName
	})

	return testCases
}

func (h *TestcaseHandler) saveFile(dir string, fileHeader *multipart.FileHeader) error {
	filename := fileHeader.Filename
	if strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
		return fmt.Errorf("invalid filename")
	}
	baseName := filepath.Base(filename)

	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dstPath := filepath.Join(dir, baseName)
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(dst, src)
	dst.Close()

	if err != nil {
		os.Remove(dstPath)
	}

	return err
}
```

- [ ] **Step 2: Run Go compile and test command**

Run: `go test -v ./internal/api/handler/...`
Expected: PASS

---

### Task 3: Overhaul Setter Panel Dashboard UI

**Files:**
- Modify: `web/src/pages/SetterPanel.tsx`

- [ ] **Step 1: Upgrade SetterPanel.tsx visual UI**

Replace with modern dark-leaning premium theme:
1. Comprehensive Overview Analytics: Grid of modern card components.
2. Search & Difficulty pills for direct dynamic filtering.
3. Codeforces Import Card with descriptive styling and purple ambient glows.
4. Sleek problem row listings withDifficulty badges, Source badges, and clean custom action buttons.

---

### Task 4: Create a Sleek, Responsive Problem Creation Form

**Files:**
- Modify: `web/src/pages/ProblemCreate.tsx`

- [ ] **Step 1: Overhaul ProblemCreate.tsx**

1. Grid layouts for core fields.
2. **Auto-Slug Generator**: Generate clean URL-safe slugs dynamically in real-time as users type titles.
3. Upgraded select dropdowns and memory limits form fields.
4. Informative step guidelines and clean validations.

---

### Task 5: Overhaul the Massive Polygon Workspace Interface

**Files:**
- Modify: `web/src/pages/SetterProblemWorkspace.tsx`

- [ ] **Step 1: Redesign Tab Switcher & Live Markdown Split Preview**

1. Premium active sidebar tab indicators with clean icon designs.
2. Statement edit page: Dual-pane layout. Left edit textareas, right real-time rendered KaTeX + Markdown preview screen.
3. Clean lists of sample cases with inline delete actions.

- [ ] **Step 2: Redesign Testcases ZIP upload card & automatic pairing layout**

1. Drag-and-drop styled dashed file uploader container.
2. Instant display of automatically matched testcase pairs immediately when ZIP upload finishes successfully. Let problem setters easily tweak default scores inline!

- [ ] **Step 3: Redesign Checker Tab & Special Judge presets**

1. Selector card cards for checker type choices.
2. Special Judge preset loading buttons with robust warning cards.
3. Custom interactor code editor space for interactive problems.

---

### Task 6: Modernize the Contest Creation Form

**Files:**
- Modify: `web/src/pages/ContestCreate.tsx`

- [ ] **Step 1: Upgrade ContestCreate.tsx form**

1. Format card choices for ACM, OI, IOI, AtCoder, and Codeforces scoring modes.
2. Hide/show custom input configs (decay factors, penalty values) based on selected format with clean transitions.
3. Dynamic real-time lobby preview pane displaying formatted results.

---

### Task 7: End-to-End Compile & Build Verification

**Files:**
- Test: Build frontend codebase using Vite build command

- [ ] **Step 1: Build the Vite production bundle**

Run: `npm run build` or `vite build` inside `web/` to confirm absolute compilation success with zero typescript errors.

- [ ] **Step 2: Commit all changes**

```bash
git add internal/api/handler/problem.go internal/fps/zip.go internal/api/handler/testcase.go web/src/pages/SetterPanel.tsx web/src/pages/ProblemCreate.tsx web/src/pages/SetterProblemWorkspace.tsx web/src/pages/ContestCreate.tsx
git commit -m "feat: polygon feature-parity, secure zip extractor, and premium setter UI overhaul"
```
