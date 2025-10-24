# Release Process Documentation

## Overview

The release process is now simplified to use manual VERSION file management with automatic release triggering.

## How to Create a Release

### 1. Update VERSION File
Create a PR that updates the `VERSION` file in the repository root:

```bash
# Update the version (use semantic versioning X.Y.Z)
echo "1.2.3" > VERSION

# Commit and create PR
git add VERSION
git commit -m "Release version 1.2.3"
git push origin feature/release-1.2.3
```

### 2. Merge PR
When the PR is merged to main, the release workflow will automatically:
- Validate the version format
- Check that the tag doesn't already exist
- Build and push container images
- Generate release manifests
- Create GitHub release with artifacts

### 3. Release Artifacts
The release will include:
- **Container Images**: Pushed to `public.ecr.aws/q5p6u7h8/neuron-openshift/`
- **Manifests**: `aws-neuron-operator.yaml`, `nfd-rule.yaml`, `deviceconfig-sample.yaml`
- **Git Tag**: `v{VERSION}` (e.g., `v1.2.3`)

## Version Format

Use semantic versioning (X.Y.Z):
- **Major** (X): Breaking changes
- **Minor** (Y): New features, backward compatible
- **Patch** (Z): Bug fixes, backward compatible

Examples: `1.0.0`, `1.2.3`, `2.0.0`

## Republishing Releases

To republish an existing release (rebuild images/manifests):

1. Go to **Actions** → **Republish Release**
2. Click **Run workflow**
3. Enter the version to republish (e.g., `1.2.3`)
4. Click **Run workflow**

## Test Pipeline

To test changes before release:

1. Go to **Actions** → **Test Pipeline**
2. Click **Run workflow**
3. Enter the PR number to test
4. Click **Run workflow**
5. Download test manifests from workflow artifacts

## Security Scanning

### Automated Security Scans

All releases and PRs are automatically scanned for security vulnerabilities:

- **Container Images**: Trivy scans for OS and application vulnerabilities
- **Source Code**: CodeQL and Gosec analyze code for security issues
- **Kubernetes Manifests**: Configuration scanning for security best practices
- **Dependencies**: Go modules scanned for known vulnerabilities

### Local Security Scanning

#### First-time Setup (macOS)

If you're on macOS, install security tools first:

```bash
# Install security tools via Homebrew (recommended)
make install-security-tools-macos

# Or install manually
./hack/install-security-tools-macos.sh
```

#### Running Security Scans

Run security scans locally before submitting PRs:

```bash
# Run comprehensive security scan
make security-scan

# Or run the script directly
./hack/security-scan.sh
```

**Note for Apple Silicon (M1/M2/M3) users**: The script automatically handles ARM64 architecture and builds Docker images for the correct platform.

### Security Reports

- **GitHub Security Tab**: View detailed vulnerability reports
- **Workflow Artifacts**: Download security scan reports
- **Daily Scans**: Scheduled scans check for new vulnerabilities

## Troubleshooting

### Common Issues

**Error: "Tag v1.2.3 already exists"**
- The version has already been released
- Use a different version number

**Error: "Version '1.2' is not a valid semantic version"**
- Use full semantic version format (X.Y.Z)
- Example: `1.2.0` instead of `1.2`

**Error: "VERSION file not found"**
- Ensure VERSION file exists in repository root
- File should contain only the version number

**Security Scan Failures**
- Review security scan results in GitHub Actions
- Check the Security tab for detailed vulnerability reports
- High/Critical vulnerabilities are reported but don't block releases
- Consider updating base images or dependencies

### Getting Help

1. Check workflow logs in GitHub Actions
2. Verify VERSION file format and content
3. Ensure no existing tag conflicts
4. Validate semantic versioning format
5. Review security scan results and reports