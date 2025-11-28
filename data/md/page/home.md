# {{ $.Name }}::**{{ $.Event.Name }}** {#title}

Get ready for a series of coding puzzles released one at a time! Each puzzle unlocks on schedule - solve them to climb the leaderboard and uncover the full story as it unfolds.

To get started:
- Read the [rules](/{{ $.Event.Path }}/rules)
- Join the discussion // TODO link to a discord channel
- [Log in](/{{ $.Event.Path }}/login) to start solving
- Visit the [latest puzzle](/{{ $.Event.Path }}/latest)
- Check out the [leaderboard](/{{ $.Event.Path }}/leaderboard)

## Puzzles

Below are all the puzzles for this event. Locked puzzles will appear greyed out until they’re ready to play. You can also find all the puzzles for this event in the *side menu*.

{{- range .Puzzles }}
{{- if .Locked }}
- ~~{{ .Name }}~~
{{- else }}
- [{{ .Name }}](/{{ $.Event.Path }}/puzzle/{{ .Path }}){{ if $.User }}{{ repeat (.Name | len | sub $.PuzzleAlign | int) " " }}{{ puzzleHint . }}{{ end }}
{{- end }}
{{- end }}

Good luck, and happy puzzling!
