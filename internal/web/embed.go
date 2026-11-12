// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package web

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var assets embed.FS

// AssetsFS returns the embedded UI files rooted at the assets directory.
func AssetsFS() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	return sub
}
