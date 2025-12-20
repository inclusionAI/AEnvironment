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
{{- end }}

{{/*
Selector labels
*/}}
{{- define "envhub.selectorLabels" -}}
app.kubernetes.io/name: {{ .Values.name }}
{{- end }}

{{/*
Generate Redis Sentinel addresses based on replica count
If sentinelAddrs is empty or not set, automatically generate addresses based on sentinelReplicaCount
*/}}
{{- define "envhub.redisSentinelAddrs" -}}
{{- $sentinelReplicaCount := .Values.redis.sentinelReplicaCount | default 3 | int }}
{{- $namespace := .Values.metadata.namespace | default "aenv" }}
{{- $sentinelAddrs := .Values.redis.sentinelAddrs | default "" | trim }}
{{- if and $sentinelAddrs (ne $sentinelAddrs "") }}
{{- /* Use explicitly provided sentinelAddrs if it's not empty */}}
{{- $sentinelAddrs }}
{{- else if gt $sentinelReplicaCount 0 }}
{{- /* Auto-generate addresses based on sentinelReplicaCount when sentinelAddrs is empty or not set */}}
{{- $addrs := list }}
{{- range $i := until $sentinelReplicaCount }}
{{- $addrs = append $addrs (printf "redis-sentinel-%d.redis-sentinel-headless.%s.svc.cluster.local:26379" $i $namespace) }}
{{- end }}
{{- join "," $addrs }}
{{- else }}
{{- /* Fallback to Service address */}}
{{- printf "redis-sentinel.%s.svc.cluster.local:26379" $namespace }}
{{- end }}
{{- end }}
