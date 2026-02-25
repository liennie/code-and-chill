package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"cc/internal/ctxlog"
	"cc/internal/db"
)

type DiscordAuth struct {
	clientID        string
	clientSecret    string
	redirectURI     string
	exchangeTimeout time.Duration
	guildID         string

	db                *db.DB
	bucketUser        *db.BucketKey[User]
	bucketDiscordUser *db.BucketKey[dbDiscordUser]
}

func newDiscordAuth(config DiscordConfig, ddb *db.DB) DiscordAuth {
	if config.ClientID == "" {
		panic("discord auth: clientID is required")
	}
	if config.ClientSecret == "" {
		panic("discord auth: clientSecret is required")
	}
	if config.RedirectURI == "" {
		panic("discord auth: redirectURI is required")
	}

	secret, err := os.ReadFile(config.ClientSecret)
	if err != nil {
		panic(fmt.Errorf("discord auth: read discord client secret: %w", err))
	}

	return DiscordAuth{
		clientID:        config.ClientID,
		clientSecret:    strings.TrimSpace(string(secret)),
		redirectURI:     config.RedirectURI,
		exchangeTimeout: config.ExchangeTimeout,
		guildID:         config.GuildID,

		db:                ddb,
		bucketUser:        db.NewBucketKey[User](ddb, db.BucketUser),
		bucketDiscordUser: db.NewBucketKey[dbDiscordUser](ddb, db.BucketDiscordUser),
	}
}

const discordAuthURL = "https://discord.com/oauth2/authorize"
const discordTokenURL = "https://discord.com/api/oauth2/token"
const discordTokenRevokeURL = "https://discord.com/api/oauth2/token/revoke"
const discordAPIBaseURL = "https://discord.com/api/v10"
const discordUserEndpoint = discordAPIBaseURL + "/users/@me"
const discordGuildMemberEndpoint = discordAPIBaseURL + "/users/@me/guilds/%s/member"
const discordAvatarURL = "https://cdn.discordapp.com/avatars/%s/%s.png?size=32"

func (a *DiscordAuth) AuthURL(state string) string {
	scope := "identify"
	if a.guildID != "" {
		scope = "identify guilds.members.read"
	}

	v := url.Values{
		"client_id":     {a.clientID},
		"response_type": {"code"},
		"redirect_uri":  {a.redirectURI},
		"scope":         {scope},
		"state":         {state},
	}
	return discordAuthURL + "?" + v.Encode()
}

func (a *DiscordAuth) Exchange(ctx context.Context, code string, token string) (*User, error) {
	cli := &http.Client{
		Timeout: a.exchangeTimeout,
	}
	// TODO https://docs.discord.com/developers/reference#user-agent

	t, err := a.exchange(ctx, cli, code)
	if err != nil {
		return nil, fmt.Errorf("discord token exchange: %w", err)
	}

	defer func() {
		revokeErr := a.revoke(ctx, cli, t)
		if revokeErr != nil {
			logger := ctxlog.Get(ctx)
			if extra, ok := ctxlog.ErrExtra(err); ok {
				logger = logger.With("extra", extra)
			}
			logger.Error("discord token revoke", "error", revokeErr)
		}
	}()

	user, err := a.getUser(ctx, cli, t)
	if err != nil {
		return nil, fmt.Errorf("discord get user: %w", err)
	}

	return a.updateDB(user, token)
}

type discordError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Errors  json.RawMessage `json:"errors,omitempty"`
}

func (e *discordError) Error() string {
	return fmt.Sprintf("discord error %d: %s", e.Code, e.Message)
}

func (e *discordError) Extra() string {
	return string(e.Errors)
}

func (a *DiscordAuth) exchange(ctx context.Context, cli *http.Client, code string) (token, error) {
	resp, err := cli.PostForm(discordTokenURL, url.Values{
		"client_id":     {a.clientID},
		"client_secret": {a.clientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {a.redirectURI},
	})
	if err != nil {
		return token{}, fmt.Errorf("client: %w", err)
	}
	defer ctxlog.CloseErr(ctx, "discord exchange body", resp.Body)

	if resp.StatusCode != http.StatusOK {
		var dErr discordError
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&dErr)
		if err == nil && dErr.Code != 0 {
			return token{}, &dErr
		}
		return token{}, fmt.Errorf("status %d: %s", resp.StatusCode, resp.Status)
	}

	var t token
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&t)
	if err != nil {
		return token{}, fmt.Errorf("decode response: %w", err)
	}

	return t, nil
}

