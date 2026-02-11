# Already solved

You attempted to submit an answer to an already solved part of this puzzle.

{{ if not $.Puzzle.Finished -}}
[Continue to part {{ $.Puzzle.Part }}](/{{ $.Event.Path }}/puzzle/{{ $.Puzzle.Path }}#{{ $.Puzzle.Anchor }}).
{{- else -}}
*All parts of this puzzle are solved.*

What happens next:
- Continue to the [latest puzzle](/{{ $.Event.Path }}/latest)
- Return to the [homepage](/{{ $.Event.Path }})
{{- end }}

If you believe you received this message in error, please [let us know](/{{ $.Event.Path }}/contact) and include which puzzle part you were on and the exact text you submitted.
