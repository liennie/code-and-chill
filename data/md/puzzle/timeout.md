# Please wait

Your answer {{ $.Puzzle.Submitted | codeesc }} cannot be submitted just yet. You recently submitted an incorrect answer and must wait *{{ $.Puzzle.Timeout.Sub $.Now | formatDuration }}*{data-timeout="{{ $.Puzzle.Timeout | rfc3339Time }}" title="{{ $.Puzzle.Timeout | humanTime }}"} before trying again.

[Return to the puzzle](/{{ $.Event.Path }}/puzzle/{{ $.Puzzle.Path }}?a={{ $.Puzzle.Submitted | queryesc }}#{{ $.Puzzle.Anchor }}-answer).

Do not attempt automated or brute-force methods - see the [rules](/{{ $.Event.Path }}/rules#fair-play) for permitted behavior.

If you believe the cooldown is incorrect or excessive, please [let us know](/{{ $.Event.Path }}/contact) with the time of your last attempt and any error details.
