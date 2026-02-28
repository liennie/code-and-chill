# Profile::(({{ $.User.Name | abbrev 32 | cleanutf | mdesc }})){.secondary-color {{- if gt ($.User.Name | len) 32 }} title="{{ $.User.Name }}" {{- end }}} {#profile}

{{ $return := $.Request.URL.Query.Get "return" | trimPrefix "/" -}}
<form action="/{{ $.Event.Path }}/logout{{ if $return }}?return={{ $return }}{{ end }}" method="POST">
	<button>Log out</button>
</form>
