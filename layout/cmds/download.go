package cmds

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	maxArchiveSize      int64 = 256 << 20
	maxBinarySize       int64 = 256 << 20
	connectionTimeout         = 10 * time.Second
	totalRequestTimeout       = 2 * time.Minute
)

// ToolDownloader represents tool downloader.
type ToolDownloader struct {
	Name    string
	Module  string
	Version string
}

// ErrFailedToGetRleaseURL is returned when failed to get release URL.
var ErrFailedToGetRleaseURL = fmt.Errorf("failed to get release URL")

type temporaryFile interface {
	io.Writer
	Name() string
	Chmod(os.FileMode) error
	Close() error
	Sync() error
}

var (
	downloadDialer       = &net.Dialer{Timeout: connectionTimeout}
	downloadHTTPClient   = newDownloadHTTPClient()
	downloadVerifiedFunc = downloadVerified
	createTempFileFunc   = func(dir, pattern string) (temporaryFile, error) {
		return os.CreateTemp(dir, pattern)
	}
	renameFileFunc = os.Rename
	removeFileFunc = os.Remove
)

func newDownloadHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:       http.ProxyFromEnvironment,
			DialContext: downloadDialer.DialContext,
		},
		CheckRedirect: requireHTTPSRedirect,
		Timeout:       totalRequestTimeout,
	}
}

func requireHTTPSRedirect(request *http.Request, _ []*http.Request) error {
	if request.URL.Scheme != "https" {
		return fmt.Errorf("download redirect must use HTTPS")
	}
	return nil
}

// DownloadFromURLTemplate rejects downloads without integrity metadata.
// Use DownloadVerifiedFromURLTemplate to provide the required SHA-256 digest.
func DownloadFromURLTemplate(
	name,
	version,
	urlTemplate,
	archiveType,
	binaryName,
	goos,
	goarch,
	destPath string,
) error {
	return DownloadVerifiedFromURLTemplate(
		name,
		version,
		urlTemplate,
		archiveType,
		binaryName,
		goos,
		goarch,
		destPath,
		"",
	)
}

// DownloadVerifiedFromURLTemplate downloads from a URL template after
// verifying the expected SHA-256 digest.
func DownloadVerifiedFromURLTemplate(
	name,
	version,
	urlTemplate,
	archiveType,
	binaryName,
	goos,
	goarch,
	destPath,
	expectedSHA256 string,
) error {
	if urlTemplate == "" {
		return fmt.Errorf("download urlTemplate is required for %s", name)
	}

	digest, err := requireExpectedDigest(expectedSHA256)
	if err != nil {
		return err
	}
	resolvedURL := renderDownloadURL(urlTemplate, version, goos, goarch)
	if archiveType == "" {
		archiveType = "binary"
	}
	if binaryName == "" {
		binaryName = name
	}

	return downloadVerifiedFunc(name, resolvedURL, archiveType, binaryName, destPath, digest)
}

// DownloadFromURL rejects downloads without integrity metadata.
// Use DownloadVerifiedFromURL to provide the required SHA-256 digest.
func DownloadFromURL(name, sourceURL, archiveType, binaryName, destPath string) error {
	return DownloadVerifiedFromURL(name, sourceURL, archiveType, binaryName, destPath, "")
}

// DownloadVerifiedFromURL downloads from a URL after verifying the expected
// SHA-256 digest.
func DownloadVerifiedFromURL(name, sourceURL, archiveType, binaryName, destPath, expectedSHA256 string) error {
	digest, err := requireExpectedDigest(expectedSHA256)
	if err != nil {
		return err
	}
	return downloadVerifiedFunc(name, sourceURL, archiveType, binaryName, destPath, digest)
}

