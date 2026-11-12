// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package tui

import (
	"math"
	"strings"
)

var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// RenderSparkline draws the last width values with Unicode blocks.
func RenderSparkline(values []float64, width int) string {
	if width <= 0 || len(values) == 0 {
		return ""
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}

	min, max := values[0], values[0]
	for _, value := range values {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}

	var b strings.Builder
	for i := 0; i < width-len(values); i++ {
		b.WriteRune(sparkBlocks[0])
	}
	for _, value := range values {
		index := 0
		if max > min {
			index = int(math.Round((value - min) / (max - min) * float64(len(sparkBlocks)-1)))
		} else {
			index = len(sparkBlocks) / 2
		}
		if index < 0 {
			index = 0
		}
		if index >= len(sparkBlocks) {
			index = len(sparkBlocks) - 1
		}
		b.WriteRune(sparkBlocks[index])
	}
	return b.String()
}
