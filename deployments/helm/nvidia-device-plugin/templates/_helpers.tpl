{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "nvidia-device-plugin.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "nvidia-device-plugin.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "nvidia-device-plugin.chart" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" $name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "nvidia-device-plugin.labels" -}}
helm.sh/chart: {{ include "nvidia-device-plugin.chart" . }}
{{ include "nvidia-device-plugin.templateLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Template labels
*/}}
{{- define "nvidia-device-plugin.templateLabels" -}}
app.kubernetes.io/name: {{ include "nvidia-device-plugin.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Values.selectorLabelsOverride }}
{{ toYaml .Values.selectorLabelsOverride }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "nvidia-device-plugin.selectorLabels" -}}
{{- if .Values.selectorLabelsOverride -}}
{{ toYaml .Values.selectorLabelsOverride }}
{{- else -}}
{{ include "nvidia-device-plugin.templateLabels" . }}
{{- end }}
{{- end }}

{{/*
Full image name with tag
*/}}
{{- define "nvidia-device-plugin.fullimage" -}}
{{- $tag := printf "v%s" .Chart.AppVersion }}
{{- .Values.image.repository -}}:{{- .Values.image.tag | default $tag -}}
{{- end }}

{{/*
Check if migStrategy (from all possible configurations) is "none"
*/}}
{{- define "nvidia-device-plugin.allPossibleMigStrategiesAreNone" -}}
{{- $result := true -}}
{{- if .Values.migStrategy -}}
  {{- if ne .Values.migStrategy "none" -}}
    {{- $result = false -}}
  {{- end -}}
{{- else -}}
  {{- range $.Values.config.files -}}
    {{- $config := .contents | fromYaml -}}
    {{- if $config.flags -}}
      {{- if ne $config.flags.migStrategy "none" -}}
        {{- $result = false -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- $result -}}
{{- end }}

{{/*
Check if config files have been provided or not
*/}}
{{- define "nvidia-device-plugin.hasConfigFiles" -}}
{{- $result := false -}}
{{- if ne (len .Values.config.files) 0 -}}
  {{- $result = true -}}
{{- end -}}
{{- $result -}}
{{- end }}

{{/*
Get the name of the default configuration
*/}}
{{- define "nvidia-device-plugin.getDefaultConfig" -}}
{{- $result := "" -}}
{{- if .Values.config.default -}}
  {{- $result = .Values.config.default -}}
{{- else if ne (len .Values.config.files) 0 -}}
  {{- with (index .Values.config.files 0) -}}
  {{- $result = .name -}}
  {{- end -}}
{{- end -}}
{{- $result -}}
{{- end }}
