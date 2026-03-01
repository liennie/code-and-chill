# Puzzle::(({{ .Admin.Puzzle.Name | mdesc }})){.secondary-color} {#puzzle}

## Info

|                 |   |
| -               | - |
| *Link*          | [{{ .Admin.Puzzle.Name }}](/{{ $.Event.Path }}/puzzle/{{ .Admin.Puzzle.Path }}) |
| *ID*            | {{ .Admin.Puzzle.ID }} |
| *Path*          | {{ .Admin.Puzzle.Path }} |
| *Name*          | {{ .Admin.Puzzle.Name }} |
| *Unlock*        | {{ .Admin.Puzzle.Unlock | humanTime }} |

## Inputs

| Input | User | Part 1 | Part 2 |
| ----- | ---- | -----: | -----: |
{{- range $idx, $_ := .Admin.Puzzle.Inputs }}
| [{{ .File | base }}](/{{ $.Event.Path }}/admin/puzzle/{{ $.Admin.Puzzle.Path }}/input/{{ $idx }}){#{{ .File | base }}} | |
{{- range .Answers }} ((********)){data-spoiler="{{ . }}" data-pad="start"} | {{- end }}
{{- range index $.Admin.PuzzleInputUsers $idx }}
| | ![{{ .User.Name | mdesc }} avatar]({{ .User.AvatarURL }}){.avatar} [{{ .User.Name | abbrev 32 | cleanutf | mdesc }}](/{{ $.Event.Path }}/admin/user/{{ .User.ID }}){{ if gt (.User.Name | len) 32 }}{title="{{ .User.Name }}"}{{- end }} |
{{- range .Progress.Solves }} (({{ .Sub $.Admin.Puzzle.Unlock | formatDuration }})){title="{{ . | humanTime }}"} | {{- end }}
{{- end }}
| (( )){.pre-wrap} | | | |
{{- end }}

# Text

{{- range .Admin.Puzzle.Parts }}
{{ .Text }}
{{- end }}