func requireExpectedDigest(value string) (string, error) {
	if !validSHA256(value) {
		return "", fmt.Errorf("a valid SHA-256 digest is required")
	}
	return strings.ToLower(value), nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func downloadVerified(name, sourceURL, archiveType, binaryName, destPath, expectedDigest string) (resultErr error) {
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("download URL for %s must use HTTPS", name)
	}
	if binaryName == "" || path.Base(strings.ReplaceAll(binaryName, "\\", "/")) != binaryName {
		return fmt.Errorf("binary name must be a file name")
	}

	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	archiveFile, err := createTempFileFunc(destDir, ".andurel-archive-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary archive: %w", err)
	}
	archivePath := archiveFile.Name()
	archivePresent := true
	defer func() {
		if archivePresent {
			if err := removeFileFunc(archivePath); err != nil && !os.IsNotExist(err) {
				resultErr = errors.Join(resultErr, fmt.Errorf("failed to remove temporary archive: %w", err))
			}
		}
	}()

	actualDigest, err := downloadToTemporaryFile(sourceURL, archiveFile)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", name, err)
	}
	if actualDigest != expectedDigest {
		return fmt.Errorf("SHA-256 mismatch for %s: expected %s, got %s", name, expectedDigest, actualDigest)
	}

	if archiveType == "binary" {
		if err := os.Chmod(archivePath, 0o755); err != nil {
			return fmt.Errorf("failed to set executable permissions: %w", err)
		}
		if err := renameFileFunc(archivePath, destPath); err != nil {
			return fmt.Errorf("failed to atomically install %s: %w", name, err)
		}
		archivePresent = false
		return nil
	}

	extractedFile, err := createTempFileFunc(destDir, ".andurel-binary-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary binary: %w", err)
	}
	extractedPath := extractedFile.Name()
	extractedPresent := true
	defer func() {
		if extractedPresent {
			if err := removeFileFunc(extractedPath); err != nil && !os.IsNotExist(err) {
				resultErr = errors.Join(resultErr, fmt.Errorf("failed to remove temporary binary: %w", err))
			}
		}
	}()

	if err := extractArchiveToFile(archivePath, binaryName, archiveType, extractedFile); err != nil {
		return fmt.Errorf("failed to extract %s: %w", name, errors.Join(err, extractedFile.Close()))
	}
	if err := extractedFile.Chmod(0o755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", errors.Join(err, extractedFile.Close()))
	}
	if err := extractedFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync extracted binary: %w", errors.Join(err, extractedFile.Close()))
	}
	if err := extractedFile.Close(); err != nil {
		return fmt.Errorf("failed to close extracted binary: %w", err)
	}
	if err := removeFileFunc(archivePath); err != nil {
		return fmt.Errorf("failed to remove temporary archive: %w", err)
	}
	archivePresent = false
	if err := renameFileFunc(extractedPath, destPath); err != nil {
		return fmt.Errorf("failed to atomically install %s: %w", name, err)
	}
	extractedPresent = false
	return nil
}

func downloadToTemporaryFile(sourceURL string, file temporaryFile) (string, error) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", errors.Join(err, file.Close()))
	}
	response, err := downloadHTTPClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", errors.Join(err, file.Close()))
	}
	if response.StatusCode != http.StatusOK {
		closeErr := response.Body.Close()
		fileCloseErr := file.Close()
		if closeErr != nil {
			return "", fmt.Errorf("unexpected status code %d and failed to close response: %w", response.StatusCode, errors.Join(closeErr, fileCloseErr))
		}
		if fileCloseErr != nil {
			return "", fmt.Errorf("unexpected status code %d and failed to close temporary archive: %w", response.StatusCode, fileCloseErr)
		}
		return "", fmt.Errorf("unexpected status code %d for %s", response.StatusCode, sourceURL)
	}

	hash := sha256.New()
	_, copyErr := copyBounded(io.MultiWriter(file, hash), response.Body, maxArchiveSize, "archive")
	bodyCloseErr := response.Body.Close()
	if copyErr != nil {
		return "", errors.Join(copyErr, bodyCloseErr, file.Close())
	}
	if bodyCloseErr != nil {
		return "", fmt.Errorf("failed to close response body: %w", errors.Join(bodyCloseErr, file.Close()))
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("failed to sync temporary archive: %w", errors.Join(err, file.Close()))
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("failed to close temporary archive: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func copyBounded(dst io.Writer, src io.Reader, limit int64, description string) (int64, error) {
	written, err := io.Copy(dst, io.LimitReader(src, limit+1))
	if err != nil {
		return written, fmt.Errorf("failed to read or write %s: %w", description, err)
	}
	if written > limit {
		return written, fmt.Errorf("%s exceeds maximum size of %d bytes", description, limit)
	}
	return written, nil
}

func renderDownloadURL(urlTemplate, version, goos, goarch string) string {
	replacer := strings.NewReplacer(
		"{{version}}", version,
		"{{version_no_v}}", strings.TrimPrefix(version, "v"),
		"{{os}}", goos,
		"{{arch}}", goarch,
		"{{os_capitalized}}", capitalize(goos),
		"{{arch_x86_64}}", mapArch(goarch),
		"{{os_tailwind}}", normalizeTailwindOS(goos),
		"{{arch_tailwind}}", normalizeTailwindArch(goarch),
	)
	return replacer.Replace(urlTemplate)
}

func normalizeTailwindOS(goos string) string {
	if goos == "darwin" {
		return "macos"
	}
	return goos
}

func normalizeTailwindArch(goarch string) string {
	if goarch == "amd64" {
		return "x64"
	}
	return goarch
}

// DownloadGoTool rejects downloads without integrity metadata.
// Use DownloadVerifiedGoTool to provide the required SHA-256 digest.
func DownloadGoTool(name, module, version, goos, goarch, destPath string) error {
	return DownloadVerifiedGoTool(name, module, version, goos, goarch, destPath, "")
}

// DownloadVerifiedGoTool downloads a Go tool release asset after verifying the
// expected SHA-256 digest.
func DownloadVerifiedGoTool(name, module, version, goos, goarch, destPath, expectedSHA256 string) error {
	downloader := &ToolDownloader{Name: name, Module: module, Version: version}
	resolvedURL, archiveType, err := downloader.getReleaseURL(goos, goarch)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrFailedToGetRleaseURL, err)
	}
	return DownloadVerifiedFromURL(name, resolvedURL, archiveType, name, destPath, expectedSHA256)
}

