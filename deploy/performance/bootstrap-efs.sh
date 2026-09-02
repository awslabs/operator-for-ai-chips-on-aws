#!/usr/bin/env bash
#
# bootstrap-efs.sh — the one step of the performance deployment that cannot be
# GitOps driven.
#
# Why this script exists: mounting EFS from a ROSA cluster requires an IAM role
# whose trust policy references the cluster's OIDC provider. Creating that role
# requires AWS credentials, and nothing running inside the cluster has any until
# the role exists. So the AWS-side prerequisites are created here, once, and
# everything in-cluster afterwards is declarative.
#
# What it creates in AWS:
#   - an IAM policy with the EFS CSI driver permissions
#   - an IAM role trusted by the two EFS CSI driver service accounts
#   - an EFS filesystem (adopted if one already exists for this cluster)
#   - a security group allowing NFS from the worker nodes
#   - one mount target per worker subnet
#
# What it creates in the cluster:
#   - an ArgoCD cluster Secret carrying the filesystem ID, role ARN and workload
#     settings as annotations. That Secret is the config bus the ApplicationSet
#     reads, which is why no Application ever needs patching.
#
# Re-running is safe. Every step adopts existing resources rather than
# duplicating them.
#
# Usage:
#   cp efs.env.example efs.env && $EDITOR efs.env
#   ./bootstrap-efs.sh [--dry-run] [--env-file path]

set -euo pipefail

ENV_FILE="$(dirname "$0")/efs.env"
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)  DRY_RUN=true; shift ;;
    --env-file) ENV_FILE="$2"; shift 2 ;;
    -h|--help)  sed -n '2,30p' "$0"; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done

log()  { printf '\033[0;34m==>\033[0m %s\n' "$*"; }
ok()   { printf '\033[0;32m  ok\033[0m %s\n' "$*"; }
warn() { printf '\033[0;33m  !!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[0;31mERROR\033[0m %s\n' "$*" >&2; exit 1; }

run() {
  if [[ "$DRY_RUN" == "true" ]]; then
    printf '\033[0;33m  would run:\033[0m %s\n' "$*"
    return 0
  fi
  "$@"
}

# ---------------------------------------------------------------------------
# Preflight
# ---------------------------------------------------------------------------

log "Preflight"

for tool in aws oc jq; do
  command -v "$tool" >/dev/null || die "$tool is required but not on PATH"
done

[[ -f "$ENV_FILE" ]] || die "Config file not found: $ENV_FILE (copy efs.env.example to efs.env)"
# shellcheck disable=SC1090
source "$ENV_FILE"

: "${AWS_REGION:?AWS_REGION must be set in $ENV_FILE}"
: "${EFS_NAME:?EFS_NAME must be set in $ENV_FILE}"
: "${GITOPS_NAMESPACE:=openshift-gitops}"
: "${PERF_REPO_URL:?PERF_REPO_URL must be set in $ENV_FILE}"
: "${PERF_REPO_REVISION:=main}"
: "${SERVING_NAMESPACE:=neuron-inference}"
: "${MODEL_NAME:?MODEL_NAME must be set in $ENV_FILE}"
: "${TENSOR_PARALLEL_SIZE:=2}"
: "${STORAGE_CLASS_NAME:=efs-sc}"
: "${COMPILE_CACHE_SIZE:=50Gi}"
: "${MODEL_CACHE_SIZE:=100Gi}"
: "${EFS_THROUGHPUT_MODE:=elastic}"
: "${EFS_PERFORMANCE_MODE:=generalPurpose}"
: "${EFS_ENCRYPTED:=true}"
: "${ENABLE_STORAGE:=true}"
: "${ENABLE_NEURON_OPERATOR:=true}"
: "${ENABLE_OAI:=true}"

export AWS_REGION

oc whoami >/dev/null 2>&1 || die "Not logged in to a cluster. Run 'oc login' first."
ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)" \
  || die "Unable to call AWS STS. Check your AWS credentials."
