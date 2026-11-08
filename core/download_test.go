//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

package hanime1

import (
	"strings"
	"testing"
)

func TestParseDownloadOptions(t *testing.T) {
	body := `<html><body><h3>Title</h3><table class="download-table">
		<tr><th>#</th><th>分辨率</th><th>格式</th><th>大小</th><th>链接</th></tr>
		<tr><td>1</td><td>1080P</td><td>MP4</td><td>1 GB</td><td><a data-url="https://vdownload.hembed.com/13078-1080p.mp4?secure=abc,123">下载</a></td></tr>
		<tr><td>2</td><td>720P</td><td>MP4</td><td>700 MB</td><td><a data-url="https://vdownload.hembed.com/13078-720p.mp4?secure=def,123">下载</a></td></tr>
		<tr><td>3</td><td>480P</td><td>MP4</td><td>400 MB</td><td><a href="//hanime1.me/download.php?id=480">下载</a></td></tr>
	</table></body></html>`
	opts, err := parseDownloadOptions(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) != 3 {
		t.Fatalf("options = %+v", opts)
	}
	if opts[0].Quality != 1080 || opts[1].Quality != 720 || opts[2].Quality != 480 {
		t.Fatalf("qualities = %+v", opts)
	}
	if opts[0].Label != "1080P" || opts[0].URL != "https://vdownload.hembed.com/13078-1080p.mp4?secure=abc,123" {
		t.Fatalf("first option = %+v", opts[0])
	}
	if opts[2].URL != "https://hanime1.me/download.php?id=480" {
		t.Fatalf("protocol-relative URL = %q", opts[2].URL)
	}
}

func TestParseDownloadOptionsMissingTable(t *testing.T) {
	if _, err := parseDownloadOptions("<html><body>no table</body></html>"); err == nil {
		t.Fatal("expected structure error")
	}
}

func TestFilterDownloadOptionsAllowsOfficialCDN(t *testing.T) {
	opts := []DownloadOption{
		{Quality: 1080, URL: "https://vdownload.hembed.com/13078-1080p.mp4?secure=abc"},
		{Quality: 720, URL: "https://hanime1.me/download.php?id=720"},
		{Quality: 480, URL: "https://evil.example/video.mp4"},
		{Quality: 240, URL: "http://vdownload.hembed.com/insecure.mp4"},
	}
	got := filterDownloadOptions(opts)
	if len(got) != 2 {
		t.Fatalf("filtered options = %+v", got)
	}
	if !strings.Contains(got[0].URL, "vdownload.hembed.com") || !strings.Contains(got[1].URL, "hanime1.me") {
		t.Fatalf("unexpected filtered links = %+v", got)
	}
}

func TestIsHanime1AssetHost(t *testing.T) {
	cases := map[string]bool{
		"hanime1.me":             true,
		"www.hanime1.me":         true,
		"vdownload.hembed.com":   true,
		"hembed.com":             true,
		"hembed.com.evil.test":   false,
		"evilembed.com":          false,
		"vdownload.hembed.co":    false,
		"not-hanime1.me.evil.io": false,
	}
	for host, want := range cases {
		if got := isHanime1AssetHost(host); got != want {
			t.Errorf("isHanime1AssetHost(%q) = %v, want %v", host, got, want)
		}
	}
}
