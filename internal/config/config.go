package config

import (
	"fmt"

	"github.com/udemy/docu-jarvis-cli/internal/settings"
)

type Config struct {
	RepoURL string
}

func Load() (*Config, error) {
	s, err := settings.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	repoURL := s.GetRepoURL()
	if repoURL == "" || repoURL == "https://github.com/your-org/your-repo.git" {
		return nil, fmt.Errorf("repository URL not configured.\n\nConfigure it:\n  docu-jarvis -config\n\nOr use environment variable:\n  export REPO_URL=\"https://github.com/your-org/your-repo.git\"")
	}

	return &Config{
		RepoURL: repoURL,
	}, nil
}

func (c *Config) GetRepoName() string {
	repoURL := c.RepoURL
	// Extract the last part of the URL
	parts := []rune(repoURL)
	lastSlash := -1
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == '/' {
			lastSlash = i
			break
		}
	}

	repoName := ""
	if lastSlash >= 0 && lastSlash < len(parts)-1 {
		repoName = string(parts[lastSlash+1:])
	} else {
		repoName = repoURL
	}

	if len(repoName) > 4 && repoName[len(repoName)-4:] == ".git" {
		repoName = repoName[:len(repoName)-4]
	}

	return repoName
}
