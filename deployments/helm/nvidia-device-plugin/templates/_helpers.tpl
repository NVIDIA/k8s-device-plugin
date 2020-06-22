{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "nvidia-device-plugin.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "nvidia-device-plugin.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "nvidia-device-plugin.labels" -}}
helm.sh/chart: {{ include "nvidia-device-plugin.chart" . }}
{{ include "nvidia-device-plugin.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "nvidia-device-plugin.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nvidia-device-plugin.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
