# Login failed

Something went wrong during authorization.

You can:

{{ $return := $.Request.URL.Query.Get "return" | trimPrefix "/" -}}
- Try to [log in](/{{ $.Event.Path }}/login{{ if $return }}?return={{ $return }}{{ end }}) again
- Return to the [homepage](/{{ $.Event.Path }})
- Take a break and come back later :)

If you believe this is an error, please [let us know](/{{ $.Event.Path }}/contact).
