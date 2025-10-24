#!/bin/bash
set -e

# Local security scanning script
echo "🔒 Running local security scans..."

# Check if we're in the right directory
if [[ ! -f "go.mod" ]] || [[ ! -f "Dockerfile" ]] || [[ ! -f "Makefile" ]]; then
    echo "❌ This script must be run from the repository root directory"
    echo "Expected files: go.mod, Dockerfile, Makefile"
    echo "Current directory: $(pwd)"
    exit 1
fi

echo "✅ Running from repository root: $(pwd)"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Normalize architecture names
case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "Detected OS: $OS, Architecture: $ARCH"

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is required but not installed"
    echo "Please install Docker Desktop: https://www.docker.com/products/docker-desktop/"
    exit 1
fi

# Check if Docker daemon is running
if ! docker info &> /dev/null; then
    echo "❌ Docker daemon is not running"
    echo "Please start Docker Desktop and try again"
    exit 1
fi

echo "✅ Docker is available and running"



# Check if trivy is installed
if ! command -v trivy &> /dev/null; then
    echo "Installing Trivy..."
    
    if [[ "$OS" == "darwin" ]]; then
        # macOS - check if Homebrew is available
        if command -v brew &> /dev/null; then
            echo "Installing Trivy via Homebrew..."
            brew install trivy
        else
            echo "Homebrew not found. Installing Trivy manually..."
            TRIVY_VERSION="0.48.3"  # Use a recent stable version
            DOWNLOAD_URL="https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_${OS^}_${ARCH^}.tar.gz"
            
            echo "Downloading Trivy from: $DOWNLOAD_URL"
            curl -L "$DOWNLOAD_URL" | tar xz
            sudo mv trivy /usr/local/bin/
            chmod +x /usr/local/bin/trivy
            echo "Trivy installed to /usr/local/bin/trivy"
        fi
        
    elif [[ "$OS" == "linux" ]]; then
        # Linux - try package manager first, then manual install
        if command -v apt-get &> /dev/null; then
            echo "Installing Trivy via apt..."
            sudo apt-get update
            sudo apt-get install -y wget apt-transport-https gnupg lsb-release
            wget -qO - https://aquasecurity.github.io/trivy-repo/deb/public.key | sudo apt-key add -
            echo "deb https://aquasecurity.github.io/trivy-repo/deb $(lsb_release -sc) main" | sudo tee -a /etc/apt/sources.list.d/trivy.list
            sudo apt-get update
            sudo apt-get install -y trivy
        elif command -v yum &> /dev/null; then
            echo "Installing Trivy via yum..."
            TRIVY_VERSION="0.48.3"
            sudo yum install -y wget
            wget "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.rpm"
            sudo rpm -ivh "trivy_${TRIVY_VERSION}_Linux-64bit.rpm"
            rm "trivy_${TRIVY_VERSION}_Linux-64bit.rpm"
        else
            echo "Installing Trivy manually..."
            TRIVY_VERSION="0.48.3"
            DOWNLOAD_URL="https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_${OS^}_${ARCH^}.tar.gz"
            
            echo "Downloading Trivy from: $DOWNLOAD_URL"
            curl -L "$DOWNLOAD_URL" | tar xz
            sudo mv trivy /usr/local/bin/
            chmod +x /usr/local/bin/trivy
            echo "Trivy installed to /usr/local/bin/trivy"
        fi
    else
        echo "Unsupported OS: $OS"
        echo "Please install Trivy manually: https://aquasecurity.github.io/trivy/latest/getting-started/installation/"
        exit 1
    fi
    
    # Verify installation
    if command -v trivy &> /dev/null; then
        echo "✅ Trivy installed successfully: $(trivy --version)"
    else
        echo "❌ Trivy installation failed"
        exit 1
    fi
else
    echo "✅ Trivy already installed: $(trivy --version)"
fi

# Update Trivy database
echo "🔄 Updating Trivy vulnerability database..."
if ! trivy image --download-db-only; then
    echo "⚠️  Failed to update Trivy database, continuing with existing database"
else
    echo "✅ Trivy database updated successfully"
fi

# Build image for scanning
echo "📦 Building image for security scan..."

# Check if Dockerfile exists
if [[ ! -f "Dockerfile" ]]; then
    echo "❌ Dockerfile not found in current directory"
    echo "Please run this script from the repository root"
    exit 1
fi

# Build the image with better error handling
echo "Building Docker image..."
if [[ "$OS" == "darwin" && "$ARCH" == "arm64" ]]; then
    echo "Building for ARM64 platform (Apple Silicon) -> linux/amd64..."
    if ! docker build --platform linux/amd64 -t local/aws-neuron-operator:security-scan .; then
        echo "❌ Docker build failed"
        exit 1
    fi
    echo "✅ Built for linux/amd64 to match production deployment platform"
