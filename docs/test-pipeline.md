# Test Pipeline Documentation

## Overview

The Test Pipeline allows developers to build and test container images from Pull Requests before merging to main. This enables validation of changes in an isolated environment using private ECR registry.

## Usage

### Triggering the Test Pipeline

1. Navigate to the **Actions** tab in the GitHub repository
2. Select **Test Pipeline** from the workflow list
3. Click **Run workflow**
4. Enter the Pull Request number you want to test
5. Click **Run workflow** to start the process

### Parameters

- **pr_number** (required): The Pull Request number to test (e.g., `123`)

## What the Pipeline Does

### 1. PR Validation
- Verifies the PR exists and is in OPEN state (works with fork PRs)
- Checks that the PR is mergeable (no conflicts)
- Validates PR can be merged with base branch
- Handles both same-repo and fork PRs automatically

### 2. Code Preparation
- Fetches the PR branch (handles both fork and same-repo PRs)
- Merges PR branch with latest base branch (usually main)
- Generates unique version tag: `pr-{PR_NUMBER}-{SHORT_SHA}`
- Creates semantic version for OLM bundle: `0.0.0-pr{PR_NUMBER}.{SHORT_SHA}`

### 3. Image Building
- Builds operator image and pushes to private ECR
- Generates OLM bundle and pushes bundle image
- Creates catalog index image for OLM installation
- All images use private registry: `582767206473.dkr.ecr.us-east-1.amazonaws.com`

### 4. Manifest Generation
- Creates test-specific Kubernetes manifests
- Updates image references to use private registry
- Generates three files:
  - `aws-neuron-operator.yaml` - Direct installation manifest
  - `nfd-rule.yaml` - Node Feature Discovery configuration
  - `deviceconfig-sample.yaml` - Example DeviceConfig with test images

### 5. Artifact Upload
- Uploads individual manifest files as workflow artifacts
- Creates compressed archive of all manifests
- Artifacts retained for 30 days

## Generated Artifacts

### Container Images

All images are pushed to private ECR registry with PR-specific tags:

```
582767206473.dkr.ecr.us-east-1.amazonaws.com/neuron-openshift/operator:pr-{PR_NUMBER}-{SHORT_SHA}
582767206473.dkr.ecr.us-east-1.amazonaws.com/neuron-openshift/operator-bundle:pr-{PR_NUMBER}-{SHORT_SHA}
582767206473.dkr.ecr.us-east-1.amazonaws.com/neuron-openshift/operator-index:pr-{PR_NUMBER}-{SHORT_SHA}
```

### Test Manifests

**Important**: Test artifacts are NOT published as GitHub releases. They are only available as workflow artifacts to prevent customer confusion.

Download from GitHub Actions workflow artifacts:

1. **Individual Files**: `test-manifests-pr-{PR_NUMBER}-{VERSION}`
2. **Archive**: `test-manifests-archive-pr-{PR_NUMBER}` (tar.gz format)

**How to access:**
1. Navigate to the completed test pipeline workflow run
2. Scroll to the bottom of the workflow run page
3. Download artifacts from the "Artifacts" section
4. These artifacts are only visible to repository collaborators, not public users

## Testing Your Changes

### Prerequisites

- Access to AWS account `582767206473`
- Kubernetes cluster with appropriate permissions
- `kubectl` CLI tool installed

### Installation Steps

1. **Download Artifacts**
   - Go to the completed workflow run
   - Download the `test-manifests-archive-pr-{PR_NUMBER}` artifact
   - Extract the tar.gz file

2. **Apply Test Manifests**
   ```bash
   # Apply the operator
   kubectl apply -f aws-neuron-operator.yaml
   
   # Wait for operator to be ready
   kubectl wait --for=condition=Available deployment/aws-neuron-operator-controller-manager -n ai-operator-on-aws --timeout=300s
   
   # Apply the test DeviceConfig
   kubectl apply -f deviceconfig-sample.yaml
   ```

3. **Verify Installation**
   ```bash
   # Check operator status
   kubectl get pods -n ai-operator-on-aws
   
   # Check DeviceConfig status
   kubectl get deviceconfig -n ai-operator-on-aws
   
   # View operator logs
   kubectl logs -n ai-operator-on-aws deployment/aws-neuron-operator-controller-manager
   ```

### Cleanup

```bash
# Remove test resources
kubectl delete -f deviceconfig-sample.yaml
kubectl delete -f aws-neuron-operator.yaml
```

## Troubleshooting

### Common Issues

#### 1. PR Validation Failures

**Error**: "PR does not exist or is not accessible"
- **Solution**: Verify the PR number is correct and the PR is still open
- **Note**: Works with both fork and same-repo PRs

**Error**: "PR is not open"
- **Solution**: Ensure the PR is still open and not closed/merged

**Error**: "PR has merge conflicts"
- **Solution**: Resolve merge conflicts in your PR branch first

#### 2. Authentication Failures

**Error**: "ECR private registry authentication failed"
- **Solution**: Verify AWS role permissions and OIDC configuration

**Error**: "Docker ECR authentication failed"
- **Solution**: Check AWS credentials and ECR login permissions

#### 3. Build Failures

**Error**: "Failed to build operator image"
- **Solution**: Check build logs for compilation errors in your PR

**Error**: "Bundle validation failed"
- **Solution**: Verify operator manifests and CRDs are valid

#### 4. Manifest Generation Failures

**Error**: "Failed to generate manifests"
- **Solution**: Check kustomize configuration and operator manifests

**Error**: "Generated manifest is empty"
- **Solution**: Verify kustomize build process and image references

### Getting Help

1. **Check Workflow Logs**: Review detailed logs in the GitHub Actions workflow run
2. **Validate PR**: Ensure your PR is mergeable and builds successfully locally
3. **Fork PR Issues**: The pipeline automatically handles fork PRs - check the logs for "Is Fork: true/false"
4. **AWS Permissions**: Verify ECR access and image push permissions
5. **Local Testing**: Test manifest generation locally using `make test-manifests`

## Advanced Usage

### Local Testing

You can test manifest generation locally:

```bash
# Set test environment variables
export PROJECT_VERSION="pr-123-abc1234"
export IMAGE_TAG="pr-123-abc1234"

# Generate test manifests
make test-manifests

# Check generated files
ls -la release/pr-123-abc1234/
```

### Custom Registry Testing

To test with a different private registry:

1. Update `TEST_IMAGE_TAG_BASE` in Makefile
2. Modify the workflow to use your registry
3. Ensure authentication is configured for your registry

### Integration with CI/CD

The test pipeline can be integrated into your development workflow:

1. **Automated Triggers**: Add webhook triggers for PR events
2. **Status Checks**: Use GitHub status API to report test results
3. **Notifications**: Add Slack/email notifications for test completion

## Security Considerations

- Test images are isolated in private ECR registry
- Artifacts have limited retention (30 days)
- No production credentials or secrets are exposed
- PR validation prevents unauthorized access

## Limitations

- Only supports single PR testing (no batch testing)
- Requires manual trigger (no automatic PR testing)
- Limited to AWS ECR private registry
- Test manifests reference private images (not suitable for production)
- Test artifacts are only available as workflow artifacts (not as releases)
- Artifacts have limited retention (30 days)

## Related Documentation

- [Release Process](../README.md)
- [Makefile Targets](../Makefile)
- [GitHub Actions Workflows](../.github/workflows/)