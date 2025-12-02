package updater

import (
	"archive/tar"
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/ulikunitz/xz"

	"github.com/highercomve/tasktracker/internal/version"
)

const (
	githubAPIURL = "https://api.github.com/repos/%s/%s/releases/latest"
)

// GitHubRelease represents the structure of a GitHub release.
type GitHubRelease struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// SelfUpdate checks for a new version on GitHub and updates the application if available.
func SelfUpdate(owner, repo string) error {
	fmt.Println("Checking for updates...")
	currentVersion := version.Version
	if currentVersion == "dev" {
		fmt.Println("Running in development mode, skipping update check.")
		return nil
	}

	latestVersionTag, downloadURL, err := CheckForUpdates(owner, repo)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if latestVersionTag == "" || downloadURL == "" {
		fmt.Println("No update found.")
		return nil
	}

	if compareVersions(currentVersion, latestVersionTag) >= 0 {
		fmt.Printf("Current version (%s) is up to date or newer than latest (%s).\n", currentVersion, latestVersionTag)
		return nil
	}

	fmt.Printf("New version (%s) available. Current: %s. Downloading...\n", latestVersionTag, currentVersion)

	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	err = DownloadAndReplace(downloadURL, executablePath)
	if err != nil {
		return fmt.Errorf("failed to download and replace: %w", err)
	}

	fmt.Printf("Update to version %s successful! Please restart the application.\n", latestVersionTag)
	return nil
}

// CheckForUpdates fetches the latest release from GitHub and returns its tag name and appropriate download URL.
func CheckForUpdates(owner, repo string) (string, string, error) {
	url := fmt.Sprintf(githubAPIURL, owner, repo)
	resp, err := http.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch release info from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, resp.Status)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", fmt.Errorf("failed to decode GitHub release JSON: %w", err)
	}

	// Find the appropriate asset for the current OS and architecture
	assetPlatform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	expectedAssetSuffix := ""
	executableName := "task-tracker" // Base name without extension

	if runtime.GOOS == "windows" {
		expectedAssetSuffix = fmt.Sprintf("%s.zip", assetPlatform)
		executableName += ".exe"
	} else if runtime.GOOS == "linux" {
		expectedAssetSuffix = fmt.Sprintf("%s.tar.xz", assetPlatform)
	} else {
		return "", "", fmt.Errorf("unsupported operating system for self-update: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, expectedAssetSuffix) {
			// Also ensure the asset name contains the executable base name to be more specific
			if strings.Contains(asset.Name, strings.TrimSuffix(executableName, ".exe")) {
				return release.TagName, asset.BrowserDownloadURL, nil
			}
		}
	}

	return "", "", fmt.Errorf("no suitable asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// DownloadAndReplace downloads the new version and replaces the current executable.
func DownloadAndReplace(downloadURL, executablePath string) error {
	// Create a temporary directory for the download
	tmpDir, err := os.MkdirTemp("", "github.com/highercomve/tasktracker-update-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir) // Clean up temp dir later

	// Determine the temporary file path for the downloaded archive
	archiveName := filepath.Base(downloadURL)
	tmpArchivePath := filepath.Join(tmpDir, archiveName)

	// Download the archive
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download archive, HTTP status: %d (%s)", resp.StatusCode, resp.Status)
	}

	outFile, err := os.Create(tmpArchivePath)
	if err != nil {
		return fmt.Errorf("failed to create temporary archive file: %w", err)
	}
	defer outFile.Close() // Ensure file is closed even if io.Copy fails

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write download to temporary file: %w", err)
	}
	outFile.Close() // Explicitly close before opening for extraction to ensure flush

	// Extract the new executable
	extractedExecutablePath := ""
	if strings.HasSuffix(archiveName, ".tar.xz") {
		extractedExecutablePath, err = extractTarXz(tmpArchivePath, tmpDir, executablePath)
		if err != nil {
			return fmt.Errorf("failed to extract .tar.xz: %w", err)
		}
	} else if strings.HasSuffix(archiveName, ".zip") {
		extractedExecutablePath, err = extractZip(tmpArchivePath, tmpDir, executablePath)
		if err != nil {
			return fmt.Errorf("failed to extract .zip: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported archive format: %s", archiveName)
	}

	if extractedExecutablePath == "" {
		return fmt.Errorf("failed to find extracted executable in archive")
	}

	// Replace the old executable with the new one
	return replaceExecutable(executablePath, extractedExecutablePath)
}

// extractTarXz extracts the binary from a .tar.xz archive.
func extractTarXz(archivePath, destDir, executablePath string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	xzReader, err := xz.NewReader(file)
	if err != nil {
		return "", err
	}

	tarReader := tar.NewReader(xzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return "", err
		}

		if header.Typeflag == tar.TypeReg {
				// Expecting the executable to be named after the original executable
				expectedExecutableName := filepath.Base(executablePath)
				// On Linux, the executable name won't have .exe
				if runtime.GOOS == "linux" && strings.HasSuffix(expectedExecutableName, ".exe") {
					expectedExecutableName = strings.TrimSuffix(expectedExecutableName, ".exe")
				}
				if filepath.Base(header.Name) == expectedExecutableName {				newExecutablePath := filepath.Join(destDir, expectedExecutableName)
				newFile, err := os.OpenFile(newExecutablePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, header.FileInfo().Mode())
				if err != nil {
					return "", err
				}
				defer newFile.Close()

				if _, err := io.Copy(newFile, tarReader); err != nil {
					return "", err
				}
				return newExecutablePath, nil
			}
		}
	}
	return "", fmt.Errorf("executable 'task-tracker' not found in .tar.xz archive")
}

