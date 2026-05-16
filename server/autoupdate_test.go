package server

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAutoUpdateVersionCompare(t *testing.T) {
	if !isVersionGreater("1.1.7", "1.1.6") {
		t.Fatalf("expected 1.1.7 to be newer than 1.1.6")
	}
	if isVersionGreater("1.1.6", "1.1.6") {
		t.Fatalf("expected equal versions not to update")
	}
	if isVersionGreater("1.1.5", "1.1.6") {
		t.Fatalf("expected older version not to update")
	}
}

func TestCoreUpdaterFindLatestRelease(t *testing.T) {
	server := newResourceTestServer(t, map[string][]resourceEntry{
		"StellarCore": {
			{Name: "source-a", IsDir: true},
		},
		"StellarCore/source-a": {
			{Name: "frps", IsDir: true},
		},
		"StellarCore/source-a/frps": {
			{Name: "1.1.6", IsDir: true},
			{Name: "1.1.7", IsDir: true},
		},
		"StellarCore/source-a/frps/1.1.7": {
			{Name: "StellarCore_1.1.7_linux_amd64.zip"},
			{Name: "StellarCore_1.1.7_linux_amd64.tar.gz"},
		},
	})
	defer server.Close()

	updater := &coreAutoUpdater{
		httpClient:      server.Client(),
		apiURL:          server.URL + "/api/fs/list",
		downloadBaseURL: server.URL + "/d/StellarCore",
		rootPath:        "StellarCore",
		goos:            "linux",
		goarch:          "amd64",
	}

	candidate, err := updater.findLatestRelease(context.Background())
	if err != nil {
		t.Fatalf("findLatestRelease returned error: %v", err)
	}
	if candidate.Version != "1.1.7" {
		t.Fatalf("version = %q, want 1.1.7", candidate.Version)
	}
	if candidate.AssetName != "StellarCore_1.1.7_linux_amd64.tar.gz" {
		t.Fatalf("asset = %q, want tar.gz asset", candidate.AssetName)
	}
}

func TestCoreUpdaterCheckLatestDetectsUpdate(t *testing.T) {
	server := newResourceTestServer(t, map[string][]resourceEntry{
		"StellarCore": {
			{Name: "source-a", IsDir: true},
		},
		"StellarCore/source-a": {
			{Name: "frps", IsDir: true},
		},
		"StellarCore/source-a/frps": {
			{Name: "1.1.7", IsDir: true},
		},
		"StellarCore/source-a/frps/1.1.7": {
			{Name: "StellarCore_1.1.7_linux_amd64.tar.gz"},
		},
	})
	defer server.Close()

	updater := &coreAutoUpdater{
		httpClient:      server.Client(),
		apiURL:          server.URL + "/api/fs/list",
		downloadBaseURL: server.URL + "/d/StellarCore",
		rootPath:        "StellarCore",
		goos:            "linux",
		goarch:          "amd64",
		currentVersion:  "1.1.6",
	}

	result, err := updater.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest returned error: %v", err)
	}
	if !result.HasUpdate {
		t.Fatalf("expected update to be available")
	}
	if result.CurrentVersion != "1.1.6" || result.Candidate.Version != "1.1.7" {
		t.Fatalf("unexpected check result %#v", result)
	}
}

func TestCoreUpdaterCheckLatestNoUpdate(t *testing.T) {
	server := newResourceTestServer(t, map[string][]resourceEntry{
		"StellarCore": {
			{Name: "source-a", IsDir: true},
		},
		"StellarCore/source-a": {
			{Name: "frps", IsDir: true},
		},
		"StellarCore/source-a/frps": {
			{Name: "1.1.6", IsDir: true},
		},
		"StellarCore/source-a/frps/1.1.6": {
			{Name: "StellarCore_1.1.6_linux_amd64.tar.gz"},
		},
	})
	defer server.Close()

	updater := &coreAutoUpdater{
		httpClient:      server.Client(),
		apiURL:          server.URL + "/api/fs/list",
		downloadBaseURL: server.URL + "/d/StellarCore",
		rootPath:        "StellarCore",
		goos:            "linux",
		goarch:          "amd64",
		currentVersion:  "1.1.6",
	}

	result, err := updater.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest returned error: %v", err)
	}
	if result.HasUpdate {
		t.Fatalf("expected no update, got %#v", result)
	}
	if result.Candidate == nil || result.Candidate.Version != "1.1.6" {
		t.Fatalf("unexpected candidate %#v", result.Candidate)
	}
}

