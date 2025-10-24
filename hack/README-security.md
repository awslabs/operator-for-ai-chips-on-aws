# Security Tools

This directory contains security scanning and setup scripts for local development.

## Scripts

### `security-scan.sh`
Comprehensive security scanning script that works on both macOS and Linux.

**Features:**
- Scans container images, filesystem, and Kubernetes manifests
- Cross-platform compatible
- Integrates with CI/CD workflows

**Usage:**
```bash
make security-scan
```

### `install-security-tools-macos.sh`
macOS-specific script to install security tools via Homebrew.

**Usage:**
```bash
make install-security-tools-macos
```

## Platform Support

### macOS (Intel & Apple Silicon)
- ✅ Homebrew installation (recommended)
- ✅ Manual binary installation (fallback)
- ✅ Docker platform handling for ARM64
- ✅ Apple Silicon M1/M2/M3 support

### Linux
- ✅ APT package manager (Debian/Ubuntu)
- ✅ YUM package manager (RHEL/CentOS)
- ✅ Manual binary installation (fallback)

## Security Scans Performed

1. **Container Image Scan**
   - OS package vulnerabilities
   - Application dependencies
   - Base image security issues

2. **Filesystem Scan**
   - Source code vulnerabilities
   - Dependency issues
   - Configuration problems

3. **Kubernetes Manifest Scan**
   - Security best practices
   - RBAC configurations
   - Pod security policies

4. **Dockerfile Scan**
   - Best practices validation
   - Security configuration issues

## Requirements

- Docker (Docker Desktop on macOS)
- Internet connection (for downloading Trivy database)
- Sufficient disk space for vulnerability database (~500MB)

## Troubleshooting

### macOS Issues

**"Docker daemon not running"**
- Start Docker Desktop application
- Wait for Docker to fully initialize

**"Permission denied" errors**
- Ensure scripts are executable: `chmod +x hack/*.sh`
- May need `sudo` for some installations

**Homebrew not found**
- Install Homebrew: `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`
- Restart terminal after installation

### General Issues

**"Trivy database download failed"**
- Check internet connection
- Try running again (database download can be flaky)
- Clear Trivy cache: `trivy clean --all`

**"No vulnerabilities found" (suspicious)**
- Ensure image was built successfully
- Check if Trivy database is up to date
- Verify scan is running against correct image

**"Unable to find the specified image" error**
- Ensure you're in the repository root directory
- Check Docker Desktop is running and has sufficient resources
- Try rebuilding the image manually: `docker build -t local/aws-neuron-operator:security-scan .`

## Integration

These scripts are integrated into:
- GitHub Actions workflows (`.github/workflows/security-scan.yml`)
- Release pipeline (automatic scanning)
- Test pipeline (PR validation)
- Local development (Make targets)

## Security Policy

See `SECURITY.md` for the complete security policy and vulnerability reporting procedures.