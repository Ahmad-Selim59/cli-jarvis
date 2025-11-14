package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/udemy/docu-jarvis-cli/internal/settings"
)

const (
	owner   = "udemy"
	repo    = "docu-jarvis-cli2"
	version = "2.2.1"
)

type Release struct {
	Version      string
	AssetURL     string
	AssetName    string
	ReleaseNotes string
}

func (r *Release) LessOrEqual(version string) bool {
	return r.Version <= version
}

type AuthenticatedGitHubSource struct {
	token string
}

func (s *AuthenticatedGitHubSource) GetLatestRelease(ctx context.Context) (*Release, bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, false, err
	}

	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, false, nil
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(body))
	}

	var ghRelease struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Body    string `json:"body"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			URL                string `json:"url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghRelease); err != nil {
		return nil, false, err
	}

	var assetURL, assetName string
	for _, asset := range ghRelease.Assets {
		if asset.Name == "docu-jarvis" {
			assetURL = asset.URL
			assetName = asset.Name
			break
		}
	}

	if assetURL == "" {
		return nil, false, fmt.Errorf("binary 'docu-jarvis' not found in release assets")
	}

	release := &Release{
		Version:      ghRelease.TagName,
		AssetURL:     assetURL,
		AssetName:    assetName,
		ReleaseNotes: ghRelease.Body,
	}

	return release, true, nil
}

func CheckForUpdates(currentVersion string) (*Release, bool, error) {
	s, err := settings.Load()
	if err != nil {
		return nil, false, fmt.Errorf("failed to load settings: %w", err)
	}

	source := &AuthenticatedGitHubSource{token: s.GetGitHubToken()}

	latest, found, err := source.GetLatestRelease(context.Background())
	if err != nil {
		return nil, false, fmt.Errorf("error checking for updates: %w", err)
	}

	if !found {
		return nil, false, fmt.Errorf("no release found")
	}

	if latest.LessOrEqual(currentVersion) {
		return latest, false, nil
	}

	return latest, true, nil
}

func UpdateToLatest(currentVersion string) error {
	s, err := settings.Load()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	source := &AuthenticatedGitHubSource{token: s.GetGitHubToken()}

	latest, found, err := source.GetLatestRelease(context.Background())
	if err != nil {
		return fmt.Errorf("error detecting latest version: %w", err)
	}

	if !found {
		return fmt.Errorf("no release found")
	}

	if latest.LessOrEqual(currentVersion) {
		fmt.Println("Already up to date!")
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate executable path: %w", err)
	}

	if err := downloadAndReplace(context.Background(), latest.AssetURL, exe, s.GetGitHubToken()); err != nil {
		return fmt.Errorf("error updating binary: %w", err)
	}

	fmt.Printf("Successfully updated to version %s!\n", latest.Version)
	return nil
}

func downloadAndReplace(ctx context.Context, url, targetPath, token string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	if token != "" {
		req.Header.Set("Authorization", "token "+token)
		req.Header.Set("Accept", "application/octet-stream")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(body))
	}

	tmpFile := targetPath + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		os.Remove(tmpFile)
		return err
	}

	if err := out.Sync(); err != nil {
		os.Remove(tmpFile)
		return err
	}

	out.Close()

	if err := os.Chmod(tmpFile, 0755); err != nil {
		os.Remove(tmpFile)
		return err
	}

	if err := os.Rename(tmpFile, targetPath); err != nil {
		os.Remove(tmpFile)
		return err
	}

	return nil
}

func AutoCheckForUpdates(currentVersion string, silent bool) {
	latest, hasUpdate, err := CheckForUpdates(currentVersion)
	if err != nil {
		if !silent {
			log.Printf("Update check failed: %v", err)
		}
		return
	}

	if !hasUpdate {
		if !silent {
			fmt.Printf("You're running the latest version (%s)\n", currentVersion)
		}
		return
	}

	if !silent {
		fmt.Printf("\n OH YES! New version available: %s (current: %s)\n", latest.Version, currentVersion)
		fmt.Printf("Release notes: %s\n", latest.ReleaseNotes)
		fmt.Println("\nRun 'docu-jarvis -update' to upgrade")
	}
}

func GetCurrentVersion() string {
	return version
}

func ShouldCheckForUpdates() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return true
	}

	lastCheckFile := homeDir + "/.docu-jarvis/last_update_check"
	info, err := os.Stat(lastCheckFile)
	if err != nil {
		return true
	}

	return time.Since(info.ModTime()) > 24*time.Hour
}

func UpdateLastCheckTime() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := homeDir + "/.docu-jarvis"
	os.MkdirAll(configDir, 0755)

	lastCheckFile := configDir + "/last_update_check"
	return os.WriteFile(lastCheckFile, []byte(time.Now().String()), 0644)
}
