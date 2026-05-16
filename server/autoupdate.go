package server

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/65658dsf/StellarCore/pkg/util/log"
	"github.com/65658dsf/StellarCore/pkg/util/version"
)

const (
	coreAutoUpdateInterval = 24 * time.Hour
	coreUpdateAPIURL       = "https://resources.stellarfrp.top/api/fs/list"
	coreUpdateDownloadURL  = "https://resources.stellarfrp.top/d/StellarCore"
	coreUpdateRootPath     = "StellarCore"
)

type coreAutoUpdater struct {
	httpClient      *http.Client
	apiURL          string
	downloadBaseURL string
	rootPath        string
	goos            string
	goarch          string
	executablePath  string
	currentVersion  string
	restartExecutor RestartExecutor
}

type resourceListResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Content []resourceEntry `json:"content"`
	} `json:"data"`
}

type resourceEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}

type coreUpdateCandidate struct {
	Version     string
	AssetName   string
	DownloadURL string
}

type coreUpdateCheckResult struct {
	CurrentVersion string
	Candidate      *coreUpdateCandidate
	HasUpdate      bool
}

type coreReleaseAsset struct {
	Name        string
	Version     string
	ArchiveType string
}

func newCoreAutoUpdater(restartExecutor RestartExecutor) *coreAutoUpdater {
	return &coreAutoUpdater{
		httpClient:      &http.Client{Timeout: time.Minute},
		apiURL:          coreUpdateAPIURL,
		downloadBaseURL: coreUpdateDownloadURL,
		rootPath:        coreUpdateRootPath,
		goos:            runtime.GOOS,
		goarch:          runtime.GOARCH,
		currentVersion:  version.Core(),
		restartExecutor: restartExecutor,
	}
}

func (svr *Service) runAutoUpdateWorker() {
	if !autoUpdateSupportedGOOS(runtime.GOOS) {
		log.Infof("当前平台 %s 不支持 StellarCore 自动更新，已跳过", runtime.GOOS)
		return
	}
	if svr.restartExecutor == nil {
		log.Infof("当前平台不支持进程自重启，StellarCore 自动更新已跳过")
		return
	}

	updater := newCoreAutoUpdater(svr.restartAfterUpdate)
	ticker := time.NewTicker(coreAutoUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-svr.ctx.Done():
			return
		case <-ticker.C:
			if !svr.updatePending.CompareAndSwap(false, true) {
				log.Infof("StellarCore 更新任务已在执行，跳过本次自动检查")
				continue
			}
			if err := updater.CheckAndUpdate(svr.ctx); err != nil {
				log.Warnf("StellarCore 自动更新检查失败: %v", err)
			}
			svr.updatePending.Store(false)
		}
	}
}

func (u *coreAutoUpdater) CheckAndUpdate(ctx context.Context) error {
	if !autoUpdateSupportedGOOS(u.goos) {
		log.Infof("当前平台 %s 不支持 StellarCore 自动更新，已跳过", u.goos)
		return nil
	}
	if u.restartExecutor == nil {
		return fmt.Errorf("restart executor is not available")
	}

	result, err := u.CheckLatest(ctx)
	if err != nil {
		return err
	}
	if !result.HasUpdate {
		log.Infof("StellarCore 当前版本 %s 已是最新版本", result.CurrentVersion)
		return nil
	}

	log.Infof("发现 StellarCore 新版本 %s，当前版本 %s，开始自动更新", result.Candidate.Version, result.CurrentVersion)
	return u.downloadInstallAndRestart(ctx, result.Candidate)
}

func (u *coreAutoUpdater) CheckLatest(ctx context.Context) (coreUpdateCheckResult, error) {
	currentVersion := u.currentVersion
	if currentVersion == "" {
		currentVersion = version.Core()
	}
	result := coreUpdateCheckResult{
		CurrentVersion: currentVersion,
	}

	candidate, err := u.findLatestRelease(ctx)
	if err != nil {
		return result, err
	}
	result.Candidate = candidate
	result.HasUpdate = isVersionGreater(candidate.Version, currentVersion)
	return result, nil
}

