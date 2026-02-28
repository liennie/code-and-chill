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
| ----- | ---- | ------ | ------ |
{{- range $idx, $_ := .Admin.Puzzle.Inputs }}
| [{{ .File | base }}](/{{ $.Event.Path }}/admin/puzzle/{{ $.Admin.Puzzle.Path }}/input/{{ $idx }}){#{{ .File | base }}} | |
{{- range .Answers }} ((********)){data-spoiler="{{ . }}" data-placeholder="********"} | {{- end }}
{{- range index $.Admin.PuzzleInputUsers $idx }}
| | [{{ .Name | base }}](/{{ $.Event.Path }}/admin/user/{{ .ID }}) |
{{- end }}
| (( )){.pre-wrap} | | | |
{{- end }}

# Text

{{- range .Admin.Puzzle.Parts }}
{{ .Text }}
{{- end }}