func (a *DiscordAuth) revoke(ctx context.Context, cli *http.Client, t token) error {
	resp, err := cli.PostForm(discordTokenRevokeURL, url.Values{
		"client_id":       {a.clientID},
		"client_secret":   {a.clientSecret},
		"token":           {t.AccessToken},
		"token_type_hint": {"access_token"},
	})
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}
	defer ctxlog.CloseErr(ctx, "discord revoke body", resp.Body)

	if resp.StatusCode != http.StatusOK {
		var dErr discordError
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&dErr)
		if err == nil && dErr.Code != 0 {
			return &dErr
		}

		return fmt.Errorf("status %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

type guildMember struct {
	User   *discordUser `json:"user"`
	Nick   *string      `json:"nick"`
	Avatar *string      `json:"avatar"`
}

func (u *guildMember) toDiscordUser() *discordUser {
	if u.User == nil {
		return nil
	}

	user := u.User
	if u.Nick != nil {
		user.GlobalName = *u.Nick
	}
	if u.Avatar != nil {
		user.Avatar = u.Avatar
	}
	user.GuildMember = true

	return user
}

type discordUser struct {
	ID            string  `json:"id"`
	Username      string  `json:"username"`
	Discriminator string  `json:"discriminator"`
	GlobalName    string  `json:"global_name"`
	Avatar        *string `json:"avatar"`
	GuildMember   bool    `json:"-"`
}

func (a *DiscordAuth) getGuildMember(ctx context.Context, cli *http.Client, t token) (*discordUser, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf(discordGuildMemberEndpoint, a.guildID), nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+t.AccessToken)

	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}
	defer ctxlog.CloseErr(ctx, "discord user body", resp.Body)

	if resp.StatusCode != http.StatusOK {
		var dErr discordError
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&dErr)
		if err == nil && dErr.Code != 0 {
			return nil, &dErr
		}

		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, resp.Status)
	}

	var member guildMember
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&member)
	if err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return member.toDiscordUser(), nil
}

func (a *DiscordAuth) getUser(ctx context.Context, cli *http.Client, t token) (*discordUser, error) {
	if a.guildID != "" {
		user, err := a.getGuildMember(ctx, cli, t)
		if err != nil {
			if derr, ok := errors.AsType[*discordError](err); !ok || derr.Code != 10004 {
				return nil, fmt.Errorf("get guild member: %w", err)
			}
		}
		if user != nil {
			return user, nil
		}
	}

	req, err := http.NewRequest("GET", discordUserEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+t.AccessToken)

	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}
	defer ctxlog.CloseErr(ctx, "discord user body", resp.Body)

	if resp.StatusCode != http.StatusOK {
		var dErr discordError
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&dErr)
		if err == nil && dErr.Code != 0 {
			return nil, &dErr
		}

		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, resp.Status)
	}

	var user discordUser
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &user, nil
}

type dbDiscordUser struct {
	ID string `json:"id"`
}

func (a *DiscordAuth) updateDB(discordUser *discordUser, token string) (*User, error) {
	username := discordUser.GlobalName
	if username == "" {
		username = fmt.Sprintf("%s#%s", discordUser.Username, discordUser.Discriminator)
	}

	var user *User
	err := a.db.Update(func(tx *db.Tx) error {
		discordBucket := a.bucketDiscordUser.Open(tx)
		userBucket := a.bucketUser.Open(tx)

		discordUserID := discordBucket.Get(discordUser.ID)
		if discordUserID != nil {
			user = userBucket.Get(discordUserID.ID)
		}

		if user == nil {
			user = &User{
				Hidden: !discordUser.GuildMember,
			}
			user.ID = newID()
			for userBucket.Has(user.ID) {
				user.ID = newID()
			}

			err := discordBucket.Put(discordUser.ID, &dbDiscordUser{ID: user.ID})
			if err != nil {
				return err
			}
		}

		user.Name = username
		if discordUser.Avatar != nil {
			user.AvatarURL = fmt.Sprintf(discordAvatarURL, discordUser.ID, *discordUser.Avatar)
			user.RandomAvatar = false
		} else if !user.RandomAvatar {
			user.AvatarURL = randomAvatar()
			user.RandomAvatar = true
		}
		user.Token = token

		return userBucket.Put(user.ID, user)
	})
	if err != nil {
		return nil, fmt.Errorf("discord user db update: %w", err)
	}

	return user, nil
}
