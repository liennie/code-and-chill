# Correct

Congratulations - your answer is correct!

{{ if not $.Puzzle.Finished -}}
[Continue to part {{ $.Puzzle.Part }}](/{{ $.Event.Path }}/puzzle/{{ $.Puzzle.Path }}#{{ $.Puzzle.Anchor }}).
{{- else -}}
*All parts of this puzzle are solved.*

What happens next:
- Your solve has been recorded on the [leaderboard](/{{ $.Event.Path }}/leaderboard)
- Continue to the [latest puzzle](/{{ $.Event.Path }}/latest)
- Return to the [homepage](/{{ $.Event.Path }})

Thanks for playing - good luck on the next one!
{{- end }}
