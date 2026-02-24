# User::(({{ .Admin.User.Name | mdesc }})){.secondary-color} {#user}

## Info

|          |   |
| -        | - |
| *Admin*  | {{ choose .Admin.User.Admin "[x]" "[ ]" }}{disabled=""} |
| *ID*     | {{ .Admin.User.ID }} |
| *Avatar* | ![{{ .Name | mdesc }} avatar]({{ .Admin.User.Avatar }}){.avatar} |
| *Name*   | {{ .Admin.User.Name | mdesc }} |

## Progress

|             |   |
| -           | - |
| *Incorrect* | {{ .Admin.Progress.Incorrect }} |
| *Timeout*   | {{ if .Admin.Progress.Timeout.After .Now }}*{{ .Admin.Progress.Timeout.Sub .Now | formatDuration }}*{data-timeout="{{ .Admin.Progress.Timeout | rfc3339Time }}" title="{{ .Admin.Progress.Timeout | humanTime }}"}{{ end }} |

| Puzzle | Input | Part 1 | Part 2 |
| ------ | ----- | ------ | ------ |
{{- range .Admin.Progress.Puzzles }}
| [{{ .Name }}](/{{ $.Event.Path }}/puzzle/{{ .Path }}) | [{{ .Input }}](/{{ $.Event.Path }}/admin/input/{{ .Path }}/{{ .InputIndex }}) |
{{- $puzzle := . }}
{{- range .Solves }} (({{ .Sub $puzzle.Unlock | formatDuration }})){title="{{ . | humanTime }}"} | {{- end }}
{{- end }}
