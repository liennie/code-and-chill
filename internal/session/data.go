package session

type Data struct {
	Auth *Auth
	User *User
}

type Auth struct {
	State  string
	Event  string
	Return string
}

type User struct {
	ID string
}
