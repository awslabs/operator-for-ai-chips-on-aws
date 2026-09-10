#!/usr/bin/env bash
#
# bootstrap-efs.sh - the one step of this deployment that cannot be GitOps driven.
#
# Mounting EFS from ROSA needs an IAM role whose trust policy references the
# cluster's OIDC provider. Creating that role needs AWS credentials, and nothing
# in the cluster has any until the role exists. So the AWS side is created here,
# once, and everything after it is declarative.
#
# Creates in AWS: an IAM policy and a role trusted by the two EFS CSI driver
# service accounts, an EFS filesystem, a security group allowing NFS from the
# workers, and one mount target per worker subnet.
#
# Creates in the cluster: an ArgoCD cluster Secret carrying the filesystem ID and
# role ARN. Those are the only two facts unknowable until AWS is provisioned, so
# they are the only two this feeds to GitOps. Model, tensor parallelism, cache
# sizes and namespaces are deployment config and live in the charts' values.yaml,
# under version control.
#
# Re-running is safe: every step adopts existing resources instead of duplicating
# them. Nothing is required as input; the region is derived from the cluster.
#
# Usage: ./bootstrap-efs.sh [--dry-run] [--env-file path]
#        copy efs.env.example to efs.env to override any default
#

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

# Reports an action that only happened for real outside dry-run mode, so
# --dry-run never claims to have created something.
did() {
  if [[ "$DRY_RUN" == "true" ]]; then
    printf '\033[0;33m  would have\033[0m %s\n' "$*"
  else
    printf '\033[0;32m  ok\033[0m %s\n' "$*"
  fi
}

