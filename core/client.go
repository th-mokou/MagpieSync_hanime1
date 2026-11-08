//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

// Package hanime1 is a read-only Hanime1 adapter. It only performs GET
// requests against the site; authentication is supplied as browser cookies.
package hanime1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const BaseURL = "https://hanime1.me"

const (
	requestTimeout = 45 * time.Second
	maxBodyBytes   = 8 << 20
)

var (
	// ErrAuth means the copied session cookie is missing or no longer valid.
	ErrAuth = errors.New("hanime1: authentication required")
	// ErrChallenge means Cloudflare rejected the request before the site could
	// be read. It is fatal for a sync run so no removal step can run.
	ErrChallenge = errors.New("hanime1: Cloudflare verification required; use the same proxy IP and copy a fresh cf_clearance cookie")
	// ErrCookieExpired means Cloudflare challenged a request and there is no
	// browser fallback to complete the JavaScript challenge. The saved cookie,
	// including cf_clearance, has to be refreshed in the account page.
	ErrCookieExpired = errors.New("hanime1: Cloudflare challenge could not be completed; 请更新账户 Cookie 并保持相同出口 IP")
)

type HTTPError struct {
	StatusCode int
	Path       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("hanime1: HTTP %d for %s", e.StatusCode, e.Path)
}

func IsAuth(err error) bool {
	return errors.Is(err, ErrAuth) || errors.Is(err, ErrCookieExpired)
}

func IsFatal(err error) bool {
	return errors.Is(err, ErrAuth) || errors.Is(err, ErrChallenge) || errors.Is(err, ErrCookieExpired)
}

// Credential is the encrypted account payload. It intentionally contains
// cookies only; no site write token is used by this adapter.
type Credential struct {
	Cookies   map[string]string `json:"cookies"`
	UserAgent string            `json:"user_agent,omitempty"`
}

func ParseCredential(raw []byte) (*Credential, error) {
	var c Credential
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse hanime1 credential: %w", err)
	}
	if c.Cookies == nil || strings.TrimSpace(c.Cookies["hanime1_session"]) == "" {
		return nil, errors.New("hanime1 credential is missing hanime1_session")
	}
	return &c, nil
}

type Profile struct {
	UID       string
	Name      string
	AvatarURL string
}

type Client struct {
	http      *http.Client
	cookies   map[string]string
	baseURL   string
	userAgent string
	proxyURL  string

	mu      sync.Mutex
	profile *Profile
}

func NewClient(cookies map[string]string, proxyURL string) (*Client, error) {
	return NewClientWithUserAgent(cookies, proxyURL, "")
}

// NewClientWithUserAgent keeps the browser UA that obtained cf_clearance.
// Cloudflare binds the clearance cookie to that UA, so sync requests must keep
// using the same value.
func NewClientWithUserAgent(cookies map[string]string, proxyURL, userAgent string) (*Client, error) {
	hc, err := newSiteHTTPClient(proxyURL, requestTimeout)
	if err != nil {
		return nil, err
	}
	c := newClient(hc, cookies, BaseURL)
	c.proxyURL = strings.TrimSpace(proxyURL)
	if ua := strings.TrimSpace(userAgent); ua != "" {
		if strings.ContainsAny(ua, "\r\n") {
			return nil, errors.New("hanime1: invalid user agent")
		}
		c.userAgent = ua
	}
	return c, nil
}

func newClient(hc *http.Client, cookies map[string]string, baseURL string) *Client {
	cp := make(map[string]string, len(cookies))
	for k, v := range cookies {
		cp[k] = v
	}
	return &Client{
		http:      hc,
		cookies:   cp,
		baseURL:   strings.TrimRight(baseURL, "/") + "/",
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	}
}

func (c *Client) Profile(ctx context.Context) (Profile, error) {
	if strings.TrimSpace(c.cookies["hanime1_session"]) == "" {
		return Profile{}, ErrAuth
	}
	c.mu.Lock()
	if c.profile != nil {
		p := *c.profile
		c.mu.Unlock()
		return p, nil
	}
	c.mu.Unlock()

	body, err := c.get(ctx, "", nil)
	if err != nil {
		return Profile{}, err
	}
	p, err := parseProfileHTML(body, c.baseURL)
	if err != nil {
		return Profile{}, err
	}
	c.mu.Lock()
	c.profile = &p
	c.mu.Unlock()
	return p, nil
}

func parseProfileHTML(body, baseURL string) (Profile, error) {
	doc := parseHTML(body)
	name := textContent(findID(doc, "user-modal-name"))
	if name == "" {
		name = textContent(findClassFirst(doc, "profile-display-name"))
	}
	uid := profileUserID(doc)
	if uid == "" {
		uid = profileNumericID(textContent(findID(doc, "user-modal-dp-wrapper")))
	}
	if name == "" && uid == "" {
		return Profile{}, ErrAuth
	}
	if name == "" {
		name = uid
	}
	if uid == "" {
		uid = name
	}
	avatar := ""
	if wrapper := findID(doc, "user-modal-dp-wrapper"); wrapper != nil {
		if img := findFirst(wrapper, func(n *html.Node) bool { return isElement(n, "img") }); img != nil {
			avatar = imageURL(img)
		}
	}
	if avatar == "" {
		if img := findFirst(doc, func(n *html.Node) bool {
			return isElement(n, "img") && strings.Contains(attr(n, "class"), "profile-avatar")
		}); img != nil {
			avatar = imageURL(img)
		}
	}
	return Profile{UID: uid, Name: name, AvatarURL: resolveURL(baseURL, avatar)}, nil
}

