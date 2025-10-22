#!/bin/bash
set -e

PROFILE="aws-os-partners-PartnerSA-admin"
REGION="us-east-1"
DAYS_OLD=${1:-7}  # Default to 7 days, can override with argument

# List of repositories to clean up
REPOSITORIES=(
  "neuron-openshift/operator"
  "neuron-openshift/operator-bundle" 
  "neuron-openshift/operator-index"
)

echo "Cleaning up untagged images older than $DAYS_OLD days..."

for repo in "${REPOSITORIES[@]}"; do
  echo "Cleaning repository: $repo"
  
  # Get untagged images older than specified days
  IMAGES_TO_DELETE=$(aws ecr-public describe-images \
    --repository-name "$repo" \
    --region "$REGION" \
    --profile "$PROFILE" \
    --filter tagStatus=UNTAGGED \
    --query "imageDetails[?imageDigest && imagePushedAt<='$(date -d "$DAYS_OLD days ago" -u +%Y-%m-%dT%H:%M:%SZ)'].imageDigest" \
    --output text)
  
  if [ -n "$IMAGES_TO_DELETE" ] && [ "$IMAGES_TO_DELETE" != "None" ]; then
    echo "Deleting $(echo $IMAGES_TO_DELETE | wc -w) untagged images from $repo"
    for digest in $IMAGES_TO_DELETE; do
      aws ecr-public batch-delete-image \
        --repository-name "$repo" \
        --region "$REGION" \
        --profile "$PROFILE" \
        --image-ids imageDigest="$digest" > /dev/null
    done
    echo "✅ Cleaned up $repo"
  else
    echo "ℹ️  No old untagged images found in $repo"
  fi
done

echo "🎉 Cleanup completed!"