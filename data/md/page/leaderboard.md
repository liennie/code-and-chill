# Leaderboard

{{ if .Leaderboard -}}
|  # | Name |  ✔ | Score |
| -: | ---- | -: | ----: |

{{- range $idx, $_ := .Leaderboard }}
{{- if and $.User (eq .User.ID $.User.ID)}}
| (({{ $idx | add 1 }})){.user} | ![{{ .User.Name | mdesc }} avatar]({{ .User.Avatar }}){.avatar} (({{ .User.Name | mdesc }})){.user} | (({{ .Solved }})){.user} | (({{ .Score }})){.user} |
{{- else }}
| {{ $idx | add 1 }} | ![{{ .User.Name | mdesc }} avatar]({{ .User.Avatar }}){.avatar} {{ .User.Name | mdesc }} | {{ .Solved }} | {{ .Score }} |
{{- end }}
{{- end }}

{{- else -}}

No entries yet - be the first to solve a puzzle and appear on the leaderboard!

{{- if .User }}
{{- if .PuzzleUnlocked }}

Try the [latest puzzle](/{{ $.Event.Path }}/latest).
{{- end }}
{{- else }}
{{- if .PuzzleUnlocked }}

[Log in](/{{ $.Event.Path }}/login?return=leaderboard) and try the [latest puzzle](/{{ $.Event.Path }}/latest).
{{- else }}

[Log in](/{{ $.Event.Path }}/login?return=leaderboard) to start solving.
{{- end }}
{{- end }}

{{- end }}

## How it works

- *Ranking:* Players are ranked first by the *number of puzzles solved*, then by *score*, then by the *time* of their last solution.
- *Scoring:* For each correct solution you get 1 point plus 1 additional point for every other participant who solved it later or not at all.
- *Example:* With 10 participants, the 2nd solver gets 1 + (10 − 2) = 9 points.

## Questions?

If something looks wrong or you have questions about scoring, [reach out to us](/{{ $.Event.Path }}/contact).
