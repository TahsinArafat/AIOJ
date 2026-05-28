# Phase 2: Problem Ecosystem — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement FPS (Free Problem Set) import/export and Hydro format support to allow users to easily import/export problems between AIOJ and other major OJ systems. Add ZIP file upload support for standard test cases.

**Architecture:** Create an `internal/fps` package to handle XML parsing, validation, and generation. Add an import handler (`internal/api/handler/import.go`) to handle file uploads (XML or ZIP), extract data, create DB models, and write test case files to disk. Update the testcase handler to support uploading standard test data in ZIP files.

**Tech Stack:** Go 1.21+, PostgreSQL 18, React 19 + TypeScript + Tailwind CSS

---

## File Structure

### Files to Create
```
internal/fps/fps.go             — FPS XML struct models (xml tags)
internal/fps/parser.go          — FPS XML Parser (XML → Problem model)
internal/fps/generator.go       — FPS XML Generator (Problem model → XML)
internal/fps/hydro.go           — Hydro format parser (YAML → Problem model)
internal/fps/zip.go             — ZIP extraction and validation helpers
internal/api/handler/import.go  — POST /api/problems/import (multipart XML/ZIP)
internal/fps/fps_test.go        — Unit tests for FPS parser/generator
```

### Files to Modify
```
internal/api/router.go          — Wire new import/export endpoints
internal/api/handler/problem.go  — Export endpoint handler + cleanup on Delete
internal/api/handler/testcase.go — Add ZIP upload/extraction logic
web/src/pages/ProblemList.tsx    — Import button (Admin/Setter only)
web/src/pages/ProblemDetail.tsx  — Export button (Admin/Setter only)
web/src/lib/api.ts               — Frontend API declarations
```

---

## Task 1: FPS XML Structs & Parser

**Files:**
- Create: `internal/fps/fps.go`
- Create: `internal/fps/parser.go`
- Create: `internal/fps/fps_test.go`

- [ ] **Step 1: Create FPS XML Structs**

Create `internal/fps/fps.go`:

```go
package fps

import "encoding/xml"

// FPS represents the root Free Problem Set element.
type FPS struct {
    XMLName  xml.Name  `xml:"fps"`
    Version  string    `xml:"version,attr"`
    Problems []Problem `xml:"problem"`
}

// Problem represents a single problem in FPS format.
type Problem struct {
    Title        string   `xml:"title"`
    TimeLimit    int      `xml:"time_limit"`   // in ms
    MemoryLimit  int      `xml:"memory_limit"` // in MB
    Description  string   `xml:"description"`
    Input        string   `xml:"input"`
    Output       string   `xml:"output"`
    SampleInput  []string `xml:"sample_input"`
    SampleOutput []string `xml:"sample_output"`
    TestInput    []string `xml:"test_input"`
    TestOutput   []string `xml:"test_output"`
    Hint         string   `xml:"hint"`
    Source       string   `xml:"source"`
    SPJ          *SPJ     `xml:"spj"`
    Tags         string   `xml:"tags"` // comma-separated
}

// SPJ represents a special judge program inside the FPS element.
type SPJ struct {
    Language   string `xml:"language,attr"`
    SourceCode string `xml:",cdata"`
}
```

- [ ] **Step 2: Create FPS Parser**

Create `internal/fps/parser.go`:

