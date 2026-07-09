package cmds

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerifiedDownloadRequiresHTTPSAndDigest(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "tool")
	if err := DownloadVerifiedFromURL("tool", "http://example.com/tool", "binary", "tool", dest, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("HTTP URL error = %v", err)
	}
	if err := DownloadFromURL("tool", "https://example.com/tool", "binary", "tool", dest); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("missing digest error = %v", err)
	}
	if err := DownloadVerifiedFromURL("tool", "https://example.com/tool", "binary", "tool", dest, "bad"); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("invalid digest error = %v", err)
	}

	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "binary")
	}))
	defer insecure.Close()
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, insecure.URL, http.StatusFound)
	}))
	defer secure.Close()
	client := secure.Client()
	client.CheckRedirect = requireHTTPSRedirect
	useDownloadClient(t, client)
	if err := DownloadVerifiedFromURL("tool", secure.URL, "binary", "tool", dest, sha256Hex("binary")); err == nil || !strings.Contains(err.Error(), "redirect must use HTTPS") {
		t.Fatalf("insecure redirect error = %v", err)
	}
}

func TestVerifiedDownloadAtomicallyReplacesOnlyAfterDigestMatch(t *testing.T) {
	content := "verified binary"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, content)
	}))
	defer server.Close()
	useDownloadClient(t, server.Client())

	root := t.TempDir()
	dest := filepath.Join(root, "tool")
	if err := os.WriteFile(dest, []byte("working binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := DownloadVerifiedFromURL("tool", server.URL, "binary", "tool", dest, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("digest mismatch error = %v", err)
	}
	assertFileContent(t, dest, "working binary")
	assertNoDownloadTemps(t, root)

	if err := DownloadVerifiedFromURL("tool", server.URL, "binary", "tool", dest, sha256Hex(content)); err != nil {
		t.Fatalf("verified download: %v", err)
	}
	assertFileContent(t, dest, content)
	if executableMode(t, dest)&0o111 == 0 {
		t.Fatalf("installed file is not executable")
	}
	assertNoDownloadTemps(t, root)
}

func TestVerifiedDownloadPreservesExistingBinaryOnRequestAndRenameFailures(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "tool")
	if err := os.WriteFile(dest, []byte("working"), 0o755); err != nil {
		t.Fatal(err)
	}

	downloadHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection failed")
	})}
	t.Cleanup(func() { downloadHTTPClient = newDownloadHTTPClient() })
	if err := DownloadVerifiedFromURL("tool", "https://example.com/tool", "binary", "tool", dest, strings.Repeat("1", 64)); err == nil || !strings.Contains(err.Error(), "connection failed") {
		t.Fatalf("connection error = %v", err)
	}
	assertFileContent(t, dest, "working")
	assertNoDownloadTemps(t, root)

	content := "replacement"
	downloadHTTPClient = responseClient(io.NopCloser(strings.NewReader(content)))
	originalRename := renameFileFunc
	renameFileFunc = func(string, string) error { return errors.New("rename failed") }
	t.Cleanup(func() { renameFileFunc = originalRename })
	if err := DownloadVerifiedFromURL("tool", "https://example.com/tool", "binary", "tool", dest, sha256Hex(content)); err == nil || !strings.Contains(err.Error(), "rename failed") {
		t.Fatalf("rename error = %v", err)
	}
	assertFileContent(t, dest, "working")
	assertNoDownloadTemps(t, root)
}

func TestDownloadRequestTimeoutsAndStatus(t *testing.T) {
	client := newDownloadHTTPClient()
	if downloadDialer.Timeout != connectionTimeout {
		t.Fatalf("connection timeout = %v", downloadDialer.Timeout)
	}
	if client.Timeout != totalRequestTimeout {
		t.Fatalf("total timeout = %v", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.DialContext == nil {
		t.Fatalf("connection-timeout transport is not configured")
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow":
			time.Sleep(100 * time.Millisecond)
			_, _ = io.WriteString(w, "late")
		default:
			http.Error(w, "missing", http.StatusNotFound)
		}
	}))
	defer server.Close()
	timeoutClient := server.Client()
	timeoutClient.Timeout = 10 * time.Millisecond
	useDownloadClient(t, timeoutClient)
	dest := filepath.Join(t.TempDir(), "tool")
	if err := DownloadVerifiedFromURL("tool", server.URL+"/slow", "binary", "tool", dest, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "Client.Timeout") {
		t.Fatalf("total timeout error = %v", err)
	}

	statusClient := server.Client()
	useDownloadClient(t, statusClient)
	if err := DownloadVerifiedFromURL("tool", server.URL+"/missing", "binary", "tool", dest, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "status code 404") {
		t.Fatalf("status error = %v", err)
	}
}

