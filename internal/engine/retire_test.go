// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package engine

import "testing"

func TestRetireSurplusIgnoresRetiringVUs(t *testing.T) {
	active := make(map[int]*vuHandle)
	for i := 0; i < 18; i++ {
		active[i] = &vuHandle{retire: make(chan struct{})}
	}
	for i := 18; i < 20; i++ {
		handle := &vuHandle{retire: make(chan struct{}), retiring: true}
		close(handle.retire)
		active[i] = handle
	}

	retireSurplus(active, 19)

	closed := 0
	for _, handle := range active {
		select {
		case <-handle.retire:
			closed++
		default:
		}
	}
	if closed != 2 {
		t.Fatalf("closed %d retire channels; want 2 (the ones already retiring)", closed)
	}
}
