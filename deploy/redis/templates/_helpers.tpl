{{- define "redis.name" -}}
{{- default .Values.name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{- define "redis.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- include "redis.name" . }}
{{- end }}
{{- end }}

{{- define "redis.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{ include "redis.selectorLabels" . }}
{{- with .Values.global.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{- define "redis.selectorLabels" -}}
{{- if .Values.global.selectorLabels }}
{{ tpl (toYaml .Values.global.selectorLabels) . }}
{{- end }}
{{- end }}