func (u *coreAutoUpdater) findLatestRelease(ctx context.Context) (*coreUpdateCandidate, error) {
	arch, ok := stellarCoreArch(u.goarch)
	if !ok {
		return nil, fmt.Errorf("unsupported architecture %s", u.goarch)
	}

	sources, err := u.listResources(ctx, u.rootPath)
	if err != nil {
		return nil, err
	}

	var best *coreUpdateCandidate
	for _, source := range sources {
		if !source.IsDir || source.Name == "" {
			continue
		}

		sourcePath := path.Join(u.rootPath, source.Name)
		sourceEntries, err := u.listResources(ctx, sourcePath)
		if err != nil {
			log.Warnf("获取自动更新源 %s 失败: %v", source.Name, err)
			continue
		}
		if !containsDir(sourceEntries, "frps") {
			continue
		}

		versionPath := path.Join(sourcePath, "frps")
		versionEntries, err := u.listResources(ctx, versionPath)
		if err != nil {
			log.Warnf("获取自动更新版本目录 %s 失败: %v", versionPath, err)
			continue
		}
		versionNames := validVersionDirs(versionEntries)
		sort.Slice(versionNames, func(i, j int) bool {
			return compareSemanticVersion(versionNames[i], versionNames[j]) > 0
		})

		for _, versionName := range versionNames {
			filesPath := path.Join(versionPath, versionName)
			files, err := u.listResources(ctx, filesPath)
			if err != nil {
				log.Warnf("获取自动更新文件目录 %s 失败: %v", filesPath, err)
				continue
			}

			asset, ok := selectCoreReleaseAsset(files, u.goos, arch)
			if !ok {
				continue
			}

			candidate := &coreUpdateCandidate{
				Version:     asset.Version,
				AssetName:   asset.Name,
				DownloadURL: joinDownloadURL(u.downloadBaseURL, source.Name, "frps", versionName, asset.Name),
			}
			if best == nil || compareSemanticVersion(candidate.Version, best.Version) > 0 {
				best = candidate
			}
			break
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no matching StellarCore update package found for %s/%s", u.goos, arch)
	}
	return best, nil
}

func (u *coreAutoUpdater) listResources(ctx context.Context, resourcePath string) ([]resourceEntry, error) {
	apiURL, err := url.Parse(u.apiURL)
	if err != nil {
		return nil, err
	}
	query := apiURL.Query()
	query.Set("path", resourcePath)
	apiURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL.String(), nil)
	if err != nil {
		return nil, err
	}

	client := u.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("resource api returned status %d", resp.StatusCode)
	}

	var body resourceListResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil {
		return nil, err
	}
	if body.Code != 0 && body.Code != http.StatusOK {
		return nil, fmt.Errorf("resource api returned code %d: %s", body.Code, body.Message)
	}
	return body.Data.Content, nil
}

func (u *coreAutoUpdater) downloadInstallAndRestart(ctx context.Context, candidate *coreUpdateCandidate) error {
	executablePath := u.executablePath
	if executablePath == "" {
		path, err := os.Executable()
		if err != nil {
			return err
		}
		executablePath = path
	}

	tempDir, err := os.MkdirTemp(filepath.Dir(executablePath), ".stellarcore-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, candidate.AssetName)
	if err := u.downloadFile(ctx, candidate.DownloadURL, archivePath); err != nil {
		return err
	}

	extractDir := filepath.Join(tempDir, "extract")
	if err := extractArchive(archivePath, extractDir); err != nil {
		return err
	}

	binaryPath, err := findExpectedBinary(extractDir)
	if err != nil {
		return err
	}

	if err := replaceExecutableAndRestart(executablePath, binaryPath, u.restartExecutor); err != nil {
		return err
	}
	log.Infof("StellarCore 自动更新已安装版本 %s，正在重启", candidate.Version)
	return nil
}

func (u *coreAutoUpdater) downloadFile(ctx context.Context, downloadURL string, outputPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}

	client := u.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	out, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return fmt.Errorf("downloaded update package is empty")
	}
	return nil
}

