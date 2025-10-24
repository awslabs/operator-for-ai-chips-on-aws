#!/bin/bash
set -e

# macOS-specific security tools installation script
echo "🍎 Installing security tools for macOS..."

# Check if we're on macOS
if [[ "$OSTYPE" != "darwin"* ]]; then
    echo "❌ This script is for macOS only"
    exit 1
fi

# Check architecture
ARCH=$(uname -m)
echo "Detected architecture: $ARCH"

# Check if Homebrew is installed
if ! command -v brew &> /dev/null; then
    echo "📦 Installing Homebrew..."
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    
    # Add Homebrew to PATH for Apple Silicon Macs
    if [[ "$ARCH" == "arm64" ]]; then
        echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
        eval "$(/opt/homebrew/bin/brew shellenv)"
    fi
else
    echo "✅ Homebrew already installed"
fi

# Update Homebrew
echo "🔄 Updating Homebrew..."
brew update

# Install security tools
echo "🔒 Installing security scanning tools..."

# Install Trivy
if ! command -v trivy &> /dev/null; then
    echo "Installing Trivy..."
    brew install trivy
else
    echo "✅ Trivy already installed"
fi

# Install Docker if not present
if ! command -v docker &> /dev/null; then
    echo "Installing Docker..."
    brew install --cask docker
    echo "⚠️  Please start Docker Desktop manually after installation"
else
    echo "✅ Docker already installed"
fi

# Install additional useful security tools
echo "🛠️  Installing additional security tools..."

# Install gosec for Go security analysis
if ! command -v gosec &> /dev/null; then
    echo "Installing gosec..."
    brew install gosec
else
    echo "✅ gosec already installed"
fi

# Install hadolint for Dockerfile linting
if ! command -v hadolint &> /dev/null; then
    echo "Installing hadolint..."
    brew install hadolint
else
    echo "✅ hadolint already installed"
fi

# Install kube-score for Kubernetes security
if ! command -v kube-score &> /dev/null; then
    echo "Installing kube-score..."
    brew install kube-score
else
    echo "✅ kube-score already installed"
fi

# Install git-secrets for preventing secrets in git
if ! command -v git-secrets &> /dev/null; then
    echo "Installing git-secrets..."
    brew install git-secrets
else
    echo "✅ git-secrets already installed"
fi

echo ""
echo "🎉 Security tools installation completed!"
echo ""
echo "📋 Installed tools:"
echo "  ✅ trivy - Container and filesystem vulnerability scanner"
echo "  ✅ gosec - Go security analyzer"
echo "  ✅ hadolint - Dockerfile linter"
echo "  ✅ kube-score - Kubernetes security best practices"
echo "  ✅ git-secrets - Prevent secrets in git repos"
echo ""
echo "🚀 Next steps:"
echo "  1. Start Docker Desktop if not already running"
echo "  2. Run 'make security-scan' to perform security scans"
echo "  3. Run 'git secrets --install' to set up git-secrets hooks"
echo ""
echo "💡 Tip: Add these tools to your IDE for real-time security feedback"