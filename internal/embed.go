// Package internal contains embedded assets.
package internal

import (
	"embed"
)

// WebAssets holds the embedded SvelteKit build.
// The web/build directory is populated by `npm run build` in the web/ directory.
//
//go:embed all:web/build
var WebAssets embed.FS
