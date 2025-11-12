#!/bin/bash

# Quick test script for Docu-Jarvis CLI
# This verifies the installation and SDK integration

set -e

COLOR_GREEN='\033[0;32m'
COLOR_RED='\033[0;31m'
COLOR_YELLOW='\033[1;33m'
COLOR_BLUE='\033[0;34m'
COLOR_RESET='\033[0m'

echo ""
echo "╔════════════════════════════════════════╗"
echo "║   Docu-Jarvis CLI Quick Test          ║"
echo "╚════════════════════════════════════════╝"
echo ""

# Test 1: Check prerequisites
echo -e "${COLOR_BLUE}[1/5]${COLOR_RESET} Checking prerequisites..."

if ! command -v go &> /dev/null; then
    echo -e "${COLOR_RED}✗${COLOR_RESET} Go not found. Install from https://golang.org/"
    exit 1
fi
echo -e "${COLOR_GREEN}✓${COLOR_RESET} Go: $(go version | awk '{print $3}')"

if ! command -v claude &> /dev/null; then
    echo -e "${COLOR_YELLOW}⚠${COLOR_RESET} Claude CLI not found"
    echo "  Install with: npm install -g @anthropic-ai/claude-code"
    echo "  Then authenticate by running: claude"
    exit 1
fi
echo -e "${COLOR_GREEN}✓${COLOR_RESET} Claude CLI installed"

if ! command -v git &> /dev/null; then
    echo -e "${COLOR_RED}✗${COLOR_RESET} Git not found"
    exit 1
fi
echo -e "${COLOR_GREEN}✓${COLOR_RESET} Git: $(git --version | awk '{print $3}')"

if ! command -v gh &> /dev/null; then
    echo -e "${COLOR_YELLOW}⚠${COLOR_RESET} GitHub CLI not found (optional, needed for PR creation)"
else
    echo -e "${COLOR_GREEN}✓${COLOR_RESET} GitHub CLI installed"
fi

# Test 2: Download dependencies
echo ""
echo -e "${COLOR_BLUE}[2/5]${COLOR_RESET} Downloading dependencies..."
go mod download
echo -e "${COLOR_GREEN}✓${COLOR_RESET} Dependencies downloaded"

# Test 3: Build the project
echo ""
echo -e "${COLOR_BLUE}[3/5]${COLOR_RESET} Building docu-jarvis..."
go build -o docu-jarvis ./cmd/docu-jarvis
if [ ! -f "./docu-jarvis" ]; then
    echo -e "${COLOR_RED}✗${COLOR_RESET} Build failed"
    exit 1
fi
BINARY_SIZE=$(ls -lh docu-jarvis | awk '{print $5}')
echo -e "${COLOR_GREEN}✓${COLOR_RESET} Build successful (${BINARY_SIZE})"

# Test 4: Test binary execution
echo ""
echo -e "${COLOR_BLUE}[4/5]${COLOR_RESET} Testing binary..."
unset REPO_URL
OUTPUT=$(./docu-jarvis 2>&1 || true)
if echo "$OUTPUT" | grep -q "REPO_URL environment variable is not set"; then
    echo -e "${COLOR_GREEN}✓${COLOR_RESET} Binary runs and validates configuration"
else
    echo -e "${COLOR_RED}✗${COLOR_RESET} Unexpected output from binary"
    echo "$OUTPUT"
    exit 1
fi

# Test 5: Verify Claude SDK
echo ""
echo -e "${COLOR_BLUE}[5/5]${COLOR_RESET} Testing Claude Code SDK integration..."

# Create a minimal test
cat > /tmp/test_claude_sdk_$$.go << 'EOF'
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    claudecode "github.com/yukifoo/claude-code-sdk-go"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    request := claudecode.QueryRequest{
        Prompt: "Reply with exactly: 'SDK_TEST_OK'",
        Options: &claudecode.Options{
            MaxTurns: intPtr(1),
        },
    }
    
    messages, err := claudecode.QueryWithRequest(ctx, request)
    if err != nil {
        log.Printf("SDK error (this is expected if Claude CLI needs auth): %v", err)
        fmt.Println("SDK_NEEDS_AUTH")
        return
    }
    
    fmt.Printf("SDK_WORKS:%d", len(messages))
}

func intPtr(i int) *int { return &i }
EOF

# Try to run the SDK test
cd /tmp
go mod init test-sdk-$$ 2>/dev/null || true
go mod edit -require=github.com/yukifoo/claude-code-sdk-go@v0.0.0-20250618211252-be3af0d0e1b6 2>/dev/null
go mod download 2>/dev/null

SDK_OUTPUT=$(timeout 35s go run test_claude_sdk_$$.go 2>&1 || true)

# Clean up
rm -f test_claude_sdk_$$.go go.mod go.sum
cd - >/dev/null

if echo "$SDK_OUTPUT" | grep -q "SDK_WORKS"; then
    MESSAGE_COUNT=$(echo "$SDK_OUTPUT" | grep -o "SDK_WORKS:[0-9]*" | cut -d: -f2)
    echo -e "${COLOR_GREEN}✓${COLOR_RESET} Claude SDK works! (Received $MESSAGE_COUNT messages)"
elif echo "$SDK_OUTPUT" | grep -q "SDK_NEEDS_AUTH"; then
    echo -e "${COLOR_YELLOW}⚠${COLOR_RESET} Claude CLI needs authentication"
    echo "  Run: claude"
    echo "  Then follow the prompts to sign in"
elif echo "$SDK_OUTPUT" | grep -q "command not found\|not found"; then
    echo -e "${COLOR_RED}✗${COLOR_RESET} Claude CLI not properly installed"
    exit 1
else
    echo -e "${COLOR_YELLOW}⚠${COLOR_RESET} SDK test inconclusive (may need Claude CLI auth)"
    echo "  If you haven't authenticated yet, run: claude"
fi

# Summary
echo ""
echo "╔════════════════════════════════════════╗"
echo "║          Test Summary                  ║"
echo "╚════════════════════════════════════════╝"
echo ""
echo -e "${COLOR_GREEN}✓ Prerequisites installed${COLOR_RESET}"
echo -e "${COLOR_GREEN}✓ Dependencies downloaded${COLOR_RESET}"
echo -e "${COLOR_GREEN}✓ Binary built successfully${COLOR_RESET}"
echo -e "${COLOR_GREEN}✓ Configuration validation works${COLOR_RESET}"

if echo "$SDK_OUTPUT" | grep -q "SDK_WORKS"; then
    echo -e "${COLOR_GREEN}✓ Claude SDK integration confirmed${COLOR_RESET}"
else
    echo -e "${COLOR_YELLOW}⚠ Claude SDK (run 'claude' to authenticate)${COLOR_RESET}"
fi

echo ""
echo "═══════════════════════════════════════════"
echo ""
echo -e "${COLOR_GREEN}Ready to use!${COLOR_RESET}"
echo ""
echo "Next steps:"
echo "  1. Authenticate Claude CLI (if needed):"
echo "     ${COLOR_BLUE}claude${COLOR_RESET}"
echo ""
echo "  2. Set your repository URL:"
echo "     ${COLOR_BLUE}export REPO_URL='https://github.com/your-org/your-repo.git'${COLOR_RESET}"
echo ""
echo "  3. Run docu-jarvis:"
echo "     ${COLOR_BLUE}./docu-jarvis${COLOR_RESET}"
echo ""
echo "  4. (Optional) Install globally:"
echo "     ${COLOR_BLUE}sudo mv docu-jarvis /usr/local/bin/${COLOR_RESET}"
echo ""

