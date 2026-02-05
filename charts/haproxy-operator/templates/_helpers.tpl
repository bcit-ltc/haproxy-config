{{/*
Expand the name of the chart.
*/}}
{{- define "haproxy-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "haproxy-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "haproxy-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "haproxy-operator.labels" -}}
helm.sh/chart: {{ include "haproxy-operator.chart" . }}
{{ include "haproxy-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "haproxy-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "haproxy-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "haproxy-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "haproxy-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the image pull policy
*/}}
{{- define "haproxy-operator.imagePullPolicy" -}}
{{- .Values.image.pullPolicy | default "IfNotPresent" }}
{{- end }}

{{/*
Create the container image name
*/}}
{{- define "haproxy-operator.image" -}}
{{- $registry := .Values.image.registry | default "docker.io" }}
{{- $repository := .Values.image.repository | required "image.repository is required" }}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- end }}

{{/*
Create the namespace to watch
*/}}
{{- define "haproxy-operator.watchNamespace" -}}
{{- if .Values.operator.watchNamespace }}
{{- .Values.operator.watchNamespace }}
{{- else }}
{{- "" }}
{{- end }}
{{- end }}

{{/*
Create operator args
*/}}
{{- define "haproxy-operator.args" -}}
- --leader-elect={{ .Values.operator.leaderElection }}
- --metrics-bind-address={{ .Values.metrics.bindAddress }}
- --health-probe-bind-address={{ .Values.healthProbe.bindAddress }}
{{- if .Values.operator.watchNamespace }}
- --namespace={{ .Values.operator.watchNamespace }}
{{- end }}
- --secret-key={{ .Values.operator.secretKey }}
{{- if .Values.operator.logDevelopment }}
- --zap-devel=true
{{- end }}
{{- if .Values.operator.logLevel }}
- --zap-log-level={{ .Values.operator.logLevel }}
{{- end }}
{{- range .Values.extraArgs }}
- {{ . }}
{{- end }}
{{- end }}