func (d *ToolDownloader) getReleaseURL(goos, goarch string) (string, string, error) {
	repo := extractGitHubRepo(d.Module)
	switch d.Name {
	case "templ":
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/templ_%s_%s.tar.gz", repo, d.Version, capitalize(goos), mapArch(goarch)), "tar.gz", nil
	case "goose":
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/goose_%s_%s", repo, d.Version, goos, mapArch(goarch)), "binary", nil
	case "mailpit":
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/mailpit-%s-%s.tar.gz", repo, d.Version, goos, goarch), "tar.gz", nil
	case "usql":
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/usql-%s-%s-%s.tar.bz2", repo, d.Version, strings.TrimPrefix(d.Version, "v"), goos, goarch), "tar.bz2", nil
	case "dblab":
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/dblab_%s_%s_%s.tar.gz", repo, d.Version, strings.TrimPrefix(d.Version, "v"), goos, goarch), "tar.gz", nil
	case "shadowfax":
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/shadowfax-%s-%s", repo, d.Version, goos, goarch), "binary", nil
	default:
		return "", "", fmt.Errorf("unknown tool: %s", d.Name)
	}
}

// DownloadTailwindCLI rejects downloads without integrity metadata.
// Use DownloadVerifiedTailwindCLI to provide the required SHA-256 digest.
func DownloadTailwindCLI(version, goos, goarch, destPath string) error {
	return DownloadVerifiedTailwindCLI(version, goos, goarch, destPath, "")
}

// DownloadVerifiedTailwindCLI downloads the Tailwind CLI after verifying the
// expected SHA-256 digest.
func DownloadVerifiedTailwindCLI(version, goos, goarch, destPath, expectedSHA256 string) error {
	resolvedURL := fmt.Sprintf(
		"https://github.com/tailwindlabs/tailwindcss/releases/download/%s/tailwindcss-%s-%s",
		version,
		normalizeTailwindOS(goos),
		normalizeTailwindArch(goarch),
	)
	if err := DownloadVerifiedFromURL("tailwindcli", resolvedURL, "binary", "tailwindcli", destPath, expectedSHA256); err != nil {
		return fmt.Errorf("failed to download tailwindcli: %w", err)
	}
	return nil
}

func extractArchiveToFile(archivePath, binaryName, archiveType string, destination io.Writer) error {
	switch archiveType {
	case "tar.gz":
		return extractTarGzTo(archivePath, binaryName, destination)
	case "tar.bz2":
		return extractTarBz2To(archivePath, binaryName, destination)
	case "zip":
		return extractZipTo(archivePath, binaryName, destination)
	default:
		return fmt.Errorf("unsupported archive type: %s", archiveType)
	}
}

func extractTarGzTo(archivePath, binaryName string, destination io.Writer) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	reader, err := gzip.NewReader(archive)
	if err != nil {
		_ = archive.Close()
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	extractErr := extractTarEntry(tar.NewReader(reader), binaryName, destination)
	gzipCloseErr := reader.Close()
	archiveCloseErr := archive.Close()
	if extractErr != nil {
		return errors.Join(extractErr, gzipCloseErr, archiveCloseErr)
	}
	if gzipCloseErr != nil {
		return fmt.Errorf("failed to close gzip reader: %w", gzipCloseErr)
	}
	if archiveCloseErr != nil {
		return fmt.Errorf("failed to close archive: %w", archiveCloseErr)
	}
	return nil
}

