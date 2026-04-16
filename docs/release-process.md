# Release Process Documentation

## Overview

The release process uses the `VERSION` file as the single source of truth. When a PR that updates `VERSION` is merged to `main`, the release workflow automatically builds, tags, and publishes the release.

## How to Create a Release

### 1. Update the VERSION File

Create a PR that updates only the `VERSION` file in the repository root:

```bash
NEW_VERSION="1.2.3"

echo "$NEW_VERSION" > VERSION

git add VERSION
git commit -m "Release version $NEW_VERSION"
git push origin feature/release-$NEW_VERSION
```

> **Note:** You do NOT need to update `Chart.yaml` or `application.yaml` manually. The release workflow handles these automatically:
> - `deploy/helm/aws-neuron-operator/Chart.yaml` — `version` and `appVersion` are set to the release version
> - `deploy/argocd/application.yaml` — `targetRevision` is pinned to the release tag (e.g., `v1.2.3`)
>
> These changes are committed locally by the workflow and exist only on the release tag, not on `main`. On `main`, `Chart.yaml` retains the previous version and `application.yaml` keeps `targetRevision: main`.

### 2. Merge the PR

When the PR is merged to `main`, the release workflow automatically:
1. Reads the version from the `VERSION` file
2. Builds and pushes container images to ECR
3. Generates release manifests
4. Updates `Chart.yaml` and pins `application.yaml` in a local commit
5. Creates a tag (`v1.2.3`) on that commit and pushes the tag only (not the branch)
6. Creates a GitHub Release with artifacts

### 3. Release Artifacts

Each release includes:
- **Container Images**: Pushed to `public.ecr.aws/os-partners/neuron-openshift/`
  - `operator:v1.2.3`
  - `operator-bundle:v1.2.3`
  - `operator-index:v1.2.3`
- **Manifests**: `aws-neuron-operator.yaml`, `nfd-rule.yaml`, `deviceconfig-sample.yaml`
- **Git Tag**: `v1.2.3` (includes pinned `Chart.yaml` and `application.yaml`)

### Customer Deployment

Customers deploy using the Argo application file from the release tag:

```bash
oc apply -f https://raw.githubusercontent.com/awslabs/operator-for-ai-chips-on-aws/v1.2.3/deploy/argocd/application.yaml
```

This file has `targetRevision` pinned to `v1.2.3`, ensuring a stable, reproducible deployment.

## Version Format

Use semantic versioning (X.Y.Z):
- **Major** (X): Breaking changes
- **Minor** (Y): New features, backward compatible
- **Patch** (Z): Bug fixes, backward compatible

## Republishing a Release

To republish an existing release (e.g., after fixing a build issue):

1. Go to **Actions → Release → Run workflow**
2. Enter the version to republish (e.g., `1.2.3`)
3. Click **Run workflow**

The workflow will delete the existing tag, rebuild everything, and create a new tag and release.

## How It Works (Branch Protection)

The `main` branch is protected and requires pull requests. The release workflow works around this by:
- Making file changes (Chart.yaml, application.yaml) in a **local commit** that is never pushed to `main`
- Creating a **tag** on that local commit and pushing only the tag
- Tags are not subject to branch protection rules

This means:
- On `main`: `Chart.yaml` has the previous version, `application.yaml` has `targetRevision: main`
- On the tag: `Chart.yaml` has the release version, `application.yaml` has `targetRevision: v1.2.3`

## Troubleshooting

### Common Issues

**Error: "Tag v1.2.3 already exists"**
- Use `workflow_dispatch` to re-run — it deletes the existing tag automatically

**Error: "Version '1.2' is not a valid semantic version"**
- Use full semantic version format (X.Y.Z), e.g., `1.2.0`

**Error: "VERSION file not found"**
- Ensure the `VERSION` file exists in the repository root with only the version number

**Release became a draft after tag recreation**
- Go to the releases page, click the draft release, click **Edit**, then **Publish release**
