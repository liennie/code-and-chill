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

*No entries yet* - be the first to solve a puzzle and appear on the leaderboard!

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

### Rank
Players are ranked using the following criteria, in order:

1. Number of puzzle parts solved
2. Total score
3. Time of the most recent correct solution (earlier is better)

### Score
For every puzzle part you solve, you earn:

- *1 base point*, plus
- *1 bonus point* for every participant who submits that part *after* you (or never submits it).

The faster you solve a part relative to others, the more points you earn.

### Example
With 10 participants, for each part:

- The *first* solver earns:
	1 + (10 - 1) = *10 points*
- The *second* solver earns:
	1 + (10 - 2) = *9 points*

...and so on.

### Scope
The leaderboard is *global*, combining all puzzle parts from the entire event.

## Questions?

If something looks wrong or you have questions about scoring, [reach out to us](/{{ $.Event.Path }}/contact).