ok "AWS account ${ACCOUNT_ID}, region ${AWS_REGION}"
ok "Cluster user $(oc whoami)"

# ---------------------------------------------------------------------------
# Discover cluster infrastructure
#
# Read it from the worker nodes themselves rather than relying on resource tags,
# which vary between ROSA Classic, ROSA HCP and self-managed OCP.
# ---------------------------------------------------------------------------

log "Discovering cluster infrastructure"

INFRA_NAME="$(oc get infrastructure cluster -o jsonpath='{.status.infrastructureName}')"
[[ -n "$INFRA_NAME" ]] || die "Could not read .status.infrastructureName from the infrastructure CR"
ok "Infrastructure name ${INFRA_NAME}"

# providerID looks like aws:///us-east-2a/i-0123456789abcdef0
# Built with a read loop rather than mapfile, which does not exist in the bash 3.2
# that ships with macOS.
INSTANCE_IDS=()
while IFS= read -r id; do
  [[ -n "$id" ]] && INSTANCE_IDS+=("$id")
done < <(
  oc get nodes -o jsonpath='{range .items[*]}{.spec.providerID}{"\n"}{end}' \
    | sed -n 's#.*/\(i-[0-9a-f]\{1,\}\)$#\1#p' | sort -u
)
[[ ${#INSTANCE_IDS[@]} -gt 0 ]] || die "Found no EC2 instance IDs on any node providerID"
ok "Found ${#INSTANCE_IDS[@]} node instance(s)"

INSTANCE_JSON="$(aws ec2 describe-instances --instance-ids "${INSTANCE_IDS[@]}" \
  --query 'Reservations[].Instances[].{Vpc:VpcId,Subnet:SubnetId,Sgs:SecurityGroups[].GroupId}' \
  --output json)"

VPC_ID="$(jq -r '[.[].Vpc] | unique | .[0]' <<<"$INSTANCE_JSON")"
[[ "$VPC_ID" != "null" && -n "$VPC_ID" ]] || die "Could not determine the VPC from node instances"
VPC_COUNT="$(jq -r '[.[].Vpc] | unique | length' <<<"$INSTANCE_JSON")"
[[ "$VPC_COUNT" == "1" ]] || die "Nodes span ${VPC_COUNT} VPCs; EFS mount targets need a single VPC"

SUBNET_IDS=()
while IFS= read -r sn; do
  [[ -n "$sn" ]] && SUBNET_IDS+=("$sn")
done < <(jq -r '[.[].Subnet] | unique | .[]' <<<"$INSTANCE_JSON")
WORKER_SG="$(jq -r '[.[].Sgs[]] | unique | .[0]' <<<"$INSTANCE_JSON")"

ok "VPC ${VPC_ID}"
ok "Subnets ${SUBNET_IDS[*]}"
ok "Worker security group ${WORKER_SG}"

OIDC_PROVIDER="$(oc get authentication.config.openshift.io cluster \
  -o jsonpath='{.spec.serviceAccountIssuer}' | sed -e 's#^https://##')"
[[ -n "$OIDC_PROVIDER" ]] || die "Cluster has no serviceAccountIssuer; this does not look like an STS cluster"
ok "OIDC provider ${OIDC_PROVIDER}"

# ---------------------------------------------------------------------------
# Confirm
# ---------------------------------------------------------------------------

POLICY_NAME="${INFRA_NAME}-aws-efs-csi"
ROLE_NAME="${INFRA_NAME}-aws-efs-csi-operator"
EFS_SG_NAME="${INFRA_NAME}-efs-mt"
CLUSTER_SECRET_NAME="${INFRA_NAME}-neuron-perf"

cat <<SUMMARY

This will create, in AWS account ${ACCOUNT_ID} / ${AWS_REGION}:
  IAM policy          ${POLICY_NAME}
  IAM role            ${ROLE_NAME}
  EFS filesystem      ${EFS_NAME}  (adopted if it already exists)
  Security group      ${EFS_SG_NAME}  (NFS 2049 from ${WORKER_SG})
  Mount targets       one per subnet: ${SUBNET_IDS[*]}

and in the cluster:
  Secret              ${CLUSTER_SECRET_NAME} in ${GITOPS_NAMESPACE}

SUMMARY

if [[ "$DRY_RUN" != "true" ]]; then
  read -r -p "Proceed? [y/N] " reply
  [[ "$reply" == "y" || "$reply" == "Y" ]] || die "Aborted by user"
fi

# ---------------------------------------------------------------------------
# IAM policy
#
# Permissions are the set documented for the AWS EFS CSI Driver Operator on
# STS clusters. Access point create and delete are tag-scoped so this role
# cannot touch access points it did not create.
# ---------------------------------------------------------------------------

log "IAM policy ${POLICY_NAME}"

POLICY_ARN="arn:aws:iam::${ACCOUNT_ID}:policy/${POLICY_NAME}"

if aws iam get-policy --policy-arn "$POLICY_ARN" >/dev/null 2>&1; then
  ok "Already exists, reusing"
else
  POLICY_DOC="$(cat <<'JSON'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "elasticfilesystem:DescribeAccessPoints",
        "elasticfilesystem:DescribeFileSystems",
        "elasticfilesystem:DescribeMountTargets",
        "elasticfilesystem:TagResource",
        "ec2:DescribeAvailabilityZones"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": "elasticfilesystem:CreateAccessPoint",
      "Resource": "*",
      "Condition": {
        "StringLike": { "aws:RequestTag/efs.csi.aws.com/cluster": "true" }
      }
    },
    {
      "Effect": "Allow",
      "Action": "elasticfilesystem:DeleteAccessPoint",
      "Resource": "*",
      "Condition": {
        "StringEquals": { "aws:ResourceTag/efs.csi.aws.com/cluster": "true" }
      }
    }
  ]
}
JSON
)"
  run aws iam create-policy --policy-name "$POLICY_NAME" \
    --policy-document "$POLICY_DOC" >/dev/null
  ok "Created ${POLICY_ARN}"
