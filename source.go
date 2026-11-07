//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

package hanime1

import (
	"context"

	"magpiesync/internal/sites"
	"magpiesync/internal/sites/plugins/hanime1/core"
	syncpkg "magpiesync/internal/sync"
)

func (driver) NewSource(_ context.Context, in sites.AccountContext) (syncpkg.Source, error) {
	cred, err := hanime1.ParseCredential(in.Credential)
	if err != nil {
		return nil, err
	}
	client, err := hanime1.NewClientWithUserAgent(cred.Cookies, in.ProxyURL, cred.UserAgent)
	if err != nil {
		return nil, err
	}
	return hanime1.NewAdapter(client), nil
}

func (driver) NewDirectResolver(_ context.Context, in sites.AccountContext) (sites.DirectResolver, error) {
	cred, err := hanime1.ParseCredential(in.Credential)
	if err != nil {
		return nil, err
	}
	client, err := hanime1.NewClientWithUserAgent(cred.Cookies, in.ProxyURL, cred.UserAgent)
	if err != nil {
		return nil, err
	}
	return &directResolver{client: client}, nil
}

type directResolver struct {
	client *hanime1.Client
}

func (r *directResolver) Options(ctx context.Context, pageURL string) ([]sites.DirectOption, error) {
	opts, err := r.client.DownloadOptions(ctx, pageURL)
	if err != nil {
		return nil, err
	}
	out := make([]sites.DirectOption, 0, len(opts))
	for _, o := range opts {
		out = append(out, sites.DirectOption{Quality: o.Quality, Label: o.Label, URL: o.URL, Headers: o.Headers})
	}
	return out, nil
}

func (r *directResolver) Close() error { return r.client.Close() }
