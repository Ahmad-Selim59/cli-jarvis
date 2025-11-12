package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Repo struct {
	url       string
	localPath string
}

func NewRepo(url string) *Repo {
	return &Repo{
		url: url,
	}
}

func (r *Repo) Clone(repoName string) (string, error) {
	targetDir := filepath.Join("/tmp", repoName)

	if _, err := os.Stat(targetDir); err == nil {
		fmt.Printf("Removing existing directory: %s\n", targetDir)
		if err := os.RemoveAll(targetDir); err != nil {
			return "", fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	fmt.Printf("Cloning %s to %s\n", r.url, targetDir)
	cmd := exec.Command("git", "clone", r.url, targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to clone repository: %w", err)
	}

	r.localPath = targetDir
	fmt.Printf("Successfully cloned repository to: %s\n", targetDir)
	fmt.Printf("Local path set to: %s\n", r.localPath)

	return targetDir, nil
}

func (r *Repo) GetLocalPath() string {
	return r.localPath
}

func (r *Repo) SetLocalPath(path string) {
	r.localPath = path
}

func (r *Repo) CreatePR() error {
	if r.localPath == "" {
		return fmt.Errorf("repository not cloned")
	}

	now := time.Now()
	branchName := fmt.Sprintf("docu-jarvis_%02d/%02d/%d_%02d_%02d",
		now.Day(), now.Month(), now.Year(), now.Hour(), now.Minute())

	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(r.localPath); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	if err := runCommand("git", "config", "user.name", "Docu Jarvis"); err != nil {
		return fmt.Errorf("failed to set git user.name: %w", err)
	}

	if err := runCommand("git", "config", "user.email", "docu-jarvis@automation.local"); err != nil {
		return fmt.Errorf("failed to set git user.email: %w", err)
	}

	if err := runCommand("git", "checkout", "-b", branchName); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	if err := runCommand("git", "add", "documentation/"); err != nil {
		return fmt.Errorf("failed to add documentation: %w", err)
	}

	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	if err := cmd.Run(); err == nil {
		fmt.Println("No changes to commit in documentation directory")
		return nil
	}

	commitMessage := "docs: automated documentation improvements by docu-jarvis"
	if err := runCommand("git", "commit", "-m", commitMessage); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	fmt.Printf("Pushing branch: %s\n", branchName)
	if err := runCommand("git", "push", "origin", branchName); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}

	prTitle := "Documentation Update"
	prDescription := "Automated docu-jarvis suggestions"

	if err := runCommand("gh", "pr", "create",
		"--title", prTitle,
		"--body", prDescription,
		"--head", branchName,
		"--base", "main"); err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}

	fmt.Printf("Successfully created PR with branch: %s\n", branchName)
	return nil
}

func (r *Repo) HasChanges() (bool, error) {
	if r.localPath == "" {
		return false, fmt.Errorf("repository not cloned")
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(r.localPath); err != nil {
		return false, fmt.Errorf("failed to change directory: %w", err)
	}

	cmd := exec.Command("git", "status", "--porcelain", "documentation/")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

func (r *Repo) GetCommitsBetweenDates(fromDate, toDate string) ([]string, error) {
	if r.localPath == "" {
		return nil, fmt.Errorf("repository not cloned")
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(r.localPath); err != nil {
		return nil, fmt.Errorf("failed to change directory: %w", err)
	}

	// Format: hash|author|date|subject
	gitLogFormat := "--pretty=format:%H|%an|%ai|%s"

	cmd := exec.Command("git", "log", gitLogFormat, "--since="+fromDate, "--until="+toDate)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log: %w", err)
	}

	if len(output) == 0 {
		return []string{}, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var commits []string
	for _, line := range lines {
		if line != "" {
			commits = append(commits, line)
		}
	}

	return commits, nil
}

func (r *Repo) GetStagedDiff() (string, error) {
	if r.localPath == "" {
		return "", fmt.Errorf("repository not cloned")
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(r.localPath); err != nil {
		return "", fmt.Errorf("failed to change directory: %w", err)
	}

	cmd := exec.Command("git", "diff", "--cached")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get staged diff: %w", err)
	}

	if len(output) == 0 {
		return "", fmt.Errorf("no staged changes found")
	}

	return string(output), nil
}

func (r *Repo) GetCommitDiff(commitHash string) (string, error) {
	if r.localPath == "" {
		return "", fmt.Errorf("repository not cloned")
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(r.localPath); err != nil {
		return "", fmt.Errorf("failed to change directory: %w", err)
	}

	cmd := exec.Command("git", "show", commitHash, "--format=fuller")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit diff: %w", err)
	}

	if len(output) == 0 {
		return "", fmt.Errorf("commit not found: %s", commitHash)
	}

	return string(output), nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