run() {
  if [[ "$DRY_RUN" == "true" ]]; then
    # stderr, so a caller's >/dev/null on the command output cannot swallow it.
    printf '\033[0;33m  would run:\033[0m %s\n' "$*" >&2
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

if [[ -f "$ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  ok "Config ${ENV_FILE}"
else
  ok "No ${ENV_FILE}; using defaults for everything (see efs.env.example to override)"
fi

# Inputs are limited to things this script actually needs to provision AWS
# resources, plus which repo/revision and which components this particular
# cluster should get. Model choice, tensor parallelism, cache sizes and
# namespaces are deployment configuration and live in the charts' values.yaml,
# under version control where they can be reviewed.
#
# Nothing here is required. AWS_REGION is derived from the cluster if unset.
: "${EFS_NAME:=neuron-cache}"
: "${GITOPS_NAMESPACE:=openshift-gitops}"
: "${PERF_REPO_URL:=https://github.com/awslabs/operator-for-ai-chips-on-aws.git}"
: "${PERF_REPO_REVISION:=main}"
: "${EFS_THROUGHPUT_MODE:=elastic}"
: "${EFS_PERFORMANCE_MODE:=generalPurpose}"
: "${EFS_ENCRYPTED:=true}"
: "${ENABLE_STORAGE:=true}"
: "${ENABLE_NEURON_OPERATOR:=true}"
: "${ENABLE_OAI:=true}"

oc whoami >/dev/null 2>&1 || die "Not logged in to a cluster. Run 'oc login' first."
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

# Derive the region from the cluster rather than trusting the caller's aws
# config. EFS and its mount targets have to live in the cluster's own region,
# and a stale default region in ~/.aws/config would otherwise create them
# somewhere the nodes cannot reach.
if [[ -z "${AWS_REGION:-}" ]]; then
  NODE_AZ="$(oc get nodes -o jsonpath='{.items[0].spec.providerID}' \
    | sed -n 's#^aws:///\([a-z0-9-]*\)/.*#\1#p')"
  [[ -n "$NODE_AZ" ]] || die "Could not derive the region from node providerID; set AWS_REGION in $ENV_FILE"
  # AZ to region: us-west-2a -> us-west-2
  AWS_REGION="${NODE_AZ%?}"
  ok "Region ${AWS_REGION} (derived from the cluster)"
else
  ok "Region ${AWS_REGION} (from ${ENV_FILE})"
fi
export AWS_REGION

# One call serves two purposes: it proves the credentials work, and its ARN
# supplies the partition and account for the IAM policy ARN below. Partition is
# read rather than hardcoded so this also works in GovCloud and China.
CALLER="$(aws sts get-caller-identity --query Arn --output text)" \
  || die "Unable to call AWS STS. Check your AWS credentials."
AWS_PARTITION="$(cut -d: -f2 <<<"$CALLER")"
AWS_ACCOUNT="$(cut -d: -f5 <<<"$CALLER")"
ok "AWS identity ${CALLER}"

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

# Ask IAM for the provider's ARN rather than assembling it. If the cluster's
# issuer is not registered as an OIDC provider in this account, the role would be
# created with a trust policy nothing can assume, so fail here with a clear
# reason instead of leaving a broken role behind.
OIDC_ARN="$(aws iam list-open-id-connect-providers \
  --query "OpenIDConnectProviderList[?ends_with(Arn, '/${OIDC_PROVIDER}')].Arn | [0]" \
  --output text 2>/dev/null || true)"
if [[ -z "$OIDC_ARN" || "$OIDC_ARN" == "None" ]]; then
  die "No IAM OIDC provider matches the cluster issuer ${OIDC_PROVIDER} in this account.
     Either your AWS credentials point at a different account than the cluster,
     or the cluster's OIDC provider was never registered."
fi
ok "OIDC provider ${OIDC_ARN}"

# ---------------------------------------------------------------------------
# Confirm
# ---------------------------------------------------------------------------

POLICY_NAME="${INFRA_NAME}-aws-efs-csi"
ROLE_NAME="${INFRA_NAME}-aws-efs-csi-operator"
EFS_SG_NAME="${INFRA_NAME}-efs-mt"
CLUSTER_SECRET_NAME="${INFRA_NAME}-neuron-perf"

cat <<SUMMARY

This will create, in ${AWS_REGION}:
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

# An IAM policy ARN is fully determined by partition, account and name, so build
# it rather than searching. aws iam list-policies paginates, and with --output
# text the --query is applied per page, which returns one line per page.
POLICY_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT}:policy/${POLICY_NAME}"

if aws iam get-policy --policy-arn "$POLICY_ARN" >/dev/null 2>&1; then
  ok "Already exists, reusing ${POLICY_ARN}"
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
  did "created IAM policy ${POLICY_ARN}"
fi

# ---------------------------------------------------------------------------
# IAM role
#
# The sub condition is a list, so one role can serve several service accounts.
# Both EFS CSI service accounts are included.
# ---------------------------------------------------------------------------

log "IAM role ${ROLE_NAME}"

TRUST_DOC="$(jq -n \
  --arg provider_arn "${OIDC_ARN}" \
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

ROLE_ARN="$(aws iam get-role --role-name "$ROLE_NAME" \
  --query Role.Arn --output text 2>/dev/null || true)"

if [[ -n "$ROLE_ARN" && "$ROLE_ARN" != "None" ]]; then
  ok "Already exists, refreshing trust policy"
  run aws iam update-assume-role-policy --role-name "$ROLE_NAME" \
    --policy-document "$TRUST_DOC"
else
  run aws iam create-role --role-name "$ROLE_NAME" \
    --assume-role-policy-document "$TRUST_DOC" \
    --description "EFS CSI driver for OpenShift cluster ${INFRA_NAME}" >/dev/null
  ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT}:role/${ROLE_NAME}"
  did "created IAM role ${ROLE_ARN}"
fi

run aws iam attach-role-policy --role-name "$ROLE_NAME" --policy-arn "$POLICY_ARN"
did "attached policy to ${ROLE_NAME}"

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
    did "created security group ${EFS_SG}"
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
    did "created mount target in ${subnet}"
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
    vpc_id: "${VPC_ID}"
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
  did "applied the Secret"
fi

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------

APPSET_URL="${PERF_REPO_URL%.git}/raw/${PERF_REPO_REVISION}/deploy/argocd/applicationset-performance.yaml"

log "Bootstrap complete"
ok "filesystem ${FS_ID}"
ok "role ${ROLE_ARN}"
ok "config Secret ${CLUSTER_SECRET_NAME} in ${GITOPS_NAMESPACE}"

cat <<NEXT

Apply the deployment, unmodified:

  oc apply -f ${APPSET_URL}
  oc get applications -n ${GITOPS_NAMESPACE}

NEXT
