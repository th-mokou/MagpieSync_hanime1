//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

package hanime1

import (
	"context"
	"fmt"
	"strings"

	"magpiesync/internal/sync"
)

type Adapter struct {
	client *Client
}

func NewAdapter(client *Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) ListFolders(ctx context.Context) ([]sync.RemoteFolder, error) {
	out := []sync.RemoteFolder{
		{RemoteID: "LL", Title: "喜欢的影片", Kind: "favorite", Count: -1},
		{RemoteID: "WL", Title: "稍后再看", Kind: "watch_later", Count: -1},
	}
	playlists, err := a.client.Playlists(ctx)
	if err != nil {
		// Do not silently drop custom playlists: a transient failure would make
		// ReconcileFolders archive them. The engine marks the folder/run partial
		// instead when enumeration fails.
		return nil, err
	}
	for _, p := range playlists {
		out = append(out, sync.RemoteFolder{
			RemoteID: "playlist:" + p.Code,
			Title:    p.Title,
			Kind:     "playlist",
			Count:    p.Count,
		})
	}
	return out, nil
}

func (a *Adapter) ListFolderPage(ctx context.Context, remoteID string, page, pageSize int) (sync.RemotePage, error) {
	list := remoteID
	if strings.HasPrefix(list, "playlist:") {
		list = strings.TrimPrefix(list, "playlist:")
	}
	if !validListCode(list) {
		return sync.RemotePage{}, fmt.Errorf("hanime1: unsupported folder %q", remoteID)
	}
	p, err := a.client.PlaylistPage(ctx, list, page)
	if err != nil {
		return sync.RemotePage{}, err
	}
	return sync.RemotePage{Items: p.Items, HasMore: p.HasMore}, nil
}

// Close releases resources held by the read-only client. It is kept so the
// sync engine can dispose of each source; nothing needs releasing now that
// the browser fallback has been removed.
func (a *Adapter) Close() error {
	if a == nil || a.client == nil {
		return nil
	}
	return a.client.Close()
}
