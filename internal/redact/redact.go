// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package redact masks credentials before they reach logs or reports.
package redact

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
	"x-auth-token":        true,
}

var sensitiveQueryParts = []string{
	"token", "key", "secret", "password", "passwd", "sig", "signature", "auth", "session",
}

// SensitiveHeader reports whether a header value must never be logged.
func SensitiveHeader(name string) bool {
	return sensitiveHeaders[strings.ToLower(strings.TrimSpace(name))]
}

// SensitiveQueryKey reports whether a query parameter value must be masked.
func SensitiveQueryKey(name string) bool {
	lower := strings.ToLower(name)
	for _, part := range sensitiveQueryParts {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return false
}

// Redactor replaces known secret values and masks sensitive URL parts.
type Redactor struct {
	secrets []string
}

// New builds a redactor for the given secret values. Values shorter than
// four bytes are ignored to avoid destroying unrelated text.
func New(secrets []string) *Redactor {
	return newRedactor(secrets, 4)
}

// NewExplicit builds a redactor for explicitly provided secrets. Unlike New,
// it redacts values of any non-zero length; use it for environment or
// configuration secrets where every value is known to be sensitive.
func NewExplicit(secrets []string) *Redactor {
	return newRedactor(secrets, 1)
}

func newRedactor(secrets []string, minLen int) *Redactor {
	filtered := make([]string, 0, len(secrets))
	seen := map[string]bool{}
	for _, s := range secrets {
		if len(s) < minLen || seen[s] {
			continue
		}
		seen[s] = true
		filtered = append(filtered, s)
	}
	sort.Slice(filtered, func(i, j int) bool { return len(filtered[i]) > len(filtered[j]) })
	return &Redactor{secrets: filtered}
}

// Message replaces every known secret value with ***.
func (r *Redactor) Message(s string) string {
	if r == nil {
		return s
	}
	for _, secret := range r.secrets {
		s = strings.ReplaceAll(s, secret, "***")
	}
	return s
}

var urlPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

// Scrub masks every URL found in free text, then replaces known secrets.
func (r *Redactor) Scrub(s string) string {
	s = urlPattern.ReplaceAllStringFunc(s, r.urlOnly)
	return r.Message(s)
}

// URL masks sensitive query values and userinfo passwords, then applies
// message redaction to the result.
func (r *Redactor) URL(raw string) string {
	return r.Message(r.urlOnly(raw))
}

func (r *Redactor) urlOnly(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "***")
		}
	}

	if u.RawQuery != "" {
		values := u.Query()
		for key := range values {
			if SensitiveQueryKey(key) {
				values.Set(key, "***")
			}
		}
		u.RawQuery = values.Encode()
	}

	return u.String()
}