func TestDownloadReadWriteCloseAndSyncFailures(t *testing.T) {
	tests := []struct {
		name string
		body io.ReadCloser
		file *failingTemporaryFile
		want string
	}{
		{name: "read", body: io.NopCloser(&failingReader{}), file: &failingTemporaryFile{}, want: "read failed"},
		{name: "partial response", body: io.NopCloser(&partialReader{}), file: &failingTemporaryFile{}, want: "unexpected EOF"},
		{name: "response close", body: &closeFailingBody{Reader: strings.NewReader("content")}, file: &failingTemporaryFile{}, want: "close response body"},
		{name: "write", body: io.NopCloser(strings.NewReader("content")), file: &failingTemporaryFile{writeErr: errors.New("write failed")}, want: "write failed"},
		{name: "partial write", body: io.NopCloser(strings.NewReader("content")), file: &failingTemporaryFile{partialWrite: true}, want: "short write"},
		{name: "sync", body: io.NopCloser(strings.NewReader("content")), file: &failingTemporaryFile{syncErr: errors.New("sync failed")}, want: "sync failed"},
		{name: "file close", body: io.NopCloser(strings.NewReader("content")), file: &failingTemporaryFile{closeErr: errors.New("close failed")}, want: "close failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			downloadHTTPClient = responseClient(test.body)
			t.Cleanup(func() { downloadHTTPClient = newDownloadHTTPClient() })
			_, err := downloadToTemporaryFile("https://example.com/tool", test.file)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestDownloadAndExtractionSizeBounds(t *testing.T) {
	var destination bytes.Buffer
	if _, err := copyBounded(&destination, strings.NewReader("12345"), 4, "archive"); err == nil || !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("archive size error = %v", err)
	}

	archive := filepath.Join(t.TempDir(), "oversized.tar")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := tar.NewWriter(file)
	if err := writer.WriteHeader(&tar.Header{Name: "tool", Mode: 0o755, Size: maxBinarySize + 1}); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	opened, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if err := extractTarEntry(tar.NewReader(opened), "tool", io.Discard); err == nil || !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("binary size error = %v", err)
	}
}

func TestArchiveRejectsTraversalAmbiguityAndPartialContent(t *testing.T) {
	tests := []struct {
		name    string
		entries []tarEntry
		want    string
	}{
		{name: "traversal", entries: []tarEntry{{name: "../tool", content: "bad"}}, want: "unsafe archive path"},
		{name: "backslash traversal", entries: []tarEntry{{name: `..\tool`, content: "bad"}}, want: "unsafe archive path"},
		{name: "ambiguous", entries: []tarEntry{{name: "one/tool", content: "one"}, {name: "two/tool", content: "two"}}, want: "multiple files"},
		{name: "exact name", entries: []tarEntry{{name: "tool-extra", content: "bad"}}, want: "not found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "archive.tar.gz")
			writeTarEntries(t, archive, test.entries)
			if err := extractTarGzTo(archive, "tool", io.Discard); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}

	truncated := bytes.NewBuffer(nil)
	tarWriter := tar.NewWriter(truncated)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "tool", Mode: 0o755, Size: 10}); err != nil {
		t.Fatal(err)
	}
	_, _ = tarWriter.Write([]byte("short"))
	if err := extractTarEntry(tar.NewReader(bytes.NewReader(truncated.Bytes())), "tool", io.Discard); err == nil {
		t.Fatalf("expected partial extraction error")
	}
}

