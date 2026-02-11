# Leaderboard

{{ if .Leaderboard -}}
|  # | Name |  Parts | Score |
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

Your *position* is determined using the following criteria, in order:
1. How many *puzzle parts* you’ve solved,
2. Your *total score*, and
3. The time of your *most recent* correct solution (for tie-breaking).

Your *score* for each puzzle part is:
- *1 base point*, plus
- *1 bonus point* for *every* participant who submits that part *after* you (or never submits it).

For example, with *10* participants, for each puzzle part:
- The *first* solver earns: 1 + (10 - 1) = *10 points*
- The *second* solver earns: 1 + (10 - 2) = *9 points*

...and so on.

## Additional details

- The leaderboard updates automatically whenever a participant submits a solution.
- Participants appear on the leaderboard as soon as they submit *any* solution, correct or incorrect.
- Scores are calculated using all participants who have submitted at least one solution; participants without any submissions are not included.
- Each puzzle has *two parts*, and every part counts as one solved item.
- The leaderboard is *global* and combines all puzzle parts from the *entire event*.

## Questions?

If something looks wrong or you have questions about scoring, [reach out to us](/{{ $.Event.Path }}/contact).
