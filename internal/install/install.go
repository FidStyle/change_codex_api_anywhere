package install

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Result struct {
	SourcePath  string
	TargetPath  string
	PathUpdated bool
	Notes       []string
}

func Run() (*Result, error) {
	sourcePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve current executable: %w", err)
	}

	sourcePath, err = filepath.EvalSymlinks(sourcePath)
	if err != nil {
		sourcePath = filepath.Clean(sourcePath)
	}

	if runtime.GOOS == "windows" {
		return installWindows(sourcePath)
	}

	return installUnix(sourcePath)
}

func installUnix(sourcePath string) (*Result, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	targetDir, notes := selectUnixTargetDir(strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)), home, unixDirWritable)
	targetPath := filepath.Join(targetDir, filepath.Base(sourcePath))

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, fmt.Errorf("create install directory %s: %w", targetDir, err)
	}

	if err := copyFile(sourcePath, targetPath); err != nil {
		return nil, err
	}

	return &Result{
		SourcePath: sourcePath,
		TargetPath: targetPath,
		Notes:      notes,
	}, nil
}

func installWindows(sourcePath string) (*Result, error) {
	targetDir, err := selectWindowsTargetDir()
	if err != nil {
		return nil, err
	}

	targetPath := filepath.Join(targetDir, filepath.Base(sourcePath))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, fmt.Errorf("create install directory %s: %w", targetDir, err)
	}

	if err := copyFile(sourcePath, targetPath); err != nil {
		return nil, err
	}

	pathUpdated, err := ensureWindowsUserPath(targetDir)
	if err != nil {
		return nil, err
	}

	result := &Result{
		SourcePath:  sourcePath,
		TargetPath:  targetPath,
		PathUpdated: pathUpdated,
	}
	if pathUpdated {
		result.Notes = append(result.Notes, "user PATH updated; reopen the terminal to use the new command")
	}

	return result, nil
}

func selectUnixTargetDir(pathEntries []string, home string, canWrite func(string) bool) (string, []string) {
	candidates := []string{"/usr/local/bin", "/usr/bin", "/bin"}
	notes := []string{}

	for _, candidate := range candidates {
		if pathContains(pathEntries, candidate) && canWrite(candidate) {
			return candidate, notes
		}
	}

	userBin := filepath.Join(home, ".local", "bin")
	notes = append(notes, "no writable system PATH directory found; using ~/.local/bin")
	if !pathContains(pathEntries, userBin) {
		notes = append(notes, "add ~/.local/bin to PATH if your shell does not already include it")
	}

	return userBin, notes
}

func unixDirWritable(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}

	testFile, err := os.CreateTemp(dir, ".ccaa-install-*.tmp")
	if err != nil {
		return false
	}

	testPath := testFile.Name()
	_ = testFile.Close()
	_ = os.Remove(testPath)
	return true
}

func selectWindowsTargetDir() (string, error) {
	if localAppData := strings.TrimSpace(os.Getenv("LocalAppData")); localAppData != "" {
		return filepath.Join(localAppData, "ccaa", "bin"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(home, "AppData", "Local", "ccaa", "bin"), nil
}

func ensureWindowsUserPath(targetDir string) (bool, error) {
	currentPath := os.Getenv("PATH")
	if pathContainsWithSeparator(currentPath, targetDir, ';', strings.EqualFold) {
		return false, nil
	}

	script := buildPowerShellPathScript(targetDir)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("update user PATH via PowerShell: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return true, nil
}

func buildPowerShellPathScript(targetDir string) string {
	quotedDir := singleQuotePowerShell(targetDir)
	return "$dir = " + quotedDir + "; " +
		"$current = [Environment]::GetEnvironmentVariable('Path', 'User'); " +
		"if ([string]::IsNullOrWhiteSpace($current)) { " +
		"  $next = $dir " +
		"} else { " +
		"  $parts = $current -split ';' | Where-Object { $_ -and $_.Trim() -ne '' }; " +
		"  if ($parts -contains $dir) { exit 0 }; " +
		"  $next = (($parts + $dir) -join ';') " +
		"}; " +
		"[Environment]::SetEnvironmentVariable('Path', $next, 'User')"
}

func singleQuotePowerShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func copyFile(sourcePath string, targetPath string) error {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat source executable %s: %w", sourcePath, err)
	}

	if sameFile(sourcePath, targetPath) {
		return nil
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source executable %s: %w", sourcePath, err)
	}
	defer sourceFile.Close()

	tmpFile, err := os.CreateTemp(filepath.Dir(targetPath), ".ccaa-install-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp install file for %s: %w", targetPath, err)
	}

	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := io.Copy(tmpFile, sourceFile); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("copy executable to %s: %w", targetPath, err)
	}

	if err := tmpFile.Chmod(sourceInfo.Mode().Perm()); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("set executable mode for %s: %w", targetPath, err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp install file for %s: %w", targetPath, err)
	}

	if err := replaceFile(tmpPath, targetPath); err != nil {
		return fmt.Errorf("replace installed executable %s: %w", targetPath, err)
	}

	return nil
}

func replaceFile(src string, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.Rename(src, dst)
}

func sameFile(sourcePath string, targetPath string) bool {
	cleanSource := filepath.Clean(sourcePath)
	cleanTarget := filepath.Clean(targetPath)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(cleanSource, cleanTarget)
	}

	return cleanSource == cleanTarget
}

func pathContains(entries []string, target string) bool {
	for _, entry := range entries {
		if filepath.Clean(strings.TrimSpace(entry)) == filepath.Clean(target) {
			return true
		}
	}

	return false
}

func pathContainsWithSeparator(pathValue string, target string, separator rune, equals func(string, string) bool) bool {
	for _, entry := range strings.Split(pathValue, string(separator)) {
		cleanEntry := filepath.Clean(strings.TrimSpace(entry))
		if cleanEntry == "." || cleanEntry == "" {
			continue
		}

		if equals(cleanEntry, filepath.Clean(target)) {
			return true
		}
	}

	return false
}