```go
package fps

import (
    "encoding/xml"
    "fmt"
    "strings"

    "github.com/tahsinarafat/aioj/internal/model"
)

// ParseXML parses raw FPS XML bytes into model.Problem slice.
func ParseXML(data []byte) ([]*model.Problem, error) {
    var raw FPS
    if err := xml.Unmarshal(data, &raw); err != nil {
        return nil, fmt.Errorf("failed to unmarshal FPS XML: %w", err)
    }

    var problems []*model.Problem
    for _, p := range raw.Problems {
        prob := &model.Problem{
            Title:       p.Title,
            Description: p.Description,
            InputFormat: p.Input,
            OutputFormat: p.Output,
            Hint:        p.Hint,
            TimeLimit:   p.TimeLimit,
            MemoryLimit: p.MemoryLimit * 1024, // MB to KB
            Source:      p.Source,
            Visible:     true,
        }

        // Map tags (comma-separated list to string array)
        if p.Tags != "" {
            parts := strings.Split(p.Tags, ",")
            for _, tag := range parts {
                trimmed := strings.TrimSpace(tag)
                if trimmed != "" {
                    prob.Tags = append(prob.Tags, trimmed)
                }
            }
        }

        // Map sample cases
        minSamples := len(p.SampleInput)
        if len(p.SampleOutput) < minSamples {
            minSamples = len(p.SampleOutput)
        }
        for i := 0; i < minSamples; i++ {
            prob.SampleCases = append(prob.SampleCases, model.SampleCase{
                Input:  strings.TrimSpace(p.SampleInput[i]),
                Output: strings.TrimSpace(p.SampleOutput[i]),
            })
        }

        // Map SPJ
        if p.SPJ != nil && p.SPJ.SourceCode != "" {
            prob.SPJ = true
            prob.SPJLanguage = p.SPJ.Language
            prob.SPJSourceCode = p.SPJ.SourceCode
        }

        problems = append(problems, prob)
    }

    return problems, nil
}
```

**IMPORTANT**: Verify the import path for `model` matches `go.mod`.

- [ ] **Step 3: Create FPS Unit Tests**

Create `internal/fps/fps_test.go`:

```go
package fps

import (
    "testing"
)

func TestParseXML_SingleProblem(t *testing.T) {
    xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<fps version="1.2">
  <problem>
    <title>A Plus B</title>
    <time_limit>1000</time_limit>
    <memory_limit>256</memory_limit>
    <description>Find A+B</description>
    <input>Two integers</input>
    <output>Their sum</output>
    <sample_input>1 2</sample_input>
    <sample_output>3</sample_output>
    <test_input>5 5</test_input>
    <test_output>10</test_output>
    <hint>Simple arithmetic</hint>
    <source>Codeforces 1A</source>
    <tags>math,basic</tags>
    <spj language="cpp"><![CDATA[#include <iostream>]]> </spj>
  </problem>
</fps>`)

    problems, err := ParseXML(xmlData)
    if err != nil {
        t.Fatalf("failed to parse: %v", err)
    }

    if len(problems) != 1 {
        t.Fatalf("expected 1 problem, got %d", len(problems))
    }

    p := problems[0]
    if p.Title != "A Plus B" {
        t.Errorf("expected title 'A Plus B', got %q", p.Title)
    }
    if p.TimeLimit != 1000 {
        t.Errorf("expected time limit 1000, got %d", p.TimeLimit)
    }
    if p.MemoryLimit != 256*1024 {
        t.Errorf("expected memory limit 262144, got %d", p.MemoryLimit)
    }
    if len(p.Tags) != 2 || p.Tags[0] != "math" || p.Tags[1] != "basic" {
        t.Errorf("unexpected tags: %v", p.Tags)
    }
    if !p.SPJ || p.SPJLanguage != "cpp" || !strings.Contains(p.SPJSourceCode, "#include") {
        t.Errorf("SPJ mapping failed: spj=%v, lang=%s", p.SPJ, p.SPJLanguage)
    }
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/fps/ -v`
Expected: Test passes

- [ ] **Step 5: Commit**

```bash
git add internal/fps/fps.go internal/fps/parser.go internal/fps/fps_test.go
git commit -m "feat: implement FPS XML structs and basic parser"
```

---

## Task 2: FPS Generator

**Files:**
- Modify: `internal/fps/fps.go`
- Create: `internal/fps/generator.go`
- Modify: `internal/fps/fps_test.go`

- [ ] **Step 1: Create Generator**

Create `internal/fps/generator.go`:

```go
package fps

import (
    "encoding/xml"
    "fmt"

    "github.com/tahsinarafat/aioj/internal/model"
)

