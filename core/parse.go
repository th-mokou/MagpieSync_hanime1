//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

package hanime1

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"magpiesync/internal/sync"

	"golang.org/x/net/html"
)

type Page struct {
	Items   []sync.RemoteItem
	HasMore bool
}

type Playlist struct {
	Code  string
	Title string
	Count int
}

func parseHTML(body string) *html.Node {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return &html.Node{Type: html.DocumentNode}
	}
	return doc
}

func isElement(n *html.Node, tag string) bool {
	return n != nil && n.Type == html.ElementNode && (tag == "" || n.Data == tag)
}

func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func classes(n *html.Node) []string {
	return strings.Fields(attr(n, "class"))
}

func hasClass(n *html.Node, class string) bool {
	for _, c := range classes(n) {
		if c == class {
			return true
		}
	}
	return false
}

func walk(n *html.Node, fn func(*html.Node) bool) bool {
	if n == nil {
		return false
	}
	if fn(n) {
		return true
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if walk(child, fn) {
			return true
		}
	}
	return false
}

func findFirst(root *html.Node, pred func(*html.Node) bool) *html.Node {
	var found *html.Node
	walk(root, func(n *html.Node) bool {
		if pred(n) {
			found = n
			return true
		}
		return false
	})
	return found
}

func findAll(root *html.Node, pred func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	walk(root, func(n *html.Node) bool {
		if pred(n) {
			out = append(out, n)
		}
		return false
	})
	return out
}

func findID(root *html.Node, id string) *html.Node {
	return findFirst(root, func(n *html.Node) bool { return isElement(n, "") && attr(n, "id") == id })
}

func findClassFirst(root *html.Node, class string) *html.Node {
	return findFirst(root, func(n *html.Node) bool { return isElement(n, "") && hasClass(n, class) })
}

func findClassAll(root *html.Node, class string) []*html.Node {
	return findAll(root, func(n *html.Node) bool { return isElement(n, "") && hasClass(n, class) })
}

func textContent(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
			b.WriteByte(' ')
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func resolveURL(base, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	b, err := url.Parse(base)
	if err != nil {
		return raw
	}
	return b.ResolveReference(u).String()
}

func imageURL(img *html.Node) string {
	if img == nil {
		return ""
	}
	for _, key := range []string{"data-src", "data-original", "data-lazy-src", "src"} {
		if raw := strings.TrimSpace(attr(img, key)); raw != "" && !strings.HasPrefix(raw, "data:") {
			return raw
		}
	}
	return ""
}

func videoID(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	id := strings.TrimSpace(u.Query().Get("v"))
	if id == "" {
		return ""
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return id
}

func parseCards(body, baseURL string) []sync.RemoteItem {
	doc := parseHTML(body)
	cards := findClassAll(doc, "video-item-container")
	items := make([]sync.RemoteItem, 0, len(cards))
	seen := make(map[string]bool, len(cards))
	for _, card := range cards {
		link := findFirst(card, func(n *html.Node) bool {
			return isElement(n, "a") && strings.Contains(attr(n, "href"), "watch?v=")
		})
		id := videoID(attr(link, "href"))
		if id == "" || seen[id] {
			continue
		}
		title := textContent(findClassFirst(card, "video-title"))
		if title == "" {
			title = textContent(findClassFirst(card, "title"))
		}
		if title == "" {
			title = strings.TrimSpace(attr(card, "title"))
		}
		if title == "" {
			continue
		}
		seen[id] = true

		var cover string
		if thumb := findClassFirst(card, "main-thumb"); thumb != nil {
			cover = imageURL(thumb)
		}
		if cover == "" {
			if img := findFirst(card, func(n *html.Node) bool { return isElement(n, "img") }); img != nil {
				cover = imageURL(img)
			}
		}
		items = append(items, sync.RemoteItem{
			RemoteID: id,
			Title:    title,
			URL:      resolveURL(baseURL, "/watch?v="+id),
			CoverURL: resolveURL(baseURL, cover),
		})
	}
	return items
}

var playlistCountRe = regexp.MustCompile(`(\d+)\s*部`)
var userIDRe = regexp.MustCompile(`/user/(\d+)(?:/|$)`)

func profileUserID(doc *html.Node) string {
	for _, a := range findAll(doc, func(n *html.Node) bool { return isElement(n, "a") }) {
		if m := userIDRe.FindStringSubmatch(attr(a, "href")); m != nil {
			return m[1]
		}
	}
	if m := userIDRe.FindStringSubmatch(attr(doc, "href")); m != nil {
		return m[1]
	}
	return ""
}

func parsePlaylists(body, baseURL string) []Playlist {
	doc := parseHTML(body)
	cards := findClassAll(doc, "video-item-container")
	out := make([]Playlist, 0, len(cards))
	seen := map[string]bool{"LL": true, "WL": true}
	for _, card := range cards {
		link := findFirst(card, func(n *html.Node) bool {
			return isElement(n, "a") && strings.Contains(attr(n, "href"), "playlist?list=")
		})
		code := ""
		if u, err := url.Parse(attr(link, "href")); err == nil {
			code = strings.TrimSpace(u.Query().Get("list"))
		}
		if code == "" || seen[code] || !digitsOnly(code) {
			continue
		}
		seen[code] = true
		title := textContent(findClassFirst(card, "title"))
		if title == "" {
			title = strings.TrimSpace(attr(card, "title"))
		}
		if title == "" {
			title = "播放列表 " + code
		}
		count := -1
		if stats := findClassFirst(card, "stats-container"); stats != nil {
			if m := playlistCountRe.FindStringSubmatch(textContent(stats)); m != nil {
				if v, err := strconv.Atoi(m[1]); err == nil {
					count = v
				}
			}
		}
		out = append(out, Playlist{Code: code, Title: title, Count: count})
	}
	if len(cards) > 0 {
		return out
	}

	// Older layouts used .single-user-playlist cards. Keep this as a fallback
	// while the current profile playlists page uses .video-item-container.
	nodes := findClassAll(doc, "single-user-playlist")
	for _, node := range nodes {
		link := findFirst(node, func(n *html.Node) bool {
			return isElement(n, "a") && strings.Contains(attr(n, "href"), "list=")
		})
		code := ""
		if u, err := url.Parse(attr(link, "href")); err == nil {
			code = strings.TrimSpace(u.Query().Get("list"))
		}
		if code == "" || seen[code] || !digitsOnly(code) {
			continue
		}
		seen[code] = true
		title := textContent(findClassFirst(node, "card-mobile-title"))
		if title == "" {
			title = "播放列表 " + code
		}
		count := -1
		if total := findFirst(node, func(n *html.Node) bool {
			return isElement(n, "div") && attr(n, "style") != ""
		}); total != nil {
			if v, err := strconv.Atoi(strings.TrimSpace(textContent(total))); err == nil && v >= 0 {
				count = v
			}
		}
		out = append(out, Playlist{Code: code, Title: title, Count: count})
	}
	return out
}

func digitsOnly(s string) bool {
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
