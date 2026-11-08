//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

package hanime1

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const cardsFixture = `<!doctype html><html><body>
<div class="video-item-container" title="第一集 &amp; 标题">
  <div class="horizontal-card">
    <a class="video-link" href="https://hanime1.me/watch?v=12345">
      <div class="thumb-container">
        <img class="main-thumb" src="https://cdn.example.test/cover/12345.jpg">
      </div>
      <div class="title">第一集 &amp; 标题</div>
    </a>
  </div>
</div>
<div class="video-item-container">
  <a class="video-link" href="/watch?v=67890"><img class="main-thumb" src="//cdn.example.test/cover/67890.jpg"></a>
  <div class="title">第二集</div>
</div>
<div class="video-item-container">
  <a class="video-link" href="https://hanime1.me/watch?v=12345"><img class="main-thumb" src="/duplicate.jpg"></a>
  <div class="title">重复</div>
</div>
<div class="video-item-container">
  <a class="video-link" href="/watch?v=broken"><div class="title">没有图</div></a>
</div>
<div class="video-item-container">
  <a class="video-link" href="/playlist?list=999"><div class="title">这是播放列表卡片，不是影片</div></a>
</div>
</body></html>`

const profileFixture = `<html><body>
<a href="https://hanime1.me/user/1067626">个人中心</a>
<div id="user-modal-dp-wrapper"><img src="/avatar/me.jpg"></div>
<span id="user-modal-name"> Alice </span>
</body></html>`

const playlistsFixture = `<!doctype html><html><body>
<div class="video-item-container" title="我的列表">
  <a class="video-link" href="/playlist?list=456"><img class="main-thumb" src="/cover456.jpg"><div class="title">我的列表</div><div class="stats-container">12 部影片</div></a>
</div>
<div class="video-item-container" title="另一个">
  <a class="video-link" href="/playlist?list=789"><div class="title">另一个</div></a>
</div>
<div class="video-item-container">
  <a class="video-link" href="/playlist?list=456"><div class="title">重复</div><div class="stats-container">99 部影片</div></a>
</div>
</body></html>`

func testClient(t *testing.T, handler http.HandlerFunc, cookies map[string]string) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := newClient(srv.Client(), cookies, srv.URL)
	return c, srv
}

func TestChallengeReturnsCookieExpired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<html><title>Just a moment...</title></html>`))
	}))
	t.Cleanup(srv.Close)
	c := newClient(srv.Client(), map[string]string{"hanime1_session": "s"}, srv.URL)

	if _, err := c.get(context.Background(), "watch", nil); !errors.Is(err, ErrCookieExpired) || !IsAuth(err) {
		t.Fatalf("get error = %v, want ErrCookieExpired and IsAuth", err)
	}
}

func TestParseCards(t *testing.T) {
	items := parseCards(cardsFixture, "https://hanime1.me/")
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	first := items[0]
	if first.RemoteID != "12345" {
		t.Fatalf("first id = %+v", first)
	}
	if first.Title != "第一集 & 标题" {
		t.Fatalf("first title = %q", first.Title)
	}
	if first.CoverURL != "https://cdn.example.test/cover/12345.jpg" {
		t.Fatalf("first cover = %q", first.CoverURL)
	}
	if first.URL != "https://hanime1.me/watch?v=12345" {
		t.Fatalf("first url = %q", first.URL)
	}
	if items[1].CoverURL != "https://cdn.example.test/cover/67890.jpg" {
		t.Fatalf("second cover = %q", items[1].CoverURL)
	}
}

func TestParsePlaylists(t *testing.T) {
	lists := parsePlaylists(playlistsFixture, "https://hanime1.me/")
	if len(lists) != 2 {
		t.Fatalf("playlists = %+v", lists)
	}
	if lists[0].Code != "456" || lists[0].Title != "我的列表" || lists[0].Count != 12 {
		t.Fatalf("first playlist = %+v", lists[0])
	}
	if lists[1].Count != -1 {
		t.Fatalf("second count = %d, want -1", lists[1].Count)
	}
}

func TestProfileUsesNumericUserID(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(profileFixture))
	}, map[string]string{"hanime1_session": "session"})
	profile, err := c.Profile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if profile.UID != "1067626" || profile.Name != "Alice" {
		t.Fatalf("profile = %+v", profile)
	}
	if profile.AvatarURL != c.baseURL+"avatar/me.jpg" {
		t.Fatalf("avatar = %q", profile.AvatarURL)
	}
}

func TestUserListPage(t *testing.T) {
	requests := 0
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(profileFixture))
		case "/user/1067626/likes":
			requests++
			if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("sort") != "latest" {
				t.Fatalf("query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(cardsFixture))
		default:
			http.NotFound(w, r)
		}
	}, map[string]string{"hanime1_session": "session"})
	page, err := c.PlaylistPage(context.Background(), "LL", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || !page.HasMore {
		t.Fatalf("page = %+v", page)
	}
	if requests != 1 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestPlaylistPageAndAuthDetection(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(profileFixture))
		case "/playlist":
			if r.URL.Query().Get("list") != "456" || r.URL.Query().Get("page") != "2" {
				t.Fatalf("query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(cardsFixture))
		default:
			http.NotFound(w, r)
		}
	}, map[string]string{"hanime1_session": "session"})
	page, err := c.PlaylistPage(context.Background(), "456", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || !page.HasMore {
		t.Fatalf("page = %+v", page)
	}

	loginBody := `<form><input name="email"><input name="password"></form>`
	authClient, _ := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(loginBody))
	}, map[string]string{"hanime1_session": "stale"})
	if _, err := authClient.PlaylistPage(context.Background(), "LL", 1); !IsAuth(err) {
		t.Fatalf("auth error = %v", err)
	}
}

func TestChallengeIsFatal(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("cf-mitigated", "challenge")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<html><title>Just a moment...</title></html>`))
	}, map[string]string{"hanime1_session": "session"})
	_, err := c.PlaylistPage(context.Background(), "LL", 1)
	if !IsFatal(err) {
		t.Fatalf("challenge error = %v", err)
	}
}

