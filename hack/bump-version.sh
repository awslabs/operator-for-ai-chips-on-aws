#!/bin/bash
set -e

CURRENT_VERSION=$(cat VERSION)
BUMP_TYPE=${1:-patch}

if ! command -v semver &> /dev/null; then
    echo "Installing semver tool..."
    npm install -g semver 2>/dev/null || {
        echo "Error: semver tool required. Install with: npm install -g semver"
        exit 1
    }
fi

case $BUMP_TYPE in
    major|minor|patch)
        NEW_VERSION=$(semver -i $BUMP_TYPE $CURRENT_VERSION)
        ;;
    *)
        echo "Usage: $0 [major|minor|patch]"
        echo "Current version: $CURRENT_VERSION"
        exit 1
        ;;
esac

echo "Bumping version from $CURRENT_VERSION to $NEW_VERSION"
echo $NEW_VERSION > VERSION

echo "Updated VERSION file to $NEW_VERSION"
echo "Next steps:"
echo "  1. git add VERSION"
echo "  2. git commit -m 'Bump version to $NEW_VERSION'"
echo "  3. git tag v$NEW_VERSION"
echo "  4. git push origin main --tags"