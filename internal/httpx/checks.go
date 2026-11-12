// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"bytes"
	"time"

	"github.com/gac982/siafu/internal/config"
)

// Check is one evaluated assertion.
type Check struct {
	Name   string
	Passed bool
	Detail string
}

func evaluateChecks(expect config.Expect, status int, duration time.Duration, body []byte) ([]Check, string) {
	checks := make([]Check, 0, 3)
	reason := ""

	statusPass := status >= 200 && status < 400
	if len(expect.Status) > 0 {
		statusPass = false
		for _, code := range expect.Status {
			if status == code {
				statusPass = true
				break
			}
		}
	}
	checks = append(checks, Check{Name: "status", Passed: statusPass})
	if !statusPass {
		reason = "status"
	}

	if expect.BodyContains != "" {
		pass := bytes.Contains(body, []byte(expect.BodyContains))
		checks = append(checks, Check{Name: "body", Passed: pass})
		if !pass && reason == "" {
			reason = "body"
		}
	}

	if expect.MaxDuration.Value() > 0 {
		pass := duration <= expect.MaxDuration.Value()
		checks = append(checks, Check{Name: "duration", Passed: pass})
		if !pass && reason == "" {
			reason = "duration"
		}
	}

	return checks, reason
}