func TestCredentialRequiresSession(t *testing.T) {
	if _, err := ParseCredential([]byte(`{"cookies":{"cf_clearance":"x"}}`)); err == nil {
		t.Fatal("expected missing session error")
	}
	c, err := ParseCredential([]byte(`{"cookies":{"hanime1_session":"value"},"user_agent":"TestUA/1.0"}`))
	if err != nil || c.Cookies["hanime1_session"] != "value" || c.UserAgent != "TestUA/1.0" {
		t.Fatalf("credential = %+v, err = %v", c, err)
	}
	client, err := NewClientWithUserAgent(c.Cookies, "", c.UserAgent)
	if err != nil || client.userAgent != "TestUA/1.0" {
		t.Fatalf("client UA = %q, err = %v", client.userAgent, err)
	}
}

func TestPlaylistPageRejectsUnexpectedMarkup(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(profileFixture))
			return
		}
		_, _ = w.Write([]byte("<html><body>maintenance</body></html>"))
	}, map[string]string{"hanime1_session": "session"})
	_, err := c.PlaylistPage(context.Background(), "456", 1)
	if err == nil || IsFatal(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestAdapterFolders(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(profileFixture))
		case "/user/1067626/playlists":
			_, _ = w.Write([]byte(playlistsFixture))
		default:
			http.NotFound(w, r)
		}
	}, map[string]string{"hanime1_session": "session"})
	folders, err := NewAdapter(c).ListFolders(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 4 {
		t.Fatalf("folders = %+v", folders)
	}
	if folders[0].RemoteID != "LL" || folders[0].Kind != "favorite" || folders[0].Count != -1 {
		t.Fatalf("first folder = %+v", folders[0])
	}
	if folders[2].RemoteID != "playlist:456" || folders[2].Count != 12 {
		t.Fatalf("custom folder = %+v", folders[2])
	}
}

func TestAdapterFoldersRejectsUnexpectedPlaylistMarkup(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(profileFixture))
		case "/user/1067626/playlists":
			_, _ = w.Write([]byte("<html><body>maintenance</body></html>"))
		default:
			http.NotFound(w, r)
		}
	}, map[string]string{"hanime1_session": "session"})
	if _, err := NewAdapter(c).ListFolders(context.Background()); err == nil {
		t.Fatal("expected unrecognized playlist page error")
	}
}

func TestAdapterCustomPlaylistPage(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(profileFixture))
			return
		}
		if r.URL.Path != "/playlist" || r.URL.Query().Get("list") != "456" || r.URL.Query().Get("page") != "1" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(cardsFixture))
	}, map[string]string{"hanime1_session": "session"})
	page, err := NewAdapter(c).ListFolderPage(context.Background(), "playlist:456", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %+v", page.Items)
	}
}

func TestInvalidListCode(t *testing.T) {
	c, _ := testClient(t, func(http.ResponseWriter, *http.Request) {}, map[string]string{"hanime1_session": "session"})
	if _, err := c.PlaylistPage(context.Background(), "../etc", 1); err == nil || !strings.Contains(err.Error(), "invalid list code") {
		t.Fatalf("error = %v", err)
	}
}
