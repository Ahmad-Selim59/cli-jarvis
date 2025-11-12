# Docu-Jarvis CLI

AI-powered documentation and code quality tool for engineering teams.

## Installation

### Download the binary

```bash
curl -L https://github.com/udemy/docu-jarvis-cli2/releases/latest/download/docu-jarvis -o docu-jarvis
```

### Make it executable

```bash
chmod +x docu-jarvis
```

### Move to PATH

```bash
sudo mv docu-jarvis /usr/local/bin/
```

### Use from anywhere

```bash
docu-jarvis -help
```

## First Time Setup

Configure your repository and GitHub token:

```bash
docu-jarvis -config
```

Set these values:
- `repo` - Your GitHub repository URL
- `github_token` - GitHub Personal Access Token ([create one here](https://github.com/settings/tokens) with `repo` scope)
- `code_standards` - Your code quality rules (optional, for `-check-staging`)

## Features

### Update Documentation
Keep existing docs synchronized with code:
```bash
docu-jarvis -update-docs all
docu-jarvis -update-docs api.md
```

### Write Documentation
Generate new comprehensive documentation:
```bash
docu-jarvis -write-docs "API Authentication"
docu-jarvis -write-docs "API,Database,Caching"
```

### Debug Mode
Find which commit caused a bug:
```bash
docu-jarvis -debug "2024-11-01" "2024-11-10" "null pointer error"
```

### Code Quality Check
Review staged code against your standards:
```bash
git add .
docu-jarvis -check-staging
```

### Commit Explainer
Interactive conversation about a specific commit:
```bash
docu-jarvis -explain abc123
docu-jarvis -explain abc123 "What files changed?"
```

### Auto-Updates
Check for updates:
```bash
docu-jarvis -version
```

Update to latest version:
```bash
docu-jarvis -update
```

The tool automatically checks for updates once per 24 hours when you run any command.

## Requirements

- macOS (binary built for macOS)
- Git
- GitHub CLI (`gh`) - Install with `brew install gh`
- GitHub Personal Access Token (for private repos)
- Claude API access (via Claude Code SDK)

## Help

```bash
docu-jarvis -help
docu-jarvis -help update-docs
docu-jarvis -help write-docs
docu-jarvis -help debug
docu-jarvis -help check-staging
docu-jarvis -help explain
```

## Configuration

Config file location: `~/.docu-jarvis/config`

Example config:
```
repo = https://github.com/udemy/your-repo.git
github_token = ghp_your_token_here
code_standards = All functions must have documentation
code_standards = Use meaningful variable names
code_standards = Handle errors explicitly
```

## How It Works

Docu-Jarvis uses Claude AI to understand your codebase and perform intelligent documentation and analysis tasks. Each feature uses specialized AI prompts to guide Claude through specific workflows like updating documentation, analyzing commits, or reviewing code quality.

All operations that modify code create pull requests for review rather than directly committing changes.