fi

# ---------------------------------------------------------------------------
# IAM role
#
# The sub condition is a list, so one role can serve several service accounts.
# Both EFS CSI service accounts are included.
# ---------------------------------------------------------------------------

log "IAM role ${ROLE_NAME}"

TRUST_DOC="$(jq -n \
  --arg provider_arn "arn:aws:iam::${ACCOUNT_ID}:oidc-provider/${OIDC_PROVIDER}" \
  --arg sub_key "${OIDC_PROVIDER}:sub" \
  '{
    Version: "2012-10-17",
    Statement: [{
      Effect: "Allow",
      Principal: { Federated: $provider_arn },
      Action: "sts:AssumeRoleWithWebIdentity",
      Condition: {
        StringEquals: {
          ($sub_key): [
            "system:serviceaccount:openshift-cluster-csi-drivers:aws-efs-csi-driver-operator",
            "system:serviceaccount:openshift-cluster-csi-drivers:aws-efs-csi-driver-controller-sa"
          ]
        }
      }
    }]
  }')"

if aws iam get-role --role-name "$ROLE_NAME" >/dev/null 2>&1; then
  ok "Already exists, refreshing trust policy"
  run aws iam update-assume-role-policy --role-name "$ROLE_NAME" \
    --policy-document "$TRUST_DOC"
else
  run aws iam create-role --role-name "$ROLE_NAME" \
    --assume-role-policy-document "$TRUST_DOC" \
    --description "EFS CSI driver for OpenShift cluster ${INFRA_NAME}" >/dev/null
  ok "Created"
fi

run aws iam attach-role-policy --role-name "$ROLE_NAME" --policy-arn "$POLICY_ARN"
ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${ROLE_NAME}"
ok "Policy attached, role ARN ${ROLE_ARN}"

# ---------------------------------------------------------------------------
# EFS filesystem
#
# Adopt by Name tag plus cluster tag. Creating a second filesystem on a re-run
# would silently abandon a warm cache, so matching both tags matters.
# ---------------------------------------------------------------------------

