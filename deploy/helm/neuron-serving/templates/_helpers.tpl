{{/*
Name helpers. Every namespaced resource in this chart derives its name from
fullname so that a second release can serve a second model in the same
namespace without colliding.

The one exception is the compile-cache claim: see compileCacheClaim below.
*/}}

{{- define "neuron-serving.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "neuron-serving.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "neuron-serving.labels" -}}
app.kubernetes.io/name: {{ include "neuron-serving.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "neuron-serving.serviceAccountName" -}}
{{- default (printf "%s-sa" (include "neuron-serving.fullname" .)) .Values.inferenceService.serviceAccountName -}}
{{- end -}}

{{- define "neuron-serving.servingRuntimeName" -}}
{{- default (printf "%s-runtime" (include "neuron-serving.fullname" .)) .Values.servingRuntime.name -}}
{{- end -}}

{{- define "neuron-serving.inferenceServiceName" -}}
{{- default (include "neuron-serving.fullname" .) .Values.inferenceService.name -}}
{{- end -}}

{{/*
Compile-cache claim name. NOT release-derived: one EFS-backed claim holds the
cache entries for every model on the cluster, keyed by graph hash, and sharing it
is the entire point of the feature. So the default name is fixed and a second
release sets compileCache.create=false with existingClaim pointing here.

existingClaim wins when set, and setting it alongside create=true is rejected as
a contradiction rather than silently provisioning a claim nothing mounts.
*/}}
{{- define "neuron-serving.compileCacheClaim" -}}
{{- if .Values.compileCache.existingClaim -}}
{{- if .Values.compileCache.create -}}
{{- fail "compileCache.existingClaim and compileCache.create=true are contradictory: set create=false to mount an existing claim, or clear existingClaim to provision a new one." -}}
{{- end -}}
{{- .Values.compileCache.existingClaim -}}
{{- else -}}
{{- .Values.compileCache.name -}}
{{- end -}}
{{- end -}}

{{/*
Model-cache claim name. Per-model, so release-derived by default.
*/}}
{{- define "neuron-serving.modelCacheClaim" -}}
{{- if .Values.modelCache.existingClaim -}}
{{- if .Values.modelCache.create -}}
{{- fail "modelCache.existingClaim and modelCache.create=true are contradictory: set create=false to mount an existing claim, or clear existingClaim to provision a new one." -}}
{{- end -}}
{{- .Values.modelCache.existingClaim -}}
{{- else -}}
{{- default (printf "%s-model-cache" (include "neuron-serving.fullname" .)) .Values.modelCache.name -}}
{{- end -}}
{{- end -}}
