//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

package hanime1

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"magpiesync/internal/cookieutil"
	"magpiesync/internal/sites"
	"magpiesync/internal/sites/plugins/hanime1/core"
)

func (driver) CookieLogin(ctx context.Context, in sites.CookieInput) (*sites.LoginResult, error) {
	cookies := cookieutil.Parse(in.RawCookie)
	for name, value := range cookies {
		value = strings.TrimSpace(value)
		if value == "" {
			delete(cookies, name)
			continue
		}
		cookies[name] = value
	}
	if cookies["hanime1_session"] == "" {
		return nil, &sites.AuthError{Kind: sites.AuthBadRequest, Message: "hanime1_session is required"}
	}
	client, err := hanime1.NewClientWithUserAgent(cookies, in.ProxyURL, in.UserAgent)
	if err != nil {
		return nil, &sites.AuthError{Kind: sites.AuthBadRequest, Message: err.Error(), Err: err}
	}
	defer client.Close()
	profile, err := client.Profile(ctx)
	if err != nil {
		slog.Warn("hanime1 profile check failed", "site", "hanime1", "cookies", cookieNames(cookies), "error", err)
		if hanime1.IsAuth(err) {
			return nil, &sites.AuthError{Kind: sites.AuthUnauthorized, Message: "Cookie 无效或已过期", Err: err}
		}
		return nil, &sites.AuthError{Kind: sites.AuthBadGateway, Message: err.Error(), Err: err}
	}
	uid := strings.TrimSpace(profile.UID)
	name := strings.TrimSpace(profile.Name)
	if name == "" {
		name = uid
	}
	if uid == "" {
		return nil, errors.New("Hanime1 未返回数字账户 ID")
	}
	raw, err := json.Marshal(hanime1.Credential{Cookies: cookies, UserAgent: in.UserAgent})
	if err != nil {
		return nil, err
	}
	return &sites.LoginResult{
		UID:        uid,
		Name:       name,
		AvatarURL:  profile.AvatarURL,
		Credential: raw,
		AutoSync:   true,
	}, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func cookieNames(cookies map[string]string) []string {
	names := make([]string, 0, len(cookies))
	for name := range cookies {
		names = append(names, name)
	}
	return names
}
