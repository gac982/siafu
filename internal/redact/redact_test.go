// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package redact

import (
	"net/url"
	"strings"
	"testing"
)

func TestSensitiveHeader(t *testing.T) {
	for _, name := range []string{"Authorization", "authorization", "Cookie", "Set-Cookie", "X-Api-Key", "X-Auth-Token"} {
		if !SensitiveHeader(name) {
			t.Errorf("SensitiveHeader(%q) = false", name)
		}
	}
	for _, name := range []string{"Accept", "Content-Type", "User-Agent"} {
		if SensitiveHeader(name) {
			t.Errorf("SensitiveHeader(%q) = true", name)
		}
	}
}

func TestURLQueryMasking(t *testing.T) {
	r := New([]string{"supersecret"})
	got := r.URL("https://x/api?token=abc&ok=1&api_key=zzz&password=pw")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("URL output is not a URL: %q", got)
	}
	q := u.Query()
	for _, key := range []string{"token", "api_key", "password"} {
		if q.Get(key) != "***" {
			t.Errorf("%s = %q, want ***", key, q.Get(key))
		}
	}
	if q.Get("ok") != "1" {
		t.Errorf("ok = %q, want 1", q.Get("ok"))
	}
	if strings.Contains(got, "abc") || strings.Contains(got, "zzz") || strings.Contains(got, "pw") {
		t.Errorf("URL leaks a secret: %q", got)
	}
}

func TestURLUserinfoMasking(t *testing.T) {
	r := New(nil)
	got := r.URL("https://user:secret@example.com/x")
	if strings.Contains(got, "secret") {
		t.Errorf("URL leaks password: %q", got)
	}
}

func TestSecretValueReplacement(t *testing.T) {
	r := New([]string{"supersecret", "abcd"})
	got := r.Message("token=supersecret pw=abcd short=xy ok=1")
	if strings.Contains(got, "supersecret") || strings.Contains(got, "abcd") {
		t.Errorf("Message leaks a secret: %q", got)
	}
	if !strings.Contains(got, "short=xy") || !strings.Contains(got, "ok=1") {
		t.Errorf("Message removed non-secret text: %q", got)
	}
}

func TestScrubFreeText(t *testing.T) {
	r := New(nil)
	got := r.Scrub(`Get "https://user:pass@example.com/a?token=hardcoded&ok=1": boom`)
	if strings.Contains(got, "hardcoded") || strings.Contains(got, "pass@") {
		t.Errorf("Scrub leaks secrets: %q", got)
	}
	if !strings.Contains(got, "ok=1") {
		t.Errorf("Scrub removed non-secret text: %q", got)
	}
}

func TestNewExplicitRedactsShortSecret(t *testing.T) {
	r := NewExplicit([]string{"xy"})
	got := r.Message("a=xy")
	if strings.Contains(got, "xy") {
		t.Errorf("secreto corto no redactado: %q", got)
	}
	if !strings.Contains(got, "***") {
		t.Errorf("sin máscara: %q", got)
	}
}

func TestNewStillIgnoresShortGeneric(t *testing.T) {
	r := New([]string{"xy"})
	if got := r.Message("a=xy"); got != "a=xy" {
		t.Errorf("New cambió el comportamiento: %q", got)
	}
}
