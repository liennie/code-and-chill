# Admin

## Users

| Name | Admin | Hidden |
| ---- | :---: | :----: |
{{- range .Admin.Users }}
| ![{{ .Name | mdesc }} avatar]({{ .AvatarURL }}){.avatar} [{{ .Name | mdesc }}](/{{ $.Event.Path }}/admin/user/{{ .ID }}) | {{ choose .Admin "[x]" "[ ]" }}{disabled=""} | {{ choose .Hidden "[x]" "[ ]" }}{disabled="" class="user-checkbox" data-user="{{ .ID }}" data-value="hidden"} |
{{- end }}

## Puzzles

| Name | Hidden |
| ---- | :----: |
{{- range .Puzzles }}
| [{{ .Name }}](/{{ $.Event.Path }}/admin/puzzle/{{ .Path }}) | [ ]{disabled=""} |
{{- end }}
