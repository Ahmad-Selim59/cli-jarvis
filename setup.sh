#!/bin/bash


set -e

echo "======================================"
echo "Docu-Jarvis CLI Setup"
echo "======================================"
echo ""

echo "Checking Go installation..."
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21 or higher."
    echo "   Visit: https://golang.org/dl/"
    exit 1
fi
echo "✓ Go is installed: $(go version)"

echo ""
echo "Checking Claude CLI installation..."
if ! command -v claude &> /dev/null; then
    echo "⚠️  Claude CLI is not installed."
    echo "   You can install it with: npm install -g @anthropic-ai/claude-code"
    echo ""
    read -p "Continue without Claude CLI? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    echo "✓ Claude CLI is installed"
    
    # Check if authenticated
    if claude --version &> /dev/null; then
        echo "✓ Claude CLI appears to be working"
    else
        echo "⚠️  Claude CLI may not be authenticated"
        echo "   Run: claude"
        echo "   And follow the authentication prompts"
    fi
fi

echo ""
echo "Checking Git installation..."
if ! command -v git &> /dev/null; then
    echo "❌ Git is not installed. Please install Git."
    exit 1
fi
echo "✓ Git is installed: $(git --version)"

echo ""
echo "Checking GitHub CLI..."
if ! command -v gh &> /dev/null; then
    echo "⚠️  GitHub CLI is not installed."
    echo "   You can install it from: https://cli.github.com/"
    echo "   Or use: brew install gh (on macOS)"
    echo ""
    read -p "Continue without GitHub CLI? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    echo "✓ GitHub CLI is installed: $(gh --version | head -n 1)"
    
    if gh auth status &> /dev/null; then
        echo "✓ GitHub CLI is authenticated"
    else
        echo "⚠️  GitHub CLI is not authenticated"
        echo "   Run: gh auth login"
    fi
fi

echo ""
echo "Checking environment configuration..."
if [ -z "$REPO_URL" ]; then
    echo "⚠️  REPO_URL environment variable is not set"
    echo ""
    read -p "Enter your repository URL (e.g., https://github.com/org/repo.git): " repo_url
    
    if [ -n "$repo_url" ]; then
        echo ""
        echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo "  export REPO_URL=\"$repo_url\""
        echo ""
        export REPO_URL="$repo_url"
        echo "✓ REPO_URL temporarily set for this session"
    fi
else
    echo "✓ REPO_URL is set: $REPO_URL"
fi

echo ""
echo "Building Docu-Jarvis CLI..."
go build -o docu-jarvis ./cmd/docu-jarvis
echo "✓ Build successful"

echo ""
echo "======================================"
echo "Setup Complete!"
echo "======================================"
echo ""
echo "To run Docu-Jarvis:"
echo "  ./docu-jarvis"
echo ""
echo "To install globally:"
echo "  sudo mv docu-jarvis /usr/local/bin/"
echo ""
echo "Or use the Makefile:"
echo "  make build    - Build the application"
echo "  make install  - Build and install globally"
echo "  make run      - Build and run"
echo ""