func TestCoreUpdaterFindLatestReleaseWithoutMatchingAsset(t *testing.T) {
	server := newResourceTestServer(t, map[string][]resourceEntry{
		"StellarCore": {
			{Name: "source-a", IsDir: true},
		},
		"StellarCore/source-a": {
			{Name: "frps", IsDir: true},
		},
		"StellarCore/source-a/frps": {
			{Name: "1.1.7", IsDir: true},
		},
		"StellarCore/source-a/frps/1.1.7": {
			{Name: "StellarCore_1.1.7_linux_arm64.tar.gz"},
		},
	})
	defer server.Close()

	updater := &coreAutoUpdater{
		httpClient:      server.Client(),
		apiURL:          server.URL + "/api/fs/list",
		downloadBaseURL: server.URL + "/d/StellarCore",
		rootPath:        "StellarCore",
		goos:            "linux",
		goarch:          "amd64",
	}

	if _, err := updater.findLatestRelease(context.Background()); err == nil {
		t.Fatalf("expected error when no matching asset exists")
	}
}

func TestCoreUpdaterListResourcesRejectsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{"))
	}))
	defer server.Close()

	updater := &coreAutoUpdater{
		httpClient: server.Client(),
		apiURL:     server.URL,
	}
	if _, err := updater.listResources(context.Background(), "StellarCore"); err == nil {
		t.Fatalf("expected invalid JSON error")
	}
}

func TestExtractArchiveFindsExpectedBinary(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "StellarCore_1.1.7_linux_amd64.tar.gz")
	writeTarGz(t, archivePath, map[string]string{
		"StellarCore": "new-binary",
	})

	extractDir := filepath.Join(dir, "extract")
	if err := extractArchive(archivePath, extractDir); err != nil {
		t.Fatalf("extractArchive returned error: %v", err)
	}
	binaryPath, err := findExpectedBinary(extractDir)
	if err != nil {
		t.Fatalf("findExpectedBinary returned error: %v", err)
	}
	if filepath.Base(binaryPath) != "StellarCore" {
		t.Fatalf("binary = %q, want StellarCore", filepath.Base(binaryPath))
	}
}

func TestExtractArchiveRejectsUnsafeZipPath(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "unsafe.zip")
	writeZip(t, archivePath, map[string]string{
		"../StellarCore": "bad",
	})

	if err := extractArchive(archivePath, filepath.Join(dir, "extract")); err == nil {
		t.Fatalf("expected unsafe archive path error")
	}
}

func TestReplaceExecutableAndRestartSuccess(t *testing.T) {
	dir := t.TempDir()
	executablePath := filepath.Join(dir, "stellarcore")
	newBinaryPath := filepath.Join(dir, "new-StellarCore")
	if err := os.WriteFile(executablePath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBinaryPath, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}

	restarted := false
	err := replaceExecutableAndRestart(executablePath, newBinaryPath, func() error {
		restarted = true
		return nil
	})
	if err != nil {
		t.Fatalf("replaceExecutableAndRestart returned error: %v", err)
	}
	if !restarted {
		t.Fatalf("expected restart executor to be called")
	}
	content, err := os.ReadFile(executablePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("executable content = %q, want new", string(content))
	}
	backup, err := os.ReadFile(filepath.Join(dir, ".stellarcore.bak"))
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != "old" {
		t.Fatalf("backup content = %q, want old", string(backup))
	}
}

func TestReplaceExecutableAndRestartRestoresOnRestartError(t *testing.T) {
	dir := t.TempDir()
	executablePath := filepath.Join(dir, "stellarcore")
	newBinaryPath := filepath.Join(dir, "new-StellarCore")
	if err := os.WriteFile(executablePath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBinaryPath, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}

	errBoom := errors.New("boom")
	err := replaceExecutableAndRestart(executablePath, newBinaryPath, func() error {
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want boom", err)
	}
	content, err := os.ReadFile(executablePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "old" {
		t.Fatalf("executable content = %q, want old", string(content))
	}
}

func newResourceTestServer(t *testing.T, responses map[string][]resourceEntry) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourcePath := r.URL.Query().Get("path")
		entries, ok := responses[resourcePath]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		body := resourceListResponse{Code: http.StatusOK}
		body.Data.Content = entries
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
}

func writeTarGz(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()
	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(out)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeZip(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()
	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zipWriter := zip.NewWriter(out)
	for name, content := range files {
		writer, err := zipWriter.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}
