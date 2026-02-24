package main

import (
	"bytes"
	"cc/internal/auth"
	"cc/internal/server"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"

	"github.com/alexflint/go-arg"
)

func UserString(user *auth.User) string {
	if user.Admin {
		return fmt.Sprintf("<ADMIN> %s", user.Name)
	}
	return user.Name
}

func DecodeResponse[T any](resp *http.Response) (T, error) {
	var apiResp T

	dec := json.NewDecoder(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var apiErr server.APIError
		err := dec.Decode(&apiErr)
		if err != nil || apiErr.Err == "" {
			return apiResp, fmt.Errorf("%d: %s", resp.StatusCode, resp.Status)
		}
		return apiResp, &apiErr
	}

	err := dec.Decode(&apiResp)
	if err != nil {
		return apiResp, fmt.Errorf("decode response: %w", err)
	}
	return apiResp, nil
}

type FindUsersCmd struct {
	Name string `arg:"required,positional"`
}

func (c *FindUsersCmd) Do(ctx context.Context, p *arg.Parser, cli *http.Client, args Args) error {
	v := url.Values{}
	v.Set("name", c.Name)

	u := &url.URL{
		Scheme:   "http",
		Host:     net.JoinHostPort("localhost", strconv.Itoa(args.Port)),
		Path:     "/users",
		RawQuery: v.Encode(),
	}

	resp, err := cli.Get(u.String())
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}
	defer resp.Body.Close()

	apiResp, err := DecodeResponse[server.APIListUsersResponse](resp)
	if err != nil {
		return err
	}

	keys := slices.Sorted(maps.Keys(apiResp.Users))
	for _, key := range keys {
		fmt.Printf("%s: %s\n", key, UserString(apiResp.Users[key]))
	}
	return nil
}

type ListUsersCmd struct {
}

func (c *ListUsersCmd) Do(ctx context.Context, p *arg.Parser, cli *http.Client, args Args) error {
	u := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("localhost", strconv.Itoa(args.Port)),
		Path:   "/users",
	}

	resp, err := cli.Get(u.String())
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}
	defer resp.Body.Close()

	apiResp, err := DecodeResponse[server.APIListUsersResponse](resp)
	if err != nil {
		return err
	}

	keys := slices.Sorted(maps.Keys(apiResp.Users))
	for _, key := range keys {
		fmt.Printf("%s: %s\n", key, UserString(apiResp.Users[key]))
	}
	return nil
}

type GetUserCmd struct {
	ID string `arg:"required,positional"`
}

func (c *GetUserCmd) Do(ctx context.Context, p *arg.Parser, cli *http.Client, args Args) error {
	u := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("localhost", strconv.Itoa(args.Port)),
		Path:   path.Join("/user", c.ID),
	}

	resp, err := cli.Get(u.String())
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}
	defer resp.Body.Close()

	apiResp, err := DecodeResponse[server.APIGetUserResponse](resp)
	if err != nil {
		return err
	}

	fmt.Printf("%s: %s\n", c.ID, UserString(apiResp.User))
	return nil
}

type UpdateUserCmd struct {
	ID         string `arg:"required,positional"`
	SetAdmin   bool   `arg:"-a,--admin"`
	UnsetAdmin bool   `arg:"-u,--unset-admin"`
}

func (c *UpdateUserCmd) Do(ctx context.Context, p *arg.Parser, cli *http.Client, args Args) error {
	if c.SetAdmin && c.UnsetAdmin {
		p.Fail("only one of -a or -u can be set")
	}

	u := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("localhost", strconv.Itoa(args.Port)),
		Path:   path.Join("/user", c.ID),
	}

	req := server.APIUpdateUserRequest{}
	if c.SetAdmin {
		req.Admin = new(true)
	} else if c.UnsetAdmin {
		req.Admin = new(false)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	resp, err := cli.Post(u.String(), "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}
	defer resp.Body.Close()

	apiResp, err := DecodeResponse[server.APIGetUserResponse](resp)
	if err != nil {
		return err
	}

	fmt.Printf("%s: %s\n", c.ID, UserString(apiResp.User))
	return nil
}

type UserCmd struct {
	Find   *FindUsersCmd  `arg:"subcommand:find"`
	List   *ListUsersCmd  `arg:"subcommand:list"`
	Get    *GetUserCmd    `arg:"subcommand:get"`
	Update *UpdateUserCmd `arg:"subcommand:update"`
}
