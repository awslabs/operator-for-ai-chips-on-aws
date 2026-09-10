# Performance-optimized Neuron inference on ROSA

Shared-storage Neuron compile cache, OpenShift AI, and KServe serving, installed
GitOps style. This directory is the entry point; the manifests live elsewhere in
the repo and are listed under [Where things live](#where-things-live).

## The problem this solves

A Neuron model has to be compiled before it can serve. On a cold start that
compile, plus the model download, is the dominant cost. The Neuron SDK already
caches compiled artifacts on disk, but in the manifests under
`deploy/examples/` that cache is either an `emptyDir`, which is discarded when
the pod restarts, or a `ReadWriteOnce` gp3 volume, which only pods on one node
can share.

This deployment puts the cache on EFS with `ReadWriteMany`. The first pod
compiles, and every pod after it reads the result, including pods on other nodes
and pods created by scaling out. Rescheduling and scale-out stop paying compile
cost, not just restarts in place.

## What is not GitOps, and why

One step needs AWS credentials and cannot be declarative.

Mounting EFS from ROSA requires an IAM role whose trust policy references the
cluster's OIDC provider. Creating that role requires AWS credentials, and nothing
inside the cluster has any until the role exists. There is no way for a cluster
to create its own first IAM role.

So `bootstrap-efs.sh` creates the AWS prerequisites once, and everything
in-cluster after that is declarative. Created outside GitOps:

| Resource | Name |
|---|---|
| IAM policy | `<infra-name>-aws-efs-csi` |
| IAM role | `<infra-name>-aws-efs-csi-operator` |
| EFS filesystem | tagged `Name=$EFS_NAME` and `neuron-perf-cluster=<infra-name>` |
| Security group | `<infra-name>-efs-mt`, NFS 2049 from the worker group |
| Mount targets | one per subnet with worker nodes |
| ArgoCD cluster Secret | `<infra-name>-neuron-perf` |

Everything else, the EFS CSI driver, StorageClass, PVCs, the Neuron operator,
OpenShift AI, KServe, the serving runtime, is managed by ArgoCD.

## Install

Prerequisites: a ROSA cluster with Neuron nodes (inf2, trn1, trn2), `oc` logged
in as cluster-admin, `aws` configured, and `jq`.

### 1. Create the AWS prerequisites

Download and read the script before running it. It creates IAM roles, so it
deserves a look.

```bash
curl -O https://raw.githubusercontent.com/awslabs/operator-for-ai-chips-on-aws/main/deploy/performance/bootstrap-efs.sh
chmod +x bootstrap-efs.sh

./bootstrap-efs.sh --dry-run   # see what it would do
./bootstrap-efs.sh
```

No arguments and no config file are required. The region is derived from the
cluster's own nodes, which is safer than a default in `~/.aws/config`, since EFS
and its mount targets have to live in the cluster's region.

To override a default, take `efs.env.example` and edit it:

```bash
curl -O https://raw.githubusercontent.com/awslabs/operator-for-ai-chips-on-aws/main/deploy/performance/efs.env.example
cp efs.env.example efs.env && $EDITOR efs.env
```

What that file covers: the EFS filesystem name and its throughput, performance
and encryption settings; the GitOps namespace; which repo and revision this
cluster syncs from; and which of the three components this cluster gets.

What it deliberately does not cover: the model, tensor parallelism, cache sizes
and namespaces. Those are deployment configuration and live in the charts'
`values.yaml`, under version control where a change is a reviewable commit
rather than an annotation written by a script on someone's laptop.

Re-running is safe. Every step adopts existing resources instead of creating
duplicates, which matters most for the filesystem: a second filesystem would
silently abandon a warm cache.

### 2. Install the OpenShift GitOps operator

Unchanged from [../README.md](../README.md).

```bash
oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: openshift-gitops-operator
  namespace: openshift-operators
spec:
  channel: latest
  name: openshift-gitops-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF
```

### 3. Apply the deployment

One file, applied as-is. No editing, no patching.

```bash
oc apply -f https://raw.githubusercontent.com/awslabs/operator-for-ai-chips-on-aws/main/deploy/argocd/applicationset-performance.yaml
```

For a pinned deployment, replace `main` with a release tag. Tracking `main`
means an upstream merge can change this cluster.

## How configuration reaches the charts

Two categories, kept deliberately separate.

**Bootstrap facts** cannot be known until AWS is provisioned: the filesystem ID
and the role ARN. The script writes those, plus which repo this cluster syncs
from, as annotations on one Secret in `openshift-gitops` that doubles as the
ArgoCD cluster entry. Labels on the same Secret select which components the
cluster gets.

```bash
oc get secret -n openshift-gitops -l neuron_perf=true -o yaml
```

Only `efs_file_system_id` and `efs_role_arn` are injected into a chart. This is
why no Application ever needs patching: a patch stores cluster-specific state on
an object git owns, so re-applying the repo or ArgoCD self-healing reverts it.
Putting the values on a separate object avoids that.

**Deployment configuration** is everything else: model, tensor parallelism, cache
sizes, namespaces, probe timings. That lives in `values.yaml` in the charts, where
changing it is a reviewable commit. It is not routed through the Secret, because
a shell script writing model choices into annotations would put deployment config
outside version control, which is the opposite of what GitOps is for.

Toggle components by editing `ENABLE_*` in `efs.env` and re-running the script,
which is the source of truth:

```bash
sed -i 's/^ENABLE_OAI=.*/ENABLE_OAI=false/' efs.env
./bootstrap-efs.sh
```

Relabelling the Secret directly also works and takes effect within a minute:

```bash
oc label secret -n openshift-gitops <infra-name>-neuron-perf enable_oai=false --overwrite
```

but it is a temporary override. The script re-applies the Secret from `efs.env`
on its next run, so a manual label is silently reverted then. Put the decision in
`efs.env` if you want it to stick.

Turning a component off removes its Application and, because ApplicationSets
default to `preserveResourcesOnDeletion: false`, its resources too. Verified:
disabling `enable_oai` removed the `neuron-inference`, `openshift-serverless` and
`redhat-ods-operator` namespaces and the operator Subscriptions it had created.
An already-installed CSV survives, since deleting a Subscription does not
uninstall the operator; remove those by hand if you want them gone.

ArgoCD's default local cluster has no Secret, and any selector on
`argocd.argoproj.io/secret-type` excludes it, which is why the script creates one
pointing at `https://kubernetes.default.svc`.

## Where things live

| Path | What |
|---|---|
| `deploy/performance/` | this README, `bootstrap-efs.sh`, `efs.env.example` |
| `deploy/argocd/applicationset-performance.yaml` | the single file customers apply |
| `deploy/helm/efs-storage/` | EFS CSI operator, `ClusterCSIDriver`, StorageClass |
| `deploy/helm/oai-hardened/` | OpenShift AI, KServe, serving runtime, cache PVCs |
| `deploy/helm/aws-neuron-operator/` | the Neuron operator, reused unmodified |

The charts are also installable directly for anyone not using GitOps:

```bash
helm install efs-storage deploy/helm/efs-storage \
  --set efs.fileSystemId=fs-... --set efs.roleARN=arn:aws:iam::...:role/...
helm install oai-hardened deploy/helm/oai-hardened
```

## Verify

```bash
oc get applications -n openshift-gitops
oc get clustercsidriver efs.csi.aws.com -o yaml | grep -A5 conditions
oc get pvc -n neuron-inference
oc get inferenceservice -n neuron-inference
```

The three Applications are independent and are not ordered relative to each
other, because ArgoCD sync waves order resources within one Application, not
across several. This is fine: a PVC created before the EFS driver is running
stays `Pending` and binds once the driver is up. Each chart orders its own
internals with waves.

To confirm the cache is actually being used, watch for the compiler reporting a
cache hit rather than a compile:

```bash
oc logs -n neuron-inference -l serving.kserve.io/inferenceservice=llama31-8b-neuron \
  | grep -i "cache"
```

The Neuron SDK logs `Using a cached neff at ...` on a hit.

## Validated on

See [VALIDATION.md](VALIDATION.md) for the reproducible procedure and the test
manifests under `tests/`.

Storage and operator layers were tested end to end on ROSA 4.22.11 (OCP
4.22.11, one `inf2.8xlarge` in us-west-2a plus two `m5.xlarge` across
us-west-2a/2b):

- `bootstrap-efs.sh` discovered VPC, both worker subnets, the worker security
  group, the OIDC provider and the region from the cluster with no config file at
  all, and created the IAM policy, role, filesystem, security group and two mount
  targets. A second run adopted every resource rather than duplicating it,
  including the filesystem, so a re-run does not abandon a warm cache.
- The ArgoCD cluster generator matched the Secret and produced the Applications.
  Setting `enable_oai=false` suppressed that Application, and turning it back off
  after an accidental enable removed the three namespaces and the operator
  Subscriptions it had created, confirming both the toggle and its rollback.
- EFS CSI Driver Operator reached `Succeeded`, and all four `ClusterCSIDriver`
  conditions went True. The `ROLEARN` path produced an
  `aws-efs-cloud-credentials` secret containing `role_arn` and
  `web_identity_token_file`, which is what makes STS work.
- Dynamic provisioning worked: a `ReadWriteMany` PVC bound in about 20 seconds
  and the driver created an EFS access point with `775` and uid/gid 1000.
- Sharing was verified across nodes and availability zones. A pod on the
  `inf2.8xlarge` in us-west-2a wrote an 8 MB artifact; a pod on an `m5.xlarge` in
  us-west-2b read it back and appended to it. This is the property the whole
  design depends on, and it exercises both mount targets.
- Deleting the PVC removed the access point and left the filesystem intact.
- ArgoCD held no `cluster-admin` binding. The default application-controller
  role grants only read-only wildcard access, so the scoped ClusterRoles in
  these charts supplied the writes.
- The Neuron operator installed via the same flow (`aws-neuron-operator.v1.3.0`,
  KMM 2.7.0), the kernel module built, and the node began advertising
  `aws.amazon.com/neuron: 1` and `aws.amazon.com/neuroncore: 2`. Both
  Applications were still Synced and Healthy 16 hours later with no drift.

Not yet validated: the OpenShift AI layer (`enable_oai`), the `ServingRuntime`,
and the `InferenceService`. Those need a Hugging Face token for a gated model and
more worker capacity than this cluster had. In particular **no vLLM startup time
has been measured**, warm or cold, so this deployment makes no claim about how
much faster serving starts. [VALIDATION.md](VALIDATION.md) level 3 is the
procedure for producing that number.

## Known gaps and things to verify on your cluster

Stated plainly rather than discovered later.

- **EFS small-file latency is real but irrelevant here.** Measured on a ROSA
  4.22 cluster (`inf2.8xlarge`, elastic throughput, generalPurpose):

  | | write 300×32k | read all 300 | write 256 MB | read 256 MB |
  |---|---|---|---|---|
  | local `emptyDir` | 515 ms | 8 ms | 1185 ms | 27 ms |
  | EFS RWX | 5598 ms | 716 ms | 846 ms | 608 ms |

  Per-file overhead is significant, and the local read figures are flattered by
  page cache, so treat the ratios as an upper bound on EFS's disadvantage. What
  matters is the absolute cost: reading 300 cached files took 716 ms. Even a
  cache of several thousand files stays in the low seconds, against a
  compilation measured in minutes. So copying from EFS into a local `emptyDir`
  at pod start is not worth the complexity, and this chart does not do it.

  Note this measures storage, not an end-to-end cold start. No speedup figure
  for actual model serving is claimed, because that has not been measured.
- **The model-download Job and the serving pods are not `restricted` PSA
  compliant by default.** The Job now sets a compliant `securityContext`, but
  the vLLM runtime image has not been checked against `enforce=restricted`.
- **`NEURON_CACHE_URL`**, used by the manifests in `deploy/examples/`, does not
  appear in the AWS Neuron persistent cache documentation. The documented
  variables are `NEURON_COMPILE_CACHE_URL`, which this chart sets, and
  `NEURON_CC_FLAGS --cache_dir`, which takes precedence over it. If your image
  needs the other variable, add it via `servingRuntime.extraEnv`.
- **OperatorGroup conflict.** GitOps has to create an OperatorGroup in
  `openshift-cluster-csi-drivers`. This was verified to work on a cluster that
  had none. If yours already has one, set
  `efsCsiOperator.createOperatorGroup=false` or the sync will conflict.
- **ArgoCD reports Progressing indefinitely.** There is no built-in health
  assessment for `DataScienceCluster` or `InferenceService`. Add custom health
  checks to the ArgoCD CR if a permanently yellow Application is a problem.
- **ApplicationSet features.** The entrypoint uses `goTemplate` and the cluster
  generator. Confirm your OpenShift GitOps version supports them.
- **Probe timings.** `servingRuntime.probeInitialDelaySeconds` defaults to 900,
  sized for an uncached cold start. Once the cache is warm this is far longer than
  needed, but lowering it before the first successful compile risks the kubelet
  killing the pod mid-compile.
- **vLLM flags.** `--no-enable-prefix-caching` and `--no-enable-chunked-prefill`
  match the existing example and are left off. They affect steady-state
  throughput rather than startup. Whether they are safe on the Neuron backend was
  not verified.
- **EFS limits.** At most 1000 access points per filesystem, so at most 1000 PVs
  per StorageClass. PVC size requests are not enforced by EFS, so monitor real
  usage in CloudWatch rather than trusting the requested size.
- **RBAC.** These charts create scoped ClusterRoles for ArgoCD instead of the
  `cluster-admin` binding in [../README.md](../README.md). If you already granted
  cluster-admin, consider removing it.

## Uninstall

```bash
oc delete -f https://raw.githubusercontent.com/awslabs/operator-for-ai-chips-on-aws/main/deploy/argocd/applicationset-performance.yaml
oc delete secret -n openshift-gitops <infra-name>-neuron-perf
```

The EFS filesystem, mount targets, security group, IAM role and policy are not
managed by GitOps and are left in place deliberately, so an uninstall cannot
destroy a warm cache. Remove them with `aws` when you actually mean to.
