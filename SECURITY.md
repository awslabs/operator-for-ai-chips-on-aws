# Security Policy

## Supported Versions

We provide security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Security Scanning

This project implements comprehensive security scanning:

### Automated Scans

- **Container Image Scanning**: Trivy scans all built images for vulnerabilities
- **Code Analysis**: CodeQL and Gosec scan source code for security issues
- **Configuration Scanning**: Kubernetes manifests and Dockerfiles are scanned
- **Dependency Scanning**: Go modules are scanned for known vulnerabilities

### Scan Schedule

- **On every PR**: Security scans run automatically
- **On main branch**: Full security suite runs on every push
- **Daily**: Scheduled scans check for new vulnerabilities
- **Release**: Additional security validation before release

### Security Thresholds

- **HIGH/CRITICAL vulnerabilities**: Reported but don't block releases (with review)
- **Container base images**: Regularly updated for security patches
- **Dependencies**: Monitored for security advisories

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

### Private Disclosure

1. **Email**: Send details to [security@yourcompany.com]
2. **Include**: 
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if available)

### What to Expect

- **Acknowledgment**: Within 48 hours
- **Initial Assessment**: Within 5 business days
- **Regular Updates**: Every 5 business days until resolved
- **Resolution**: Security fixes are prioritized

### Public Disclosure

- We follow coordinated disclosure practices
- Public disclosure after fix is available and deployed
- Credit given to security researchers (unless requested otherwise)

## Security Best Practices

### For Contributors

- Keep dependencies updated
- Follow secure coding practices
- Run security scans locally before submitting PRs
- Review security scan results in CI/CD

### For Users

- Always use the latest stable version
- Monitor security advisories
- Follow principle of least privilege
- Regularly update container images

## Security Features

### Container Security

- **Non-root user**: Containers run as non-privileged user (UID 201)
- **Minimal base image**: Uses UBI minimal for reduced attack surface
- **Multi-stage builds**: Reduces final image size and attack surface
- **Security scanning**: All images scanned before release

### Kubernetes Security

- **RBAC**: Minimal required permissions
- **Security contexts**: Appropriate security settings
- **Network policies**: Recommended network isolation
- **Pod security standards**: Follows Kubernetes security best practices

### Supply Chain Security

- **Dependency scanning**: All Go modules scanned
- **Base image updates**: Regular updates to base images
- **Signed releases**: Consider implementing signed releases
- **SBOM**: Software Bill of Materials for transparency

## Compliance

This project aims to comply with:

- **NIST Cybersecurity Framework**
- **OWASP Top 10**
- **CIS Kubernetes Benchmark**
- **SLSA Supply Chain Security**

## Security Tools Used

- **Trivy**: Vulnerability scanning
- **CodeQL**: Static code analysis
- **Gosec**: Go security analyzer
- **kube-score**: Kubernetes security best practices
- **GitHub Security Advisories**: Dependency vulnerability alerts

## Contact

For security-related questions or concerns:
- Security Team: [security@yourcompany.com]
- Maintainers: See MAINTAINERS.md
- GitHub Security Advisories: Use GitHub's private vulnerability reporting