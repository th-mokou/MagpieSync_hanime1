//go:build !no_hanime1 && (!explicit_sites || site_hanime1)

package hanime1

import (
	_ "embed"

	"magpiesync/internal/sites"
)

//go:embed assets/logo.png
var logo []byte

func init() {
	sites.RegisterAsset("hanime1", "logo.png", logo)
}
