package session

type Data struct {
	Auth *Auth
}

type Auth struct {
	State string
	Event string
}
