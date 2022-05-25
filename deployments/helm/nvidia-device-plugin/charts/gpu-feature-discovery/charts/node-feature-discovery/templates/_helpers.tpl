{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "node-feature-discovery.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "node-feature-discovery.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create a default fully qualified app name for the nfd-master.
*/}}
{{- define "node-feature-discovery.fullname-master" -}}
{{- $fullname := (include "node-feature-discovery.fullname" .) | trunc 56 | trimSuffix "-" -}}
{{- printf "%s-%s" $fullname "master" -}}
{{- end -}}

{{/*
Create a default fully qualified app name for the nfd-worker.
*/}}
{{- define "node-feature-discovery.fullname-worker" -}}
{{- $fullname := (include "node-feature-discovery.fullname" .) | trunc 56 | trimSuffix "-" -}}
{{- printf "%s-%s" $fullname "worker" -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "node-feature-discovery.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "node-feature-discovery.labels" -}}
helm.sh/chart: {{ include "node-feature-discovery.chart" . }}
{{ include "node-feature-discovery.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "node-feature-discovery.selectorLabels" -}}
app.kubernetes.io/name: {{ include "node-feature-discovery.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the rbac role to use
*/}}
{{- define "node-feature-discovery.rbacRole" -}}
{{- if .Values.rbac.create -}}
{{ default (include "node-feature-discovery.fullname-master" .) .Values.serviceAccount.name }}
{{- else -}}
{{ default "default" .Values.rbac.role }}
{{- end -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "node-feature-discovery.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{ default (include "node-feature-discovery.fullname-master" .) .Values.serviceAccount.name }}
{{- else -}}
{{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Full image name with tag
*/}}
{{- define "node-feature-discovery.fullimage" -}}
{{- $tag := printf "v%s" .Chart.AppVersion }}
{{- .Values.image.repository -}}:{{- .Values.image.tag | default $tag -}}
{{- end }}
