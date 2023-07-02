{{- define "gvDetails" -}}
{{- $gv := . -}}

<!-- GENERATED FROM https://github.com/substratusai/substratus USING make docs WHICH WROTE TO PATH /docs/api/ -->

**API Version: {{ $gv.GroupVersionString }}**

{{ $gv.Doc }}

{{- if $gv.Kinds  }}
## Resources
{{- range $gv.SortedKinds }}
- {{ $gv.TypeForKind . | markdownRenderTypeLink }}
{{- end }}
{{ end }}

## Types
{{ range $gv.SortedTypes }}
{{ template "type" . }}
{{ end }}

{{- end -}}
