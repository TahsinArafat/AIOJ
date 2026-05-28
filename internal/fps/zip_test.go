package fps

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractZip_ExtractsTestDataFiles(t *testing.T) {
	// Build a zip in memory
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// Write problem.xml
	w, _ := zw.Create("problem.xml")
	w.Write([]byte("<fps><problem><title>T</title><time_limit>1000</time_limit><memory_limit>256</memory_limit></problem></fps>"))

	// Write a test data file
	w2, _ := zw.Create("1.in")
	w2.Write([]byte("5 5"))

	zw.Close()

	tmpDir := t.TempDir()

	xmlContent, err := ExtractZip(buf.Bytes(), tmpDir)
	if err != nil {
		t.Fatalf("ExtractZip failed: %v", err)
	}

	if len(xmlContent) == 0 {
		t.Error("expected non-empty xmlContent from problem.xml")
	}

	// Check that 1.in was extracted
	extracted := filepath.Join(tmpDir, "1.in")
	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("expected 1.in to be extracted but it wasn't: %v", err)
	}
	if string(data) != "5 5" {
		t.Errorf("unexpected content in 1.in: %q", string(data))
	}
}

func TestExtractZip_RejectsPathTraversal(t *testing.T) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	zw.Create("../../evil.sh") // path traversal attempt
	zw.Close()

	tmpDir := t.TempDir()
	_, err := ExtractZip(buf.Bytes(), tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// evil.sh should NOT exist two directories above tmpDir
	_, statErr := os.Stat(filepath.Join(tmpDir, "evil.sh"))
	if !os.IsNotExist(statErr) {
		t.Error("path traversal file was written — security bug!")
	}
}