// extractZip extracts the binary from a .zip archive.
func extractZip(archivePath, destDir, executablePath string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	expectedExecutableName := filepath.Base(executablePath)
	// On Windows, the executable name should have .exe
	if runtime.GOOS == "windows" && !strings.HasSuffix(expectedExecutableName, ".exe") {
		expectedExecutableName += ".exe"
	}

	for _, f := range r.File {
		// Expecting the executable to be named "task-tracker.exe" (or "task-tracker") directly in the zip
		if filepath.Base(f.Name) == expectedExecutableName {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			newExecutablePath := filepath.Join(destDir, expectedExecutableName)
			newFile, err := os.OpenFile(newExecutablePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
			if err != nil {
				return "", err
			}
			defer newFile.Close()

			if _, err := io.Copy(newFile, rc); err != nil {
				return "", err
			}
			return newExecutablePath, nil
		}
	}
	return "", fmt.Errorf("executable '%s' not found in .zip archive", expectedExecutableName)
}

// replaceExecutable handles replacing the running executable with the new one.
// This is OS-specific, especially for Windows where an executable cannot replace itself while running.
func replaceExecutable(oldExecutablePath, newExecutablePath string) error {
	// For Windows, a direct rename/overwrite of a running executable will fail.
	// The common workaround is to rename the old one, move the new one, and
	// rely on a subsequent start to clean up the old one, or a helper script.
	// For simplicity here, we try to rename, and if it fails, provide a message.

	// Attempt to create a temporary backup of the old executable
	backupPath := oldExecutablePath + ".old"
	if runtime.GOOS == "windows" {
		// On Windows, if the old executable is in use, os.Rename will fail.
		// We can't rename the running executable.
		// The strategy here is to download the new executable, and the user will need to
		// manually replace it or restart the app for the OS to release the old file.
		// For a fully automated update on Windows, a small launcher/helper would be needed
		// that swaps the files and then launches the new executable.
		// Since this is a Fyne app, a full restart is expected.

		// However, for consistency with other OS, I'll attempt the rename first.
		// If it fails on Windows, it's expected if the app is running.
		// The ideal for Windows is to:
		// 1. Download new EXE to temp.
		// 2. Schedule old EXE for deletion on reboot / next app launch.
		// 3. Move new EXE to old EXE's path.
		// This is beyond a simple `os.Rename`.

		// For now, I'll stick to a simple rename and inform the user.
		// If the original executable is still running and locked, this will fail.
		// A common pattern is to defer the actual replacement until restart
		// or use a separate updater process.

		// Renaming the current executable to .old is required before moving the new one.
		// If the running executable cannot be renamed, the update cannot proceed automatically.
		if err := os.Rename(oldExecutablePath, backupPath); err != nil {
			return fmt.Errorf("failed to rename current executable to backup (%s). On Windows, this may mean the application is still running and locked. Please close the application and try again: %w", oldExecutablePath, err)
		}
	} else {
		// On non-Windows, we can rename the old executable
		if err := os.Rename(oldExecutablePath, backupPath); err != nil {
			return fmt.Errorf("failed to rename old executable to backup: %w", err)
		}
	}

	// Move the new executable into place
	if err := os.Rename(newExecutablePath, oldExecutablePath); err != nil {
		// If moving fails, try to move the backup back to rollback
		_ = os.Rename(backupPath, oldExecutablePath) // Best effort rollback
		return fmt.Errorf("failed to move new executable into place: %w", err)
	}

	// Make sure the new executable has execute permissions
	// os.Rename might not preserve permissions if the underlying file systems differ or for other reasons.
	// For Linux/macOS, it's crucial. Windows handles executables differently.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(oldExecutablePath, 0755); err != nil {
			return fmt.Errorf("failed to set execute permissions on new executable: %w", err)
		}
	}

	// Clean up the backup on non-Windows. On Windows, it might be locked until the process exits.
	if runtime.GOOS != "windows" {
		_ = os.Remove(backupPath) // Best effort cleanup
	} else {
		// On Windows, the .old file might be locked by the running process.
		// It will be cleaned up automatically once the process exits and the file lock is released.
		// A more advanced updater might schedule this for deletion on reboot.
		fmt.Printf("On Windows, the old executable might remain as '%s' until restart.\n", backupPath)
	}

	return nil
}

// compareVersions compares two version strings (e.g., "v1.0.0" and "v1.0.1").
// Returns:
//
//	-1 if versionA is older than versionB
//	0 if versionA is the same as versionB
//	1 if versionA is newer than versionB
//
// It uses a simple numeric comparison of parts after stripping "v".
func compareVersions(versionA, versionB string) int {
	// Trim "v" prefix if present
	versionA = strings.TrimPrefix(versionA, "v")
	versionB = strings.TrimPrefix(versionB, "v")

	aParts := strings.Split(versionA, ".")
	bParts := strings.Split(versionB, ".")

	maxParts := len(aParts)
	if len(bParts) > maxParts {
		maxParts = len(bParts)
	}

	for i := 0; i < maxParts; i++ {
		aPart := 0
		if i < len(aParts) {
			aPart = Atoi(aParts[i])
		}
		bPart := 0
		if i < len(bParts) {
			bPart = Atoi(bParts[i])
		}

		if aPart < bPart {
			return -1
		}
		if aPart > bPart {
			return 1
		}
	}
	return 0
}

// Atoi is a wrapper for strconv.Atoi that ignores errors and returns 0 on failure.
func Atoi(s string) (i int) {
	i, _ = strconv.Atoi(s)
	return
}