func replaceExecutableAndRestart(executablePath string, binaryPath string, restartExecutor RestartExecutor) error {
	if restartExecutor == nil {
		return fmt.Errorf("restart executor is not available")
	}

	info, err := os.Stat(executablePath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(executablePath)
	base := filepath.Base(executablePath)
	newPath := filepath.Join(dir, "."+base+".new")
	backupPath := filepath.Join(dir, "."+base+".bak")
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o755
	}

	if err := copyFile(binaryPath, newPath, mode); err != nil {
		return err
	}
	defer os.Remove(newPath)

	_ = os.Remove(backupPath)
	if runtime.GOOS == "windows" {
		if err := os.Rename(executablePath, backupPath); err != nil {
			return err
		}
		if err := os.Rename(newPath, executablePath); err != nil {
			_ = os.Rename(backupPath, executablePath)
			return err
		}
	} else {
		if err := copyFile(executablePath, backupPath, mode); err != nil {
			return err
		}
		if err := os.Rename(newPath, executablePath); err != nil {
			return err
		}
	}

	if err := restartExecutor(); err != nil {
		_ = os.Remove(executablePath)
		_ = os.Rename(backupPath, executablePath)
		return fmt.Errorf("restart after update failed: %w", err)
	}
	return nil
}

func copyFile(src string, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(dst, mode)
}

func extractArchive(archivePath string, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}

	switch {
	case strings.HasSuffix(archivePath, ".tar.gz"):
		return extractTarGz(archivePath, targetDir)
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZip(archivePath, targetDir)
	default:
		return fmt.Errorf("unsupported update package format: %s", filepath.Base(archivePath))
	}
}

func extractTarGz(archivePath string, targetDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		cleanName, err := safeArchiveName(header.Name)
		if err != nil {
			return err
		}
		targetPath, err := safeJoinArchivePath(targetDir, cleanName)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, header.FileInfo().Mode().Perm()); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			mode := header.FileInfo().Mode().Perm()
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, reader)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			if err := os.Chmod(targetPath, mode); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry type for %s", header.Name)
		}
	}
}

func extractZip(archivePath string, targetDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		cleanName, err := safeArchiveName(file.Name)
		if err != nil {
			return err
		}
		targetPath, err := safeJoinArchivePath(targetDir, cleanName)
		if err != nil {
			return err
		}

		info := file.FileInfo()
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsupported archive symlink entry: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, info.Mode().Perm()); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported archive entry type for %s", file.Name)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeInErr := in.Close()
		closeOutErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeInErr != nil {
			return closeInErr
		}
		if closeOutErr != nil {
			return closeOutErr
		}
		if err := os.Chmod(targetPath, mode); err != nil {
			return err
		}
	}
	return nil
}

