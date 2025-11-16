package db

var (
	bucketUser        = []byte("user")
	bucketDiscordUser = []byte("discord_user")
)

type User struct {
	Name      string
	AvatarURL string
}

func (tx *Tx) User() *Bucket[User] {
	return openBucket[User](tx, bucketUser)
}

type DiscordUser struct {
	ID string
}

func (tx *Tx) DiscordUser() *Bucket[DiscordUser] {
	return openBucket[DiscordUser](tx, bucketDiscordUser)
}
