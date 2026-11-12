// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package version exposes build metadata injected at link time.
package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the release version, overridden with -ldflags "-X ...".
	Version = "dev"
	// Commit is the short git commit, overridden at link time.
	Commit = "none"
	// Date is the build date in UTC, overridden at link time.
	Date = "unknown"
)

// String returns the user-facing version line.
func String() string {
	return fmt.Sprintf("siafu %s (%s) built %s · %s/%s", Version, Commit, Date, runtime.GOOS, runtime.GOARCH)
}