func TestExtractionFailureCleansTemporaryDataAndPreservesBinary(t *testing.T) {
	content := "not an archive"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, content)
	}))
	defer server.Close()
	useDownloadClient(t, server.Client())
	root := t.TempDir()
	dest := filepath.Join(root, "tool")
	if err := os.WriteFile(dest, []byte("working"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := DownloadVerifiedFromURL("tool", server.URL, "tar.gz", "tool", dest, sha256Hex(content)); err == nil || !strings.Contains(err.Error(), "extract") {
		t.Fatalf("extraction error = %v", err)
	}
	assertFileContent(t, dest, "working")
	assertNoDownloadTemps(t, root)
}

func TestExtractedBinaryWriteSyncAndCloseFailures(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "tool.tar.gz")
	writeTarGz(t, archivePath, "bin/tool", "binary")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archiveContent)
	}))
	defer server.Close()
	useDownloadClient(t, server.Client())

	tests := []struct {
		name string
		file *failingTemporaryFile
		want string
	}{
		{name: "write", file: &failingTemporaryFile{writeErr: errors.New("extracted write failed")}, want: "extracted write failed"},
		{name: "sync", file: &failingTemporaryFile{syncErr: errors.New("extracted sync failed")}, want: "extracted sync failed"},
		{name: "close", file: &failingTemporaryFile{closeErr: errors.New("extracted close failed")}, want: "extracted close failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			dest := filepath.Join(root, "tool")
			if err := os.WriteFile(dest, []byte("working"), 0o755); err != nil {
				t.Fatal(err)
			}
			originalCreate := createTempFileFunc
			calls := 0
			createTempFileFunc = func(dir, pattern string) (temporaryFile, error) {
				calls++
				if calls == 1 {
					return os.CreateTemp(dir, pattern)
				}
				backing, err := os.CreateTemp(dir, pattern)
				if err != nil {
					return nil, err
				}
				test.file.backing = backing
				return test.file, nil
			}
			t.Cleanup(func() { createTempFileFunc = originalCreate })
			err := DownloadVerifiedFromURL("tool", server.URL, "tar.gz", "tool", dest, sha256File(t, archivePath))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
			assertFileContent(t, dest, "working")
			assertNoDownloadTemps(t, root)
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func responseClient(body io.ReadCloser) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: body, Header: make(http.Header)}, nil
	})}
}

type failingReader struct{}

func (*failingReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

type partialReader struct {
	sent bool
}

func (reader *partialReader) Read(content []byte) (int, error) {
	if reader.sent {
		return 0, io.ErrUnexpectedEOF
	}
	reader.sent = true
	return copy(content, "partial"), io.ErrUnexpectedEOF
}

type closeFailingBody struct {
	io.Reader
}

func (*closeFailingBody) Close() error {
	return errors.New("close response body")
}

type failingTemporaryFile struct {
	buffer       bytes.Buffer
	backing      temporaryFile
	writeErr     error
	partialWrite bool
	syncErr      error
	closeErr     error
}

func (file *failingTemporaryFile) Write(content []byte) (int, error) {
	if file.writeErr != nil {
		return 0, file.writeErr
	}
	if file.partialWrite {
		return len(content) / 2, nil
	}
	if file.backing != nil {
		return file.backing.Write(content)
	}
	return file.buffer.Write(content)
}

func (file *failingTemporaryFile) Name() string {
	if file.backing != nil {
		return file.backing.Name()
	}
	return "temporary"
}

func (file *failingTemporaryFile) Chmod(mode os.FileMode) error {
	if file.backing != nil {
		return file.backing.Chmod(mode)
	}
	return nil
}

func (file *failingTemporaryFile) Sync() error {
	if file.syncErr != nil {
		return file.syncErr
	}
	if file.backing != nil {
		return file.backing.Sync()
	}
	return nil
}

func (file *failingTemporaryFile) Close() error {
	var backingErr error
	if file.backing != nil {
		backingErr = file.backing.Close()
	}
	return errors.Join(file.closeErr, backingErr)
}

type tarEntry struct {
	name    string
	content string
}

func writeTarEntries(t *testing.T, archivePath string, entries []tarEntry) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzipWriterForTest(t, file)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		if err := tarWriter.WriteHeader(&tar.Header{Name: entry.name, Mode: 0o755, Size: int64(len(entry.content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tarWriter, entry.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func useDownloadClient(t *testing.T, client *http.Client) {
	t.Helper()
	original := downloadHTTPClient
	downloadHTTPClient = client
	t.Cleanup(func() { downloadHTTPClient = original })
}

func assertNoDownloadTemps(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".andurel-") {
			t.Fatalf("temporary data remains: %s", entry.Name())
		}
	}
}

func gzipWriterForTest(t *testing.T, writer io.Writer) io.WriteCloser {
	t.Helper()
	gzipWriter := gzip.NewWriter(writer)
	if gzipWriter == nil {
		t.Fatal(fmt.Errorf("failed to create gzip writer"))
	}
	return gzipWriter
}
