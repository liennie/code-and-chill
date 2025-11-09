package auth

import (
	"cc/internal/ctxlog"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type DiscordAuth struct {
	clientID        string
	clientSecret    string
	redirectURI     string
	exchangeTimeout time.Duration
}

func newDiscordAuth(config DiscordConfig) DiscordAuth {
	secret, err := os.ReadFile(config.ClientSecret)
	if err != nil {
		panic(fmt.Errorf("read discord client secret: %w", err))
	}

	return DiscordAuth{
		clientID:        config.ClientID,
		clientSecret:    strings.TrimSpace(string(secret)),
		redirectURI:     config.RedirectURI,
		exchangeTimeout: config.ExchangeTimeout,
	}
}

const discordAuthURL = "https://discord.com/oauth2/authorize"
const discordTokenURL = "https://discord.com/api/oauth2/token"
const discordTokenRevokeURL = "https://discord.com/api/oauth2/token/revoke"
const discordAPIBaseURL = "https://discord.com/api/v10"
const discordUserEndpoint = discordAPIBaseURL + "/users/@me"
const discordAvatarURL = "https://cdn.discordapp.com/avatars/%s/%s.png"

func (a *DiscordAuth) AuthURL(state string) string {
	v := url.Values{
		"client_id":     {a.clientID},
		"response_type": {"code"},
		"redirect_uri":  {a.redirectURI},
		"scope":         {"identify"},
		"state":         {state},
	}
	return discordAuthURL + "?" + v.Encode()
}

func (a *DiscordAuth) Exchange(ctx context.Context, code string) (*User, error) {
	cli := &http.Client{
		Timeout: a.exchangeTimeout,
	}

	t, err := a.exchange(cli, code)
	if err != nil {
		return nil, fmt.Errorf("discord token exchange: %w", err)
	}

	defer func() {
		revokeErr := a.revoke(cli, t)
		if revokeErr != nil {
			l := ctxlog.Get(ctx)
			if extra, ok := ctxlog.ErrExtra(err); ok {
				l = l.With("extra", extra)
			}
			l.Error("discord token revoke", "error", revokeErr)
		}
	}()

	user, err := a.getUser(cli, t)
	if err != nil {
		return nil, fmt.Errorf("discord get user: %w", err)
	}

	return user, nil
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

func (a *DiscordAuth) exchange(cli *http.Client, code string) (token, error) {
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var dErr discordError
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&dErr)
		if err == nil {
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

func (a *DiscordAuth) revoke(cli *http.Client, t token) error {
	resp, err := cli.PostForm(discordTokenRevokeURL, url.Values{
		"client_id":       {a.clientID},
		"client_secret":   {a.clientSecret},
		"token":           {t.AccessToken},
		"token_type_hint": {"access_token"},
	})
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var dErr discordError
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&dErr)
		if err == nil {
			return &dErr
		}

		return fmt.Errorf("status %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

type discordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	GlobalName    string `json:"global_name"`
	Avatar        string `json:"avatar"`
}

func (a *DiscordAuth) getUser(cli *http.Client, t token) (*User, error) {
	req, err := http.NewRequest("GET", discordUserEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+t.AccessToken)

	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var dErr discordError
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&dErr)
		if err == nil {
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

	username := user.GlobalName
	if username == "" {
		username = fmt.Sprintf("%s#%s", user.Username, user.Discriminator)
	}

	return &User{
		ID:        user.ID,
		Username:  username,
		AvatarURL: fmt.Sprintf(discordAvatarURL, user.ID, user.Avatar),
	}, nil
}