func extractTarBz2To(archivePath, binaryName string, destination io.Writer) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	extractErr := extractTarEntry(tar.NewReader(bzip2.NewReader(archive)), binaryName, destination)
	closeErr := archive.Close()
	if extractErr != nil {
		return errors.Join(extractErr, closeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close archive: %w", closeErr)
	}
	return nil
}

func extractTarEntry(reader *tar.Reader, binaryName string, destination io.Writer) error {
	matches := 0
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}
		cleanName, err := validateArchivePath(header.Name)
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg || path.Base(cleanName) != binaryName {
			continue
		}
		matches++
		if matches > 1 {
			return fmt.Errorf("archive contains multiple files named %s", binaryName)
		}
		if header.Size < 0 || header.Size > maxBinarySize {
			return fmt.Errorf("extracted binary exceeds maximum size of %d bytes", maxBinarySize)
		}
		written, err := copyBounded(destination, reader, maxBinarySize, "extracted binary")
		if err != nil {
			return err
		}
		if written != header.Size {
			return fmt.Errorf("extracted binary is truncated: expected %d bytes, got %d", header.Size, written)
		}
	}
	if matches == 0 {
		return fmt.Errorf("binary %s not found in archive", binaryName)
	}
	return nil
}

func extractZipTo(archivePath, binaryName string, destination io.Writer) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	var match *zip.File
	for _, file := range reader.File {
		cleanName, err := validateArchivePath(file.Name)
		if err != nil {
			return errors.Join(err, reader.Close())
		}
		if file.FileInfo().Mode().IsRegular() && path.Base(cleanName) == binaryName {
			if match != nil {
				return errors.Join(fmt.Errorf("archive contains multiple files named %s", binaryName), reader.Close())
			}
			match = file
		}
	}
	if match == nil {
		return errors.Join(fmt.Errorf("binary %s not found in zip", binaryName), reader.Close())
	}
	if match.UncompressedSize64 > uint64(maxBinarySize) {
		return errors.Join(fmt.Errorf("extracted binary exceeds maximum size of %d bytes", maxBinarySize), reader.Close())
	}
	entry, err := match.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in zip: %w", errors.Join(err, reader.Close()))
	}
	written, copyErr := copyBounded(destination, entry, maxBinarySize, "extracted binary")
	entryCloseErr := entry.Close()
	zipCloseErr := reader.Close()
	if copyErr != nil {
		return copyErr
	}
	if written != int64(match.UncompressedSize64) {
		return fmt.Errorf("extracted binary is truncated: expected %d bytes, got %d", match.UncompressedSize64, written)
	}
	if entryCloseErr != nil {
		return fmt.Errorf("failed to close zip entry: %w", entryCloseErr)
	}
	if zipCloseErr != nil {
		return fmt.Errorf("failed to close zip: %w", zipCloseErr)
	}
	return nil
}

func validateArchivePath(name string) (string, error) {
	normalized := strings.ReplaceAll(name, "\\", "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	for segment := range strings.SplitSeq(normalized, "/") {
		if segment == ".." {
			return "", fmt.Errorf("unsafe archive path %q", name)
		}
	}
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return cleaned, nil
}

func extractBinary(archivePath, binaryName, destPath, archiveType string) error {
	if archiveType == "binary" {
		return copyFile(archivePath, destPath)
	}
	destination, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	extractErr := extractArchiveToFile(archivePath, binaryName, archiveType, destination)
	closeErr := destination.Close()
	if extractErr != nil {
		return extractErr
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close output file: %w", closeErr)
	}
	return nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	destination, err := os.Create(dst)
	if err != nil {
		_ = source.Close()
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	_, copyErr := copyBounded(destination, source, maxBinarySize, "binary")
	sourceCloseErr := source.Close()
	destinationCloseErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	if sourceCloseErr != nil {
		return fmt.Errorf("failed to close source file: %w", sourceCloseErr)
	}
	if destinationCloseErr != nil {
		return fmt.Errorf("failed to close destination file: %w", destinationCloseErr)
	}
	return nil
}

func extractTarGz(archivePath, binaryName, destPath string) error {
	return extractBinary(archivePath, binaryName, destPath, "tar.gz")
}

func extractZip(archivePath, binaryName, destPath string) error {
	return extractBinary(archivePath, binaryName, destPath, "zip")
}

func capitalize(value string) string {
	if len(value) == 0 {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func mapArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	case "386":
		return "i386"
	default:
		return goarch
	}
}

// GetPlatform returns platform.
func GetPlatform() (string, string) {
	return runtime.GOOS, runtime.GOARCH
}

func extractGitHubRepo(module string) string {
	module = strings.TrimPrefix(module, "github.com/")
	parts := strings.Split(module, "/")
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return module
}