else
    if ! docker build -t local/aws-neuron-operator:security-scan .; then
        echo "❌ Docker build failed"
        exit 1
    fi
    echo "✅ Docker build completed"
fi

# Verify the image was created
echo "🔍 Verifying Docker image..."
if docker images local/aws-neuron-operator:security-scan --format "{{.Repository}}:{{.Tag}}" | grep -q "local/aws-neuron-operator:security-scan"; then
    IMAGE_SIZE=$(docker images local/aws-neuron-operator:security-scan --format "{{.Size}}")
    IMAGE_ID=$(docker images local/aws-neuron-operator:security-scan --format "{{.ID}}")
    echo "✅ Docker image created successfully:"
    echo "   Repository: local/aws-neuron-operator:security-scan"
    echo "   Image ID: $IMAGE_ID"
    echo "   Size: $IMAGE_SIZE"
else
    echo "❌ Docker image not found after build"
    echo "Available images:"
    docker images --format "table {{.Repository}}:{{.Tag}}\t{{.Size}}\t{{.ID}}"
    exit 1
fi

# Scan filesystem
echo ""
echo "🔍 Scanning filesystem..."
echo "This may take a few minutes on first run (downloading vulnerability database)..."
if ! trivy fs --severity HIGH,CRITICAL --quiet .; then
    echo ""
    echo "⚠️  Filesystem scan completed with issues (see output above)"
else
    echo ""
    echo "✅ Filesystem scan completed successfully"
fi

# Scan Dockerfile
echo ""
echo "🐳 Scanning Dockerfile..."
if ! trivy config --severity HIGH,CRITICAL --quiet Dockerfile; then
    echo ""
    echo "⚠️  Dockerfile scan found issues (see output above)"
else
    echo ""
    echo "✅ Dockerfile scan completed successfully"
fi

# Scan container image
echo ""
echo "📋 Scanning container image..."
echo "Image: local/aws-neuron-operator:security-scan"

# Save image to tar and scan the tar file (more reliable)
echo "Exporting Docker image for scanning..."
TEMP_TAR="$(mktemp).tar"
if docker save local/aws-neuron-operator:security-scan -o "$TEMP_TAR"; then
    echo "Scanning exported image..."
    if trivy image --severity HIGH,CRITICAL --quiet --input "$TEMP_TAR"; then
        echo ""
        echo "✅ Container image scan completed successfully"
    else
        echo ""
        echo "⚠️  Container image scan found issues (see output above)"
    fi
    # Clean up the temporary tar file
    rm -f "$TEMP_TAR"
else
    echo ""
    echo "⚠️  Could not export Docker image - skipping image scan"
    echo "💡 The filesystem and config scans above are still valid"
fi

# Generate manifests and scan them
echo ""
echo "☸️  Generating and scanning Kubernetes manifests..."
make manifests > /dev/null 2>&1



make release-manifests PROJECT_VERSION=security-scan > /dev/null 2>&1
if ! trivy config --severity HIGH,CRITICAL --quiet release/security-scan/; then
    echo ""
    echo "⚠️  Kubernetes manifest scan found issues (see output above)"
else
    echo ""
    echo "✅ Kubernetes manifest scan completed successfully"
fi

# Scan Go dependencies
echo ""
echo "📚 Scanning Go dependencies..."
if ! trivy fs --scanners vuln --severity HIGH,CRITICAL --quiet .; then
    echo ""
    echo "⚠️  Go dependencies scan found issues (see output above)"
else
    echo ""
    echo "✅ Go dependencies scan completed successfully"
fi

# Clean up
echo ""
echo "🧹 Cleaning up..."

# Remove Docker image
if docker images -q local/aws-neuron-operator:security-scan > /dev/null 2>&1; then
    echo "Removing Docker image..."
    docker rmi local/aws-neuron-operator:security-scan 2>/dev/null || echo "⚠️  Could not remove Docker image"
fi

# Remove temporary release directory
if [[ -d "release/security-scan" ]]; then
    echo "Removing temporary files..."
    rm -rf release/security-scan 2>/dev/null || echo "⚠️  Could not remove temporary files"
fi

echo ""
echo "✅ Security scan completed!"
echo ""
echo "📊 Summary:"
echo "- Filesystem scan: Completed"
echo "- Dockerfile scan: Completed" 
echo "- Container image scan: Completed"
echo "- Kubernetes manifests scan: Completed"
echo "- Go dependencies scan: Completed"
echo ""
echo "💡 Tips:"
echo "  • Review any HIGH or CRITICAL findings above"
echo "  • Run 'trivy image --help' for more scanning options"
echo "  • Check GitHub Security tab for detailed reports"
echo "  • Consider running 'make lint' for additional code quality checks"