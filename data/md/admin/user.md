# User::(({{ .Admin.User.Name | mdesc }})){.secondary-color} {#user}

## Info

(()){#user-error class="error"}

|                 |   |
| -               | - |
| *Admin*         | {{ choose .Admin.User.Admin "[x]" "[ ]" }}{disabled=""} |
| *Hidden*        | {{ choose .Admin.User.Hidden "[x]" "[ ]" }}{disabled="" class="user-checkbox" data-user="{{ .Admin.User.ID }}" data-value="hidden"} |
| *ID*            | {{ .Admin.User.ID }} |
| *Name*          | {{ .Admin.User.Name | mdesc }} |
| *Avatar*        | ![{{ .Name | mdesc }} avatar]({{ .Admin.User.AvatarURL }}){.avatar} |
| *Random avatar* | {{ choose .Admin.User.RandomAvatar "[x]" "[ ]" }}{disabled=""} |

## Progress

|             |   |
| -           | - |
| *Incorrect* | {{ .Admin.Progress.Incorrect }} |
| *Timeout*   | {{ if .Admin.Progress.Timeout.After .Now }}*{{ .Admin.Progress.Timeout.Sub .Now | formatDuration }}*{data-timeout="{{ .Admin.Progress.Timeout | rfc3339Time }}" title="{{ .Admin.Progress.Timeout | humanTime }}"}{{ end }} |

| Puzzle | Input | Part 1 | Part 2 |
| ------ | ----- | -----: | -----: |
{{- range .Admin.Progress.Puzzles }}
| [{{ .Name }}](/{{ $.Event.Path }}/admin/puzzle/{{ .Path }}) | [{{ .Input | base }}](/{{ $.Event.Path }}/admin/puzzle/{{ .Path }}/input/{{ .InputIndex }}) |
{{- $puzzle := . }}
{{- range .Solves }} (({{ .Sub $puzzle.Unlock | formatDuration }})){title="{{ . | humanTime }}"} | {{- end }}
{{- end }}
