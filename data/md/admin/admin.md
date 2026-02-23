# Admin

## Users

| Name | Admin |
| ---- | ----- |
{{- range $idx, $_ := .Admin.Users }}
| ![{{ .Name | mdesc }} avatar]({{ .Avatar }}){.avatar} [{{ .Name | mdesc }}](/{{ $.Event.Path }}/admin/user/{{ .ID }}) | {{ choose .Admin "[x]" "[ ]" }}{disabled=""} |
{{- end }}
