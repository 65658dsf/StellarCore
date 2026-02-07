package frps

import (
	"embed"

	"github.com/65658dsf/StellarCore/assets"
)

//go:embed public
var EmbedFS embed.FS

func init() {
	assets.Register(EmbedFS)
}
