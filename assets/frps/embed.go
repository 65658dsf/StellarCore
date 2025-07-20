package frpc

import (
	"embed"

	"github.com/65658dsf/StellarCore/assets"
)

//go:embed static/*
var content embed.FS

func init() {
	assets.Register(content)
}
