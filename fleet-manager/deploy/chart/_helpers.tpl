{{/*
Name of each object of the fleet manager.
*/}}
{{- define "fleet-manager.name" -}}
{{- include "camunda-operator.resourceName" (dict "suffix" "fleet-manager" "context" .) }}
{{- end }}

{{/*
Labels that select the fleet manager pod.
*/}}
{{- define "fleet-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "camunda-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: fleet-manager
{{- end }}

{{/*
Labels of each object of the fleet manager.
*/}}
{{- define "fleet-manager.labels" -}}
{{ include "fleet-manager.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{- end }}
