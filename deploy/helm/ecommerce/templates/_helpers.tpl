{{/* Shared names + labels. */}}

{{- define "ecommerce.fullname" -}}
{{- printf "%s-ecommerce" .Release.Name -}}
{{- end -}}

{{/* selectorLabels: stable identity for a service. Call with (dict "name" $name). */}}
{{- define "ecommerce.selectorLabels" -}}
app.kubernetes.io/name: {{ .name }}
app.kubernetes.io/part-of: ecommerce
{{- end -}}

{{/* labels: selector labels + management metadata. Call with (dict "name" $name "root" $). */}}
{{- define "ecommerce.labels" -}}
{{ include "ecommerce.selectorLabels" (dict "name" .name) }}
app.kubernetes.io/managed-by: {{ .root.Release.Service }}
helm.sh/chart: {{ .root.Chart.Name }}-{{ .root.Chart.Version }}
{{- end -}}
