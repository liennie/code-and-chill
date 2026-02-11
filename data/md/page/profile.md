# Profile::(({{ $.User.Name | mdesc }})){.secondary-color} {#profile}

{{ $return := $.Request.URL.Query.Get "return" | trimPrefix "/" -}}
<form action="/{{ $.Event.Path }}/logout{{ if $return }}?return={{ $return }}{{ end }}" method="POST">
	<button>Log out</button>
</form>