func (c *Client) userID(ctx context.Context) (string, error) {
	profile, err := c.Profile(ctx)
	if err != nil {
		return "", err
	}
	if profile.UID == "" {
		return "", ErrAuth
	}
	return profile.UID, nil
}

func (c *Client) Playlists(ctx context.Context) ([]Playlist, error) {
	uid, err := c.userID(ctx)
	if err != nil {
		return nil, err
	}
	body, err := c.get(ctx, "user/"+url.PathEscape(uid)+"/playlists", nil)
	if err != nil {
		return nil, err
	}
	playlists := parsePlaylists(body, c.baseURL)
	if len(playlists) == 0 && !recognizedPlaylistsPage(body) {
		return nil, errors.New("hanime1: playlist page structure not recognized")
	}
	return playlists, nil
}

func (c *Client) PlaylistPage(ctx context.Context, list string, page int) (Page, error) {
	if !validListCode(list) {
		return Page{}, fmt.Errorf("hanime1: invalid list code %q", list)
	}
	if page < 1 {
		page = 1
	}
	if list == "LL" || list == "WL" {
		return c.userListPage(ctx, list, page)
	}
	body, err := c.get(ctx, "playlist", url.Values{
		"list": {list},
		"page": {fmt.Sprintf("%d", page)},
	})
	if err != nil {
		return Page{}, err
	}
	items := parseCards(body, c.baseURL)
	if len(items) == 0 && !recognizedListPage(body) {
		return Page{}, errors.New("hanime1: playlist page structure not recognized")
	}
	return Page{Items: items, HasMore: len(items) > 0}, nil
}

func (c *Client) userListPage(ctx context.Context, list string, page int) (Page, error) {
	uid, err := c.userID(ctx)
	if err != nil {
		return Page{}, err
	}
	path := "user/" + url.PathEscape(uid) + "/likes"
	if list == "WL" {
		path = "user/" + url.PathEscape(uid) + "/saves"
	}
	body, err := c.get(ctx, path, url.Values{
		"page": {fmt.Sprintf("%d", page)},
		"sort": {"latest"},
	})
	if err != nil {
		return Page{}, err
	}
	items := parseCards(body, c.baseURL)
	if len(items) == 0 && !recognizedListPage(body) {
		return Page{}, errors.New("hanime1: user list page structure not recognized")
	}
	return Page{Items: items, HasMore: len(items) > 0}, nil
}

func recognizedListPage(body string) bool {
	return strings.Contains(body, "home-rows-videos-wrapper") ||
		strings.Contains(body, "video-item-container") ||
		strings.Contains(body, "playlist-video-list")
}

func recognizedPlaylistsPage(body string) bool {
	return strings.Contains(body, "video-item-container") ||
		strings.Contains(body, "single-user-playlist") ||
		strings.Contains(body, "home-rows-videos-wrapper")
}

func (c *Client) get(ctx context.Context, path string, query url.Values) (string, error) {
	body, err := c.getHTTP(ctx, path, c.pageURL(path, query))
	if errors.Is(err, ErrChallenge) {
		// Cloudflare rejected the request despite valid cookies. There is no
		// browser fallback anymore, so report the actionable condition
		// (refresh Cookie) and let the sync engine mark the account as
		// needing re-authentication.
		return "", ErrCookieExpired
	}
	return body, err
}

func (c *Client) pageURL(path string, query url.Values) string {
	raw := c.baseURL + path
	if len(query) > 0 {
		raw += "?" + query.Encode()
	}
	return raw
}

// Close releases idle connections held by the client.
func (c *Client) Close() error {
	c.http.CloseIdleConnections()
	return nil
}

func (c *Client) getHTTP(ctx context.Context, path, raw string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", fmt.Errorf("hanime1: build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", c.baseURL)
	for name, value := range c.cookies {
		if value == "" {
			continue
		}
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("hanime1: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", fmt.Errorf("hanime1: read response: %w", err)
	}
	text := string(body)
	finalURL := resp.Request.URL
	finalPath := finalURL.Path

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == 419 {
		return "", ErrAuth
	}
	if isChallenge(resp, text) || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return "", ErrChallenge
	}
	if finalPath != "/" && strings.HasSuffix(finalPath, "/login") && path != "login" {
		return "", ErrAuth
	}
	if looksLikeLogin(text) && path != "login" {
		return "", ErrAuth
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", &HTTPError{StatusCode: resp.StatusCode, Path: finalPath}
	}
	return text, nil
}

func isChallenge(resp *http.Response, body string) bool {
	if strings.EqualFold(resp.Header.Get("cf-mitigated"), "challenge") {
		return true
	}
	low := strings.ToLower(body)
	return strings.Contains(low, "/cdn-cgi/challenge-platform") ||
		strings.Contains(low, "just a moment...") ||
		strings.Contains(low, "cf-chl")
}

func looksLikeLogin(body string) bool {
	low := strings.ToLower(body)
	// The logged-in home page may keep a hidden login form in the DOM. The
	// user-modal wrapper is a reliable positive signal, so do not treat a page
	// containing it as logged out merely because an email/password form exists.
	return strings.Contains(low, `name="email"`) &&
		strings.Contains(low, `name="password"`) &&
		!strings.Contains(low, "user-modal-dp-wrapper")
}

func validListCode(code string) bool {
	if code == "LL" || code == "WL" {
		return true
	}
	if code == "" {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var numericIDRe = regexp.MustCompile(`@\s*(\d+)`)

func profileNumericID(text string) string {
	if m := numericIDRe.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return ""
}
