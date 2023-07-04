{{- define "gvList" -}}
{{- $groupVersions := . -}}

# API Reference

{{- range $groupVersions }}
{{- end }}

{{ range $groupVersions }}
{{ template "gvDetails" . }}
{{ end }}

{{- end -}}
