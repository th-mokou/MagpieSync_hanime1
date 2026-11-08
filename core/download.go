//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

package hanime1

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// DownloadOption is one official download link from /download?v=<code>.
type DownloadOption struct {
	Quality int
	Label   string
	URL     string
	Headers map[string]string
}

var downloadResolutionRe = regexp.MustCompile(`(\d{3,4})`)

// DownloadOptions reads the official download page and returns every MP4
// resolution offered by the site. It is a read-only GET; Cloudflare fallback is
// handled by Client.get.
func (c *Client) DownloadOptions(ctx context.Context, pageURL string) ([]DownloadOption, error) {
	u, err := url.Parse(strings.TrimSpace(pageURL))
	if err != nil || u.Scheme != "https" || !isHanime1Host(u.Hostname()) {
		return nil, errors.New("hanime1: invalid video URL")
	}
	code := strings.TrimSpace(u.Query().Get("v"))
	if !digitsOnly(code) {
		return nil, errors.New("hanime1: video URL has no numeric v parameter")
	}
	body, err := c.get(ctx, "download", url.Values{"v": {code}})
	if err != nil {
		return nil, err
	}
	opts, err := parseDownloadOptions(body)
	if err != nil {
		return nil, err
	}
	opts = filterDownloadOptions(opts)
	if len(opts) == 0 {
		return nil, errors.New("hanime1: download page has no usable MP4 links")
	}
	cookie := hanime1CookieHeader(c.cookies)
	referer := c.baseURL + "watch?v=" + url.QueryEscape(code)
	for i := range opts {
		opts[i].Headers = map[string]string{
			"Referer":    referer,
			"User-Agent": c.userAgent,
			"Accept":     "*/*",
		}
		// Official MP4 links are served by the site's CDN (vdownload.hembed.com
		// today). Never forward the Hanime1 session cookie to that third-party
		// host; the signed URL is enough. Keep cookies for hanime1.me links so
		// redirects and Cloudflare-protected endpoints still work.
		link, _ := url.Parse(opts[i].URL)
		if cookie != "" && isHanime1Host(link.Hostname()) {
			opts[i].Headers["Cookie"] = cookie
		}
	}
	return opts, nil
}

// filterDownloadOptions keeps only official HTTPS links. Download links are
// normally signed URLs on the site's CDN, not hanime1.me itself, so both the
// site host and its CDN domain are accepted. Anything else is dropped instead
// of being fetched by the download manager.
func filterDownloadOptions(opts []DownloadOption) []DownloadOption {
	filtered := opts[:0]
	for _, opt := range opts {
		link, err := url.Parse(opt.URL)
		if err != nil || !strings.HasPrefix(opt.URL, "https://") || !isHanime1AssetHost(link.Hostname()) {
			continue
		}
		filtered = append(filtered, opt)
	}
	return filtered
}

func isHanime1Host(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "hanime1.me" || strings.HasSuffix(host, ".hanime1.me")
}

// hembed.com is the CDN domain that serves Hanime1's signed MP4 download links
// (for example https://vdownload.hembed.com/<code>-720p.mp4?secure=...).
func isHanime1AssetHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return isHanime1Host(host) || host == "hembed.com" || strings.HasSuffix(host, ".hembed.com")
}

func hanime1CookieHeader(cookies map[string]string) string {
	parts := make([]string, 0, len(cookies))
	for name, value := range cookies {
		if strings.TrimSpace(value) == "" {
			continue
		}
		parts = append(parts, name+"="+value)
	}
	return strings.Join(parts, "; ")
}

// parseDownloadOptions parses the table.download-table rows: the second cell
// is the resolution and the fifth cell contains an anchor with data-url.
func parseDownloadOptions(body string) ([]DownloadOption, error) {
	doc := parseHTML(body)
	table := findClassFirst(doc, "download-table")
	if table == nil {
		return nil, errors.New("hanime1: download page structure not recognized")
	}
	seen := map[int]bool{}
	var out []DownloadOption
	for _, row := range findAll(table, func(n *html.Node) bool { return isElement(n, "tr") }) {
		cells := directChildren(row, "td")
		if len(cells) < 5 {
			continue
		}
		label := strings.TrimSpace(textContent(cells[1]))
		link := findFirst(cells[4], func(n *html.Node) bool {
			return isElement(n, "a") && (attr(n, "data-url") != "" || attr(n, "href") != "")
		})
		raw := strings.TrimSpace(attr(link, "data-url"))
		if raw == "" {
			raw = strings.TrimSpace(attr(link, "href"))
		}
		if raw == "" {
			continue
		}
		target := resolveURL(BaseURL+"/", raw)
		if target == "" {
			continue
		}
		quality := resolutionQuality(label)
		if quality <= 0 {
			continue
		}
		if seen[quality] {
			continue
		}
		seen[quality] = true
		out = append(out, DownloadOption{
			Quality: quality,
			Label:   label,
			URL:     target,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("hanime1: download page has no MP4 links")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Quality > out[j].Quality })
	return out, nil
}

func directChildren(n *html.Node, tag string) []*html.Node {
	var out []*html.Node
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if isElement(child, tag) {
			out = append(out, child)
		}
	}
	return out
}

func resolutionQuality(label string) int {
	m := downloadResolutionRe.FindStringSubmatch(label)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	switch {
	case n >= 2160:
		return 2160
	case n >= 1080:
		return 1080
	case n >= 720:
		return 720
	case n >= 480:
		return 480
	case n >= 240:
		return 240
	default:
		return 0
	}
}
