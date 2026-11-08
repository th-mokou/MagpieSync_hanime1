//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

package hanime1

import (
	"testing"

	"magpiesync/internal/sites"
)

func TestHanime1SiteDescriptor(t *testing.T) {
	site, ok := sites.Get("hanime1")
	if !ok {
		t.Fatal("hanime1 site is not registered")
	}
	if !site.Ready || !site.Download {
		t.Fatalf("hanime1 readiness = %+v", site)
	}
	if len(site.DownloadQualities) != 4 {
		t.Fatalf("hanime1 download qualities = %+v", site.DownloadQualities)
	}
	if len(site.Login.Methods) == 0 || site.Login.Methods[0].ID != "cookie" {
		t.Fatalf("hanime1 login methods = %+v", site.Login.Methods)
	}
	if asset, ok := sites.Asset("hanime1", "logo.png"); !ok || len(asset) == 0 {
		t.Fatal("hanime1 logo is not registered")
	}
	want := map[string]bool{"favorite": true, "watch_later": true, "playlist": true}
	for _, c := range site.Categories {
		delete(want, c.ID)
	}
	if len(want) != 0 {
		t.Fatalf("missing categories: %v", want)
	}
	if _, ok := sites.DriverFor("hanime1"); !ok {
		t.Fatal("hanime1 driver is not registered")
	}
}
