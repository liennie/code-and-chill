# Events

Here’s a list of all events you can explore.
Click on any event to see its puzzles, rules, and leaderboard.

{{- range .Events }}
- [{{ .Name }}](/{{ .Path }}){{ if $.User }}{{ repeat (.Name | len | sub $.EventAlign | int) " " }}*{{ .Solved }}* / {{ .Total }}{{ end }}
{{- end }}
