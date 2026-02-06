{{/*
Expand the name of the chart.
*/}}
{{- define "redis-stateful.name" -}}
{{- default .Values.name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "redis-stateful.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "redis-stateful.labels" -}}
helm.sh/chart: {{ include "redis-stateful.chart" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{ include "redis-stateful.selectorLabels" . }}
{{- with .Values.global.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "redis-stateful.selectorLabels" -}}
{{- if .Values.global.selectorLabels }}
{{ tpl (toYaml .Values.global.selectorLabels) . }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "redis-stateful.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "redis-stateful.name" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Redis master hostname for slave configuration
*/}}
{{- define "redis-stateful.masterHost" -}}
{{- if .Values.redis.master.host }}
{{- .Values.redis.master.host }}
{{- else }}
{{- printf "%s-master-0.%s.%s.svc.cluster.local" (include "redis-stateful.name" .) .Values.headlessService.name .Values.global.namespace }}
{{- end }}
{{- end }}

{{/*
Redis master full address for slave configuration
*/}}
{{- define "redis-stateful.masterAddress" -}}
{{- printf "%s:6379" (include "redis-stateful.masterHost" .) }}
{{- end }}
