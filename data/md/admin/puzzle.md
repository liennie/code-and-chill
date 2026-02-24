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

| Input | Part 1 | Part 2 |
| ----- | ------ | ------ |
{{- range $idx, $_ := .Admin.Puzzle.Inputs }}
| [{{ .File | base }}](/{{ $.Event.Path }}/admin/puzzle/{{ $.Admin.Puzzle.Path }}/input/{{ $idx }}) |
{{- range .Answers }} ((********)){data-spoiler="{{ . }}" data-placeholder="********"} | {{- end }}
{{- end }}

# Text

{{- range .Admin.Puzzle.Parts }}
{{ .Text }}
{{- end }}
