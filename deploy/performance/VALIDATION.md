# Validating the shared Neuron compile cache

How to prove this deployment does what it claims. Install first, per
[README.md](README.md). Three levels, in increasing cost and confidence.

| Level | Proves | Needs |
|---|---|---|
| [1. Storage layer](#level-1-the-shared-cache-works) | a cache written on one node is usable from another | two nodes |
| [2. Storage cost](#level-2-what-the-shared-cache-costs) | cache retrieval is cheap next to a compile | two nodes |
| [3. Serving](#level-3-startup-time-with-and-without-a-warm-cache) | actual vLLM startup saved | Neuron node, Hugging Face token |

Levels 1 and 2 need no Hugging Face token and no Neuron device. Level 3 is the end to
end proof and the only one that produces a publishable number. Nothing at level 3 has
ever been run here.

Resource names below are the ones the GitOps flow produces, where the Helm release
name is pinned to `neuron-serving`. Installing by hand substitutes your release name.

## Confirm the layers converged

```bash
oc get applications -n openshift-gitops

# EFS CSI driver installed, not merely subscribed
oc get clustercsidriver efs.csi.aws.com -o json \
  | jq -r '.status.conditions[] | select(.status=="True") | .type' | grep Available

# STS wiring: the role ARN on the Subscription became a usable credential
oc get secret aws-efs-cloud-credentials -n openshift-cluster-csi-drivers \
  -o jsonpath='{.data.credentials}' | base64 -d

oc get sc efs-sc
oc get nodes -o custom-columns=NAME:.metadata.name,NEURON:.status.allocatable.aws\\.amazon\\.com/neuron
```

Expect `Synced` and `Healthy` for `<cluster>-efs-storage` and
`<cluster>-aws-neuron-operator`, plus `<cluster>-oai-platform` and
`<cluster>-neuron-serving` when `enable_oai=true`. They converge in any order, so a PVC
created before the driver is ready stays `Pending` and binds later. Expect both
`AWSEFSDriverControllerServiceControllerAvailable` and
`AWSEFSDriverNodeServiceControllerAvailable`, and a `role_arn` plus
`web_identity_token_file` in the credentials. A missing credentials secret means the role
ARN never reached the operator and every PVC will stay `Pending`.

## Level 1: the shared cache works

This is the property everything else rests on. `ReadWriteOnce` would confine the cache
to one node, so scale-out and rescheduling would recompile.

```bash
oc apply -f tests/rwx-share-test.yaml
oc wait --for=condition=Ready pod/cache-writer -n neuron-cache-test --timeout=180s
oc logs -f pod/cache-reader -n neuron-cache-test
```

Expect `SHARED-CACHE-OK  read and wrote an 8MB artifact produced on <other-node>`. The
reader uses pod anti-affinity to land on a different node from the writer and fails with
`SHARED-CACHE-INCONCLUSIVE` if it does not. It also appends, because a cache has to be
writable from every node and not only readable. Landing in different availability zones
is opportunistic rather than asserted; if you get it, both mount targets have been
exercised. Keep the namespace for level 2.

## Level 2: what the shared cache costs

EFS is NFS and a compile cache is many small files, so it is fair to ask whether reading
one back beats recompiling.

```bash
oc apply -f tests/storage-latency-probe.yaml
oc wait --for=jsonpath='{.status.phase}'=Succeeded pod/storage-latency-probe \
  -n neuron-cache-test --timeout=300s
oc logs pod/storage-latency-probe -n neuron-cache-test
```

Measured on ROSA 4.22.11 (`inf2.8xlarge`, elastic throughput, generalPurpose):

```
LOCAL(emptyDir)    write300x32k=  515ms  readAll300=    8ms  write256MB= 1185ms  read256MB=   27ms
EFS(RWX)           write300x32k= 5598ms  readAll300=  716ms  write256MB=  846ms  read256MB=  608ms
```

Read the absolute numbers. Per-file overhead on EFS is real, and the local reads are
served from page cache so they flatter local disk, which makes these ratios an upper
bound on EFS's disadvantage. What matters is that retrieving a cache took well under a
second here, against a compilation measured in minutes. That is why the chart mounts EFS
directly instead of copying into a local `emptyDir` at startup. This is a storage
measurement and says nothing about end to end startup.

## Level 3: startup time with and without a warm cache

The only level that answers "how much faster does vLLM start". Nothing here has been
measured in this repo, so treat the procedure as the instrument and record your own
numbers.

Cache the model weights first, otherwise you measure download and compile together and
cannot attribute the saving. Create the token Secret as described in
[README.md](README.md), then set `modelCache.enabled=true`,
`modelCache.download.enabled=true`, `model.source=pvc` and
`model.tokenSecretName=hf-token` in `../helm/neuron-serving/values.yaml`, or in a
per-cluster `values-<cluster>.yaml` beside it, and commit.

Then take three timings. Each is: put the cache in a known state, wait for `Ready`, and
subtract the pod's `.status.startTime` from its `Ready` condition's
`lastTransitionTime`.

- **A, cold cache.** `oc delete pvc neuron-compile-cache -n neuron-inference`. Recreating
  the claim is the cleanest way to empty it, because deleting it releases the EFS access
  point. The claim carries `Prune=false`, so ArgoCD will not delete it for you and
  recreates it after you do.
- **B, warm cache, same node.** Delete the pod and keep the claim.
- **C, warm cache, a node that never compiled.** Cordon the node that ran B, delete the
  pod, let it reschedule, then uncordon. C is the number to publish, because it is the
  case a `ReadWriteOnce` cache cannot help with.

```bash
ISVC=neuron-serving; NS=neuron-inference
oc wait --for=condition=complete job/$ISVC-model-download -n $NS --timeout=3600s
oc wait --for=condition=Ready inferenceservice/$ISVC -n $NS --timeout=3600s
oc get pods -n $NS -l serving.kserve.io/inferenceservice=$ISVC \
  -o jsonpath='{.items[0].status.startTime}{"  "}{.items[0].status.conditions[?(@.type=="Ready")].lastTransitionTime}{"\n"}'
oc delete pod -n $NS -l serving.kserve.io/inferenceservice=$ISVC
# what actually happened: compiler output, or "Using a cached neff at ..." on a hit
oc logs -n $NS -l serving.kserve.io/inferenceservice=$ISVC --tail=-1 \
  | grep -i "compil\|cached neff\|Compile cache path"
```

The cache removes compilation. Image pull, Neuron runtime and device init, weight load
and vLLM engine startup remain in every run, so B and C will not approach zero. If B is
not meaningfully below A, check that `NEURON_COMPILE_CACHE_URL` is set in the runtime and
that `NEURON_CC_FLAGS --cache_dir` is not overriding it, since `--cache_dir` takes
precedence. A measured warm number is also what justifies lowering the startup probe
budget, which the chart sets to 1230 seconds for a cold compile.

`deploy/examples/oai-inference/README.md` states roughly 30 minutes cold and 5 minutes
warm. That figure is unsourced in this repo and predates these charts. This procedure
exists to replace it with something measured.

## Teardown

```bash
oc delete -f tests/rwx-share-test.yaml   # includes the test namespace
```

Uninstalling the deployment itself is in [README.md](README.md), which also explains
what a toggle deletes.
