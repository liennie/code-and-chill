# Admin

## Users

(()){#user-error .error}

| Name | Admin | Hidden |
| ---- | :---: | :----: |
{{- range .Admin.Users }}
| ![{{ .Name | mdesc }} avatar]({{ .AvatarURL }}){.avatar} [{{ .Name | abbrev 32 | cleanutf | mdesc }}](/{{ $.Event.Path }}/admin/user/{{ .ID }}){{ if gt (.Name | len) 32 }}{title="{{ .Name }}"}{{- end }} | {{ choose .Admin "[x]" "[ ]" }}{disabled=""} | {{ choose .Hidden "[x]" "[ ]" }}{.user-checkbox disabled="" data-user="{{ .ID }}" data-value="hidden"} |
{{- end }}

## Puzzles

| Name |
| ---- |
{{- range .Admin.Puzzles }}
| [{{ .Name }}](/{{ $.Event.Path }}/admin/puzzle/{{ .Path }}) |
{{- end }}
