# Profile::**{{ $.User.Name }}** {#profile}

{{ $return := $.Request.URL.Query.Get "return" | trimPrefix "/" -}}
- [Log out](/{{ $.Event.Path }}/logout{{ if $return }}?return={{ $return }}{{ end }})
