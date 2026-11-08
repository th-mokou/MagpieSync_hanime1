//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

package hanime1

import (
	"magpiesync/internal/sites"
	"magpiesync/internal/sites/plugins/hanime1/core"
)

func init() {
	sites.Register(sites.Site{
		ID:       "hanime1",
		Name:     "Hanime1",
		Homepage: "https://hanime1.me",
		Logo:     "/api/site-assets/hanime1/logo.png",
		Ready:    true,
		Download: true,
		DownloadQualities: []sites.DownloadQuality{
			{Value: 1080, Label: "1080P"},
			{Value: 720, Label: "720P"},
			{Value: 480, Label: "480P"},
			{Value: 240, Label: "240P"},
		},
		Categories: []sites.Category{
			{ID: "favorite", Name: "喜欢的影片"},
			{ID: "watch_later", Name: "稍后再看"},
			{ID: "playlist", Name: "播放列表"},
		},
		Naming: &sites.SiteNaming{
			VideoTemplate: "{{title}}",
			PageTemplate:  "{{page_title}}",
			TemplateVars:  []string{"id", "title", "page_title", "page_no"},
		},
		Login: sites.LoginConfig{
			Hint: "优先使用 Cookie 登录：不需要本机浏览器，服务器也能用；Cookie 过期后重新粘贴即可。",
			Methods: []sites.LoginMethod{
				{ID: "cookie", Label: "使用 Cookie（推荐）"},
			},
			Cookie: &sites.CookieConfig{
				Placeholder: "hanime1_session=...; cf_clearance=...",
				Help:        "从已登录的浏览器复制完整 Cookie；至少需要站点主会话字段，遇到 Cloudflare 验证时还需要同一出口 IP 下的 cf_clearance。",
				RequiresUA:  true,
				UAHelp:      "必须与生成 Cookie 的浏览器一致；使用无浏览器模式时，Cookie 过期后需要重新复制。",
			},
		},
		DownloadInfo: sites.DownloadConfig{
			ActorRole:    "作者",
			ImageReferer: "https://hanime1.me/",
		},
	})
	sites.RegisterDriver("hanime1", driver{})
}

type driver struct{}

func (driver) SiteID() string { return "hanime1" }

func (driver) IsAuth(err error) bool { return hanime1.IsAuth(err) }

func (driver) IsFatal(err error) bool { return hanime1.IsFatal(err) }
