{{/*
Expand the name of the chart.
*/}}
{{- define "api-service.name" -}}
{{ .Values.name }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "api-service.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "api-service.labels" -}}
helm.sh/chart: {{ include "api-service.chart" . }}
{{ include "api-service.selectorLabels" . }}
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
{{- define "api-service.selectorLabels" -}}
{{- if .Values.global.selectorLabels }}
{{ tpl (toYaml .Values.global.selectorLabels) . }}
{{- end }}
{{- end }}

{{- define "api-service.backendAddr" -}}
    {{ if .Values.backendAddr }}
        {{- .Values.backendAddr -}}
    {{ else }}
        {{- printf "http://envhub.%s.svc.cluster.local:8083" .Values.metadata.namespace -}}
    {{ end }}
{{ end }}

{{- define "api-service.scheduleAddr" -}}
    {{ if .Values.scheduleAddr }}
        {{- .Values.scheduleAddr -}}
    {{ else }}
        {{- printf "http://controller.%s.svc.cluster.local:8080" .Values.metadata.namespace -}}
    {{ end }}
{{ end }}
