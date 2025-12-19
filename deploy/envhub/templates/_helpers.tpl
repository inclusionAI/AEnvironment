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
app.kubernetes.io/name: {{ .Values.name }}
{{- end }}

{{- define "envhub.redisAddr" -}}
    {{ if .Values.redisAddr }}
        {{- .Values.redisAddr -}}
    {{ else }}
        {{- printf "redis.%s.svc.cluster.local:6379" .Values.metadata.namespace -}}
    {{ end }}
{{ end }}
