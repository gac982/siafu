// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import "io"

func runMenu(in io.Reader, out, errOut io.Writer) int {
	return runMenuUI(in, out, errOut)
}
