{{/*
Expand the name of the chart.
*/}}
{{- define "envhub.name" -}}
{{ .Values.name }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "envhub.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "envhub.labels" -}}
helm.sh/chart: {{ include "envhub.chart" . }}
{{ include "envhub.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- with .Values.global.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "envhub.selectorLabels" -}}
{{- if .Values.global.selectorLabels }}
{{ tpl (toYaml .Values.global.selectorLabels) . }}
{{- end }}
{{- end }}

{{- define "envhub.redisAddr" -}}
    {{ if .Values.global.redisClusterEnabled }}
        {{- printf "redis-cluster.%s.svc.%s:6379" .Values.global.namespace .Values.global.domain -}}
    {{ else }}
        {{- printf "redis.%s.svc.%s:6379" .Values.global.namespace .Values.global.domain -}}
    {{ end }}
{{ end }}