log "EFS filesystem ${EFS_NAME}"

FS_ID="$(aws efs describe-file-systems \
  --query "FileSystems[?Tags[?Key=='Name' && Value=='${EFS_NAME}'] && Tags[?Key=='neuron-perf-cluster' && Value=='${INFRA_NAME}']].FileSystemId | [0]" \
  --output text 2>/dev/null || true)"

if [[ -n "$FS_ID" && "$FS_ID" != "None" ]]; then
  ok "Adopting existing filesystem ${FS_ID}"
else
  if [[ "$DRY_RUN" == "true" ]]; then
    warn "Would create a filesystem; using placeholder ID for the rest of this dry run"
    FS_ID="fs-DRYRUN"
  else
    ENCRYPT_FLAG="--no-encrypted"
    [[ "$EFS_ENCRYPTED" == "true" ]] && ENCRYPT_FLAG="--encrypted"
    FS_ID="$(aws efs create-file-system \
      --performance-mode "$EFS_PERFORMANCE_MODE" \
      --throughput-mode "$EFS_THROUGHPUT_MODE" \
      $ENCRYPT_FLAG \
      --tags "Key=Name,Value=${EFS_NAME}" \
             "Key=neuron-perf-cluster,Value=${INFRA_NAME}" \
      --query FileSystemId --output text)"
    ok "Created ${FS_ID}, waiting for it to become available"
    for _ in $(seq 1 60); do
      state="$(aws efs describe-file-systems --file-system-id "$FS_ID" \
        --query 'FileSystems[0].LifeCycleState' --output text)"
      [[ "$state" == "available" ]] && break
      sleep 5
    done
    [[ "$state" == "available" ]] || die "Filesystem ${FS_ID} did not become available"
    ok "Available"
  fi
fi

# ---------------------------------------------------------------------------
# Security group for the mount targets
#
# A dedicated group rather than a rule on the worker group, so removing this
# deployment cannot leave stray rules on cluster-managed groups.
# ---------------------------------------------------------------------------

log "Security group ${EFS_SG_NAME}"

EFS_SG="$(aws ec2 describe-security-groups \
  --filters "Name=group-name,Values=${EFS_SG_NAME}" "Name=vpc-id,Values=${VPC_ID}" \
  --query 'SecurityGroups[0].GroupId' --output text 2>/dev/null || true)"

if [[ -n "$EFS_SG" && "$EFS_SG" != "None" ]]; then
  ok "Already exists, reusing ${EFS_SG}"
else
  if [[ "$DRY_RUN" == "true" ]]; then
    EFS_SG="sg-DRYRUN"
    warn "Would create security group; using placeholder"
  else
    EFS_SG="$(aws ec2 create-security-group --group-name "$EFS_SG_NAME" \
      --description "NFS access to EFS for OpenShift cluster ${INFRA_NAME}" \
      --vpc-id "$VPC_ID" --query GroupId --output text)"
    ok "Created ${EFS_SG}"
  fi
fi

# Idempotent: AWS rejects a duplicate rule, which is not an error here.
if [[ "$DRY_RUN" != "true" ]]; then
  if aws ec2 authorize-security-group-ingress --group-id "$EFS_SG" \
       --protocol tcp --port 2049 --source-group "$WORKER_SG" >/dev/null 2>&1; then
    ok "Allowed NFS 2049 from ${WORKER_SG}"
  else
    ok "NFS 2049 from ${WORKER_SG} already allowed"
  fi
else
  run aws ec2 authorize-security-group-ingress --group-id "$EFS_SG" \
    --protocol tcp --port 2049 --source-group "$WORKER_SG"
fi

# ---------------------------------------------------------------------------
# Mount targets, one per subnet holding worker nodes
# ---------------------------------------------------------------------------

log "Mount targets"

