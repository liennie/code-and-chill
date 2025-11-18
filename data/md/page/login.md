# Log In

{{ $return := $.Request.URL.Query.Get "return" | trimPrefix "/" -}}
Please [log in using your Discord account](/{{ $.Event.Path }}/login/discord{{ if $return }}?return={{ $return }}{{ end }}) to participate in *Code && Chill::*[{{ $.Event.Name }}](/{{ $.Event.Path }}).

Logging in allows us to:
- Track your progress and submissions
- Display your name on the leaderboard
