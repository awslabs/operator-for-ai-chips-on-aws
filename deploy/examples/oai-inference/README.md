# OpenShift AI Inference with Neuron — Example Manifests

Example manifests for deploying LLM inference on AWS Neuron using OpenShift AI (KServe).

## Prerequisites

1. AWS Neuron Operator installed (`deploy/helm/aws-neuron-operator`)
2. OpenShift AI with KServe installed (`deploy/examples/openshiftai`)
3. Namespace and credentials:
   ```bash
   oc create namespace neuron-inference
   oc label namespace neuron-inference istio-injection=enabled
   oc create secret generic hf-token --from-literal=HF_TOKEN=<your-token> -n neuron-inference
   oc create sa kserve-neuron-sa -n neuron-inference
   oc patch sa kserve-neuron-sa -n neuron-inference -p '{"secrets":[{"name":"hf-token"}]}'
   ```

## Deploy

```bash
# Optional: create PVC for Neuron compilation cache (faster restarts)
oc apply -f pvc.yaml

# Deploy the InferenceService
oc apply -f inferenceservice.yaml
```

## Monitor

```bash
oc get inferenceservice llama31-8b-neuron -n neuron-inference -w
```

First startup takes ~30 minutes (model download + Neuron compilation). Subsequent restarts with the PVC cache skip compilation (~5 minutes).

## Test

```bash
ISVC_URL=$(oc get inferenceservice llama31-8b-neuron -n neuron-inference -o jsonpath='{.status.url}')
curl -k ${ISVC_URL}/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","messages":[{"role":"user","content":"What is OpenShift?"}],"max_tokens":50}'
```

## Customization

- **Model**: Change `storageUri`, `MODEL_NAME`, and resource requests
- **Tensor parallelism**: Update `--tensor-parallel-size` in the ServingRuntime values
- **PVC**: Remove the PVC and volume references from `inferenceservice.yaml` if caching is not needed