EXISTING_MT_SUBNETS=""
if [[ "$FS_ID" != "fs-DRYRUN" ]]; then
  EXISTING_MT_SUBNETS="$(aws efs describe-mount-targets --file-system-id "$FS_ID" \
    --query 'MountTargets[].SubnetId' --output text 2>/dev/null || true)"
fi

for subnet in "${SUBNET_IDS[@]}"; do
  if grep -qw "$subnet" <<<"$EXISTING_MT_SUBNETS"; then
    ok "${subnet} already has a mount target"
  else
    run aws efs create-mount-target --file-system-id "$FS_ID" \
      --subnet-id "$subnet" --security-groups "$EFS_SG" >/dev/null
    ok "Created mount target in ${subnet}"
  fi
done

# ---------------------------------------------------------------------------
# The config bus: an ArgoCD cluster Secret
#
# Labels select and toggle, annotations carry data. The ApplicationSet reads both
# through its cluster generator, so no Application ever has to be patched and
# re-applying this repo cannot clobber these values.
#
# ArgoCD's default local cluster has no Secret, and a selector on
# argocd.argoproj.io/secret-type excludes it, so the local cluster gets one here.
# ---------------------------------------------------------------------------

log "ArgoCD cluster Secret ${CLUSTER_SECRET_NAME} in ${GITOPS_NAMESPACE}"

oc get namespace "$GITOPS_NAMESPACE" >/dev/null 2>&1 \
  || die "Namespace ${GITOPS_NAMESPACE} not found. Install the OpenShift GitOps operator first."

SECRET_MANIFEST="$(cat <<YAML
apiVersion: v1
kind: Secret
metadata:
  name: ${CLUSTER_SECRET_NAME}
  namespace: ${GITOPS_NAMESPACE}
  labels:
    argocd.argoproj.io/secret-type: cluster
    neuron_perf: "true"
    enable_storage: "${ENABLE_STORAGE}"
    enable_neuron_operator: "${ENABLE_NEURON_OPERATOR}"
    enable_oai: "${ENABLE_OAI}"
  annotations:
    perf_repo_url: "${PERF_REPO_URL}"
    perf_repo_revision: "${PERF_REPO_REVISION}"
    efs_file_system_id: "${FS_ID}"
    efs_role_arn: "${ROLE_ARN}"
    aws_region: "${AWS_REGION}"
    aws_account_id: "${ACCOUNT_ID}"
    vpc_id: "${VPC_ID}"
    serving_namespace: "${SERVING_NAMESPACE}"
    model_name: "${MODEL_NAME}"
    tensor_parallel_size: "${TENSOR_PARALLEL_SIZE}"
    storage_class_name: "${STORAGE_CLASS_NAME}"
    compile_cache_size: "${COMPILE_CACHE_SIZE}"
    model_cache_size: "${MODEL_CACHE_SIZE}"
type: Opaque
stringData:
  name: "${INFRA_NAME}"
  server: "https://kubernetes.default.svc"
  config: '{"tlsClientConfig":{"insecure":false}}'
YAML
)"

if [[ "$DRY_RUN" == "true" ]]; then
  printf '\033[0;33m  would apply:\033[0m\n%s\n' "$SECRET_MANIFEST"
else
  printf '%s\n' "$SECRET_MANIFEST" | oc apply -f - >/dev/null
  ok "Applied"
fi

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------

APPSET_URL="${PERF_REPO_URL%.git}/raw/${PERF_REPO_REVISION}/deploy/argocd/applicationset-performance.yaml"

cat <<NEXT

$(log "Bootstrap complete")

  EFS filesystem   ${FS_ID}
  IAM role         ${ROLE_ARN}
  Config Secret    ${CLUSTER_SECRET_NAME} in ${GITOPS_NAMESPACE}

Next, apply the deployment. Nothing below needs editing.

  oc apply -f ${APPSET_URL}

Then watch it converge:

  oc get applications -n ${GITOPS_NAMESPACE} -w

The first InferenceService start compiles the model and populates the shared
cache on EFS. That run is slow. Every later pod, on any node, reads the compiled
artifacts from the cache instead of recompiling.

NEXT