func safeArchiveName(name string) (string, error) {
	if name == "" || strings.Contains(name, `\`) {
		return "", fmt.Errorf("unsafe archive path: %s", name)
	}
	if path.IsAbs(name) || filepath.IsAbs(name) {
		return "", fmt.Errorf("unsafe archive path: %s", name)
	}

	cleanName := path.Clean(name)
	if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, "../") {
		return "", fmt.Errorf("unsafe archive path: %s", name)
	}
	return cleanName, nil
}

func safeJoinArchivePath(targetDir string, cleanName string) (string, error) {
	targetPath := filepath.Join(targetDir, filepath.FromSlash(cleanName))
	rel, err := filepath.Rel(targetDir, targetPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe archive path: %s", cleanName)
	}
	return targetPath, nil
}

func findExpectedBinary(searchDir string) (string, error) {
	var binaryPath string
	err := filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if name != "StellarCore" && name != "frps" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		binaryPath = path
		return filepath.SkipAll
	})
	if err != nil {
		return "", err
	}
	if binaryPath == "" {
		return "", fmt.Errorf("update package does not contain StellarCore or frps binary")
	}
	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return "", err
	}
	return binaryPath, nil
}

func containsDir(entries []resourceEntry, name string) bool {
	for _, entry := range entries {
		if entry.IsDir && entry.Name == name {
			return true
		}
	}
	return false
}

func validVersionDirs(entries []resourceEntry) []string {
	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir || !semanticVersionValid(entry.Name) {
			continue
		}
		versions = append(versions, normalizeSemanticVersion(entry.Name))
	}
	return versions
}

func selectCoreReleaseAsset(entries []resourceEntry, goos string, arch string) (coreReleaseAsset, bool) {
	pattern := regexp.MustCompile(`^StellarCore_(.+)_` + regexp.QuoteMeta(goos) + `_` + regexp.QuoteMeta(arch) + `\.(tar\.gz|zip)$`)
	var best coreReleaseAsset
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		matches := pattern.FindStringSubmatch(entry.Name)
		if len(matches) != 3 || !semanticVersionValid(matches[1]) {
			continue
		}
		asset := coreReleaseAsset{
			Name:        entry.Name,
			Version:     normalizeSemanticVersion(matches[1]),
			ArchiveType: matches[2],
		}
		if best.Name == "" ||
			compareSemanticVersion(asset.Version, best.Version) > 0 ||
			(compareSemanticVersion(asset.Version, best.Version) == 0 && archivePreference(asset.ArchiveType) > archivePreference(best.ArchiveType)) {
			best = asset
		}
	}
	return best, best.Name != ""
}

func archivePreference(archiveType string) int {
	if archiveType == "tar.gz" {
		return 2
	}
	if archiveType == "zip" {
		return 1
	}
	return 0
}

func stellarCoreArch(goarch string) (string, bool) {
	switch goarch {
	case "amd64":
		return "amd64", true
	case "arm64":
		return "arm64", true
	case "386":
		return "386", true
	case "arm":
		return "arm32v7", true
	default:
		return "", false
	}
}

func joinDownloadURL(base string, segments ...string) string {
	out := strings.TrimRight(base, "/")
	for _, segment := range segments {
		out += "/" + url.PathEscape(segment)
	}
	return out
}

func autoUpdateSupportedGOOS(goos string) bool {
	switch goos {
	case "linux", "darwin", "freebsd", "openbsd", "netbsd", "dragonfly", "illumos", "solaris", "aix":
		return true
	default:
		return false
	}
}

func isVersionGreater(remote string, current string) bool {
	return compareSemanticVersion(remote, current) > 0
}

func compareSemanticVersion(a string, b string) int {
	aParts, aOK := parseSemanticVersion(a)
	bParts, bOK := parseSemanticVersion(b)
	if !aOK || !bOK {
		return strings.Compare(a, b)
	}

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}
	for i := 0; i < maxLen; i++ {
		aPart := 0
		if i < len(aParts) {
			aPart = aParts[i]
		}
		bPart := 0
		if i < len(bParts) {
			bPart = bParts[i]
		}
		switch {
		case aPart > bPart:
			return 1
		case aPart < bPart:
			return -1
		}
	}
	return 0
}

func semanticVersionValid(v string) bool {
	_, ok := parseSemanticVersion(v)
	return ok
}

func normalizeSemanticVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

func parseSemanticVersion(v string) ([]int, bool) {
	v = normalizeSemanticVersion(v)
	parts := strings.Split(v, ".")
	if len(parts) == 0 {
		return nil, false
	}

	out := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return nil, false
		}
		out = append(out, value)
	}
	return out, true
}