// GenerateXML generates raw FPS XML bytes from a slice of model.Problem.
// Note: testCases should be provided for inline test cases.
func GenerateXML(problems []*model.Problem, testcasesMap map[string][]model.TestCaseScore) ([]byte, error) {
    fps := FPS{
        Version: "1.2",
    }

    for _, p := range problems {
        prob := Problem{
            Title:       p.Title,
            TimeLimit:   p.TimeLimit,
            MemoryLimit: p.MemoryLimit / 1024, // KB to MB
            Description: p.Description,
            Input:       p.InputFormat,
            Output:      p.OutputFormat,
            Hint:        p.Hint,
            Source:      p.Source,
        }

        // Map tags
        if len(p.Tags) > 0 {
            prob.Tags = ""
            for i, tag := range p.Tags {
                if i > 0 {
                    prob.Tags += ","
                }
                prob.Tags += tag
            }
        }

        // Map samples
        for _, sample := range p.SampleCases {
            prob.SampleInput = append(prob.SampleInput, sample.Input)
            prob.SampleOutput = append(prob.SampleOutput, sample.Output)
        }

        // Map inline test cases if scores exist
        if cases, ok := testcasesMap[p.ID]; ok {
            for _, tc := range cases {
                prob.TestInput = append(prob.TestInput, tc.InputName)
                prob.TestOutput = append(prob.TestOutput, tc.OutputName)
            }
        }

        // Map SPJ
        if p.SPJ {
            prob.SPJ = &SPJ{
                Language:   p.SPJLanguage,
                SourceCode: p.SPJSourceCode,
            }
        }

        fps.Problems = append(fps.Problems, prob)
    }

    output, err := xml.MarshalIndent(fps, "", "  ")
    if err != nil {
        return nil, fmt.Errorf("failed to marshal FPS XML: %w", err)
    }

    // Add XML declaration header
    header := []byte(xml.Header)
    return append(header, output...), nil
}
```

- [ ] **Step 2: Add Generator test**

Add to `internal/fps/fps_test.go`:

```go
func TestGenerateXML(t *testing.T) {
    p := &model.Problem{
        ID:          "1",
        Title:       "Test",
        TimeLimit:   1000,
        MemoryLimit: 262144, // 256MB
        Description: "desc",
        Tags:        []string{"math"},
    }

    xmlData, err := GenerateXML([]*model.Problem{p}, nil)
    if err != nil {
        t.Fatalf("failed to generate: %v", err)
    }

    if !strings.Contains(string(xmlData), "<title>Test</title>") {
        t.Error("missing title element in generated XML")
    }
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/fps/ -v`
Expected: Tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/fps/generator.go internal/fps/fps_test.go
git commit -m "feat: implement FPS XML generator"
```

---

## Task 3: ZIP Extraction & Import Helpers

**Files:**
- Create: `internal/fps/zip.go`

- [ ] **Step 1: Create ZIP extraction helper**

Create `internal/fps/zip.go` to handle zip extraction and FPS zip parsing:

```go
package fps

import (
    "archive/zip"
    "bytes"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
)

// ExtractZip reads a ZIP archive and extracts all entries into the target directory.
// Returns the problem.xml content if found in the root of the zip.
func ExtractZip(zipData []byte, targetDir string) ([]byte, error) {
    reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
    if err != nil {
        return nil, fmt.Errorf("failed to read zip archive: %w", err)
    }

    if err := os.MkdirAll(targetDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create target directory: %w", err)
    }

    var xmlContent []byte

    for _, file := range reader.File {
        // Path traversal protection
        cleaned := filepath.Clean(file.Name)
        if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "/") || strings.Contains(cleaned, "\\") {
            continue // skip unsafe paths
        }

        rc, err := file.Open()
        if err != nil {
            return nil, fmt.Errorf("failed to open zip file entry: %w", err)
        }

        // If it's the problem XML file, read into memory
        if file.Name == "problem.xml" || file.Name == "fps.xml" {
            xmlContent, err = io.ReadAll(rc)
            rc.Close()
            if err != nil {
                return nil, fmt.Errorf("failed to read xml entry: %w", err)
            }
            continue
        }

        // Write files to targetDir
        destPath := filepath.Join(targetDir, file.Name)
        destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
        if err != nil {
            rc.Close()
            return nil, fmt.Errorf("failed to create target file: %w", err)
        }

        if _, err := io.Copy(destFile, rc); err != nil {
            destFile.Close()
            rc.Close()
            return nil, fmt.Errorf("failed to write target file: %w", err)
        }

        destFile.Close()
        rc.Close()
    }

    return xmlContent, nil
}
```

- [ ] **Step 2: Build verification**

Run: `go build ./internal/fps/`
Expected: Compiles clean

- [ ] **Step 3: Commit**

```bash
git add internal/fps/zip.go
git commit -m "feat: add ZIP extraction utility for problem packages"
```

---

## Task 4: API Import Handler

**Files:**
- Create: `internal/api/handler/import.go`
- Modify: `internal/api/router.go`
- Modify: `cmd/aioj/main.go`

- [ ] **Step 1: Create Import Handler**

Create `internal/api/handler/import.go`:

```go
package handler

import (
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strings"

    "github.com/google/uuid"
    "github.com/tahsinarafat/aioj/internal/fps"
    "github.com/tahsinarafat/aioj/internal/model"
    "github.com/tahsinarafat/aioj/internal/store"
)

type ImportHandler struct {
    probStore store.ProblemStore
    dataDir   string
}

func NewImportHandler(probStore store.ProblemStore, dataDir string) *ImportHandler {
    return &ImportHandler{
        probStore: probStore,
        dataDir:   dataDir,
    }
}

func (h *ImportHandler) Import(w http.ResponseWriter, r *http.Request) {
    // 1. Authenticate user (Admin/Setter only)
    claims := r.Context().Value("claims") // update with actual auth retrieval
    if claims == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // 2. Parse Multipart File
    file, header, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "missing file parameter", http.StatusBadRequest)
        return
    }
    defer file.Close()

    fileBytes, err := io.ReadAll(file)
    if err != nil {
        http.Error(w, "failed to read file", http.StatusInternalServerError)
        return
    }

    var xmlBytes []byte
    var probDir string
    problemID := uuid.New().String()

    // 3. Handle ZIP vs XML
    if strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
        probDir = filepath.Join(h.dataDir, problemID)
        xmlBytes, err = fps.ExtractZip(fileBytes, probDir)
        if err != nil {
            http.Error(w, "failed to extract zip: "+err.Error(), http.StatusBadRequest)
            return
        }
        if len(xmlBytes) == 0 {
            http.Error(w, "missing problem.xml or fps.xml in root of zip archive", http.StatusBadRequest)
            return
        }
    } else if strings.HasSuffix(strings.ToLower(header.Filename), ".xml") {
        xmlBytes = fileBytes
    } else {
        http.Error(w, "unsupported file format; must be .xml or .zip", http.StatusBadRequest)
        return
    }

    // 4. Parse FPS XML
    problems, err := fps.ParseXML(xmlBytes)
    if err != nil {
        http.Error(w, "failed to parse FPS XML: "+err.Error(), http.StatusBadRequest)
        return
    }
    if len(problems) == 0 {
        http.Error(w, "xml contains no problems", http.StatusBadRequest)
        return
    }

    // Support single problem creation in v1
    p := problems[0]
    p.ID = problemID
    p.Slug = strings.ToLower(strings.ReplaceAll(p.Title, " ", "-")) // basic slug fallback
    
    // Save to db
    p.TestdataPath = probDir
    if err := h.probStore.Create(r.Context(), p); err != nil {
        if probDir != "" {
            os.RemoveAll(probDir) // clean up files on DB failure
        }
        http.Error(w, "failed to save problem: "+err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusCreated)
    // Return minimal success JSON
    w.Write([]byte(`{"status":"success","problem_id":"` + p.ID + `","slug":"` + p.Slug + `"}`))
}
```

*Note: Update the auth check logic to match the existing handler pattern (e.g. check roles in JWT claims).*

- [ ] **Step 2: Register routes in router.go**

In `internal/api/router.go`, add route for import under Auth middleware routes:
```go
// Add under router group with auth
r.Post("/api/problems/import", importHandler.Import)
```

Ensure `importHandler` is passed as a parameter to `NewRouter(...)`.

- [ ] **Step 3: Instantiation in main.go**

In `cmd/aioj/main.go`, add:
```go
importHandler := handler.NewImportHandler(problemStore, "./testdata")
```
Pass it to `NewRouter(...)` in the router wiring block.

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: Compiles clean

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/import.go internal/api/router.go cmd/aioj/main.go
git commit -m "feat: create import API handler and wire routes"
```

---

## Task 5: Problem Deletion Cleanup & Exporter

**Files:**
- Modify: `internal/api/handler/problem.go`

- [ ] **Step 1: Clean up testdata files on Problem Delete**

In `internal/api/handler/problem.go`, find the `Delete` method. Add the file cleanup step before or after DB delete:

```go
// Load problem to get TestdataPath
prob, err := h.store.GetBySlug(r.Context(), slug)
if err == nil && prob.TestdataPath != "" {
    // Safely remove the directory on disk
    os.RemoveAll(prob.TestdataPath)
}
```

- [ ] **Step 2: Add Export endpoint in Problem Handler**

In `internal/api/handler/problem.go` (or `import.go`), add the `Export` method:

```go
func (h *ProblemHandler) Export(w http.ResponseWriter, r *http.Request) {
    slug := chi.URLParam(r, "slug")
    prob, err := h.store.GetBySlug(r.Context(), slug)
    if err != nil {
        http.Error(w, "problem not found", http.StatusNotFound)
        return
    }

    // Generate XML bytes
    xmlBytes, err := fps.GenerateXML([]*model.Problem{prob}, nil)
    if err != nil {
        http.Error(w, "failed to generate XML: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // Create a temporary zip file
    buf := new(bytes.Buffer)
    zipWriter := zip.NewWriter(buf)

    // Write XML file
    xmlFile, err := zipWriter.Create("problem.xml")
    if err != nil {
        http.Error(w, "failed to create XML in zip", http.StatusInternalServerError)
        return
    }
    if _, err := xmlFile.Write(xmlBytes); err != nil {
        http.Error(w, "failed to write XML to zip", http.StatusInternalServerError)
        return
    }

    // Zip up all test data files on disk
    if prob.TestdataPath != "" {
        files, err := os.ReadDir(prob.TestdataPath)
        if err == nil {
            for _, f := range files {
                if f.IsDir() { continue }
                fPath := filepath.Join(prob.TestdataPath, f.Name())
                data, err := os.ReadFile(fPath)
                if err != nil { continue }

                zf, err := zipWriter.Create(f.Name())
                if err != nil { continue }
                zf.Write(data)
            }
        }
    }

    zipWriter.Close()

    // Send ZIP file download response
    w.Header().Set("Content-Type", "application/zip")
    w.Header().Set("Content-Disposition", "attachment; filename=\""+prob.Slug+".zip\"")
    w.Write(buf.Bytes())
}
```

- [ ] **Step 3: Register export route**

In `internal/api/router.go`, add route for export:
```go
r.Get("/api/problems/{slug}/export", problemHandler.Export)
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: Compiles clean

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/problem.go internal/api/router.go
git commit -m "feat: implement problem export as FPS ZIP and clean up test data on deletion"
```

---

## Task 6: ZIP Test Case Upload for Standard Problems

**Files:**
- Modify: `internal/api/handler/testcase.go`

- [ ] **Step 1: Add ZIP upload extraction in Test Case handler**

In `internal/api/handler/testcase.go`, modify `Upload` to check if a uploaded file is a ZIP archive, and extract it if so:

Read the current `Upload` method. Find the file handling block. Replace it with:

```go
// Check if the uploaded file is a ZIP archive
// We need to parse headers or filename
filename := fileHeader.Filename
if strings.HasSuffix(strings.ToLower(filename), ".zip") {
    // Read raw bytes
    zipBytes, err := io.ReadAll(file)
    if err != nil {
        http.Error(w, "failed to read zip", http.StatusBadRequest)
        return
    }
    
    // Extract it
    _, err = fps.ExtractZip(zipBytes, probDir)
    if err != nil {
        http.Error(w, "failed to extract zip: "+err.Error(), http.StatusBadRequest)
        return
    }
} else {
    // Fall back to existing single-file saving logic
    saveFile(file, filename, probDir)
}
```

- [ ] **Step 2: Run backend tests**

Run: `go test ./...`
Expected: Passes clean

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler/testcase.go
git commit -m "feat: add ZIP upload and extraction support for standard test cases"
```

---

## Task 7: Frontend Import/Export Controls

**Files:**
- Modify: `web/src/pages/ProblemList.tsx`
- Modify: `web/src/pages/ProblemDetail.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add API exports in `web/src/lib/api.ts`**

Add endpoints to frontend API:

```typescript
// in web/src/lib/api.ts problems namespace:
importProblem: (file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    return request<ImportResponse>('/api/problems/import', {
        method: 'POST',
        body: formData,
    });
},
exportProblemUrl: (slug: string) => `/api/problems/${slug}/export`,
```

- [ ] **Step 2: Add Import control in ProblemList.tsx**

In `web/src/pages/ProblemList.tsx`, add an "Import Problems" file selector button near the "Create Problem" button (visible to Admin/Setter only).

```tsx
const handleImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    
    try {
        const res = await api.problems.importProblem(file);
        // Toast/Alert success, redirect to workspace
        window.location.href = `/problems/${res.slug}`;
    } catch (err) {
        alert("Failed to import problem: " + err);
    }
};
```

Render block:
```tsx
{isAdminOrSetter && (
  <label className="inline-flex items-center px-4 py-2 bg-blue-600 text-white rounded cursor-pointer text-sm font-semibold hover:bg-blue-700">
    Import Problem (XML/ZIP)
    <input type="file" accept=".xml,.zip" className="hidden" onChange={handleImport} />
  </label>
)}
```

- [ ] **Step 3: Add Export button in ProblemDetail.tsx**

In `web/src/pages/ProblemDetail.tsx`, add an "Export Problem" link/button near the admin edit workspace link.

```tsx
{isAdminOrSetter && (
  <a
    href={api.problems.exportProblemUrl(problem.slug)}
    download
    className="inline-flex items-center px-4 py-2 border rounded text-sm hover:bg-gray-50"
  >
    Export (FPS ZIP)
  </a>
)}
```

- [ ] **Step 4: Build frontend**

Run: `cd web && npm run build`
Expected: Compiles cleanly

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/ProblemList.tsx web/src/pages/ProblemDetail.tsx web/src/lib/api.ts
git commit -m "feat: add problem import/export UI buttons and forms in frontend"
```

---

## Task 8: Integration Tests

**Files:**
- Create: `internal/fps/integration_test.go`

- [ ] **Step 1: Write integration test for import/export loop**

```go
package fps

import (
    "testing"

    "github.com/tahsinarafat/aioj/internal/model"
)

func TestImportExportLoop(t *testing.T) {
    // 1. Create a dummy problem
    original := &model.Problem{
        Title:       "Sum Loop",
        TimeLimit:   1000,
        MemoryLimit: 262144,
        Description: "desc text",
        Tags:        []string{"math", "easy"},
    }

    // 2. Generate XML
    xmlBytes, err := GenerateXML([]*model.Problem{original}, nil)
    if err != nil {
        t.Fatalf("generate failed: %v", err)
    }

    // 3. Parse XML back
    parsed, err := ParseXML(xmlBytes)
    if err != nil {
        t.Fatalf("parse failed: %v", err)
    }

    if len(parsed) != 1 {
        t.Fatalf("expected 1 problem, got %d", len(parsed))
    }

    imported := parsed[0]
    if imported.Title != original.Title {
        t.Errorf("title mismatch: %q vs %q", imported.Title, original.Title)
    }
    if imported.MemoryLimit != original.MemoryLimit {
        t.Errorf("memory limit mismatch: %d vs %d", imported.MemoryLimit, original.MemoryLimit)
    }
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/fps/ -v -run TestImportExportLoop`
Expected: Passes clean

- [ ] **Step 3: Commit**

```bash
git add internal/fps/integration_test.go
git commit -m "test: add integration test for FPS import/export loop"
```

---

## Verification Checklist

After completing all tasks:

- [ ] Backend tests pass (`go test ./...`)
- [ ] Frontend builds cleanly (`cd web && npm run build`)
- [ ] Import XML/ZIP creates problems successfully
- [ ] Export problem yields a valid ZIP download containing the XML statement and testdata files
- [ ] Uploading test cases via ZIP extracts files to disk and links to problem
- [ ] Problem deletion clears the corresponding directory on disk
