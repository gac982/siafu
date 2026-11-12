// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"net/http"
	"net/url"
	"testing"
)

func TestLazyJarAllocatesOnSet(t *testing.T) {
	jar := newLazyJar()
	base, err := url.Parse("http://example.com/")
	if err != nil {
		t.Fatal(err)
	}

	if jar.allocated() {
		t.Fatal("jar allocated before SetCookies")
	}
	if cookies := jar.Cookies(base); cookies != nil {
		t.Fatalf("Cookies() = %v, want nil", cookies)
	}

	jar.SetCookies(base, []*http.Cookie{{Name: "session", Value: "1", Path: "/"}})
	if !jar.allocated() {
		t.Fatal("jar not allocated after SetCookies")
	}
	cookies := jar.Cookies(base)
	if len(cookies) != 1 || cookies[0].Value != "1" {
		t.Fatalf("cookies = %v, want session=1", cookies)
	}
}

func TestLazyJarIgnoresEmptySetCookies(t *testing.T) {
	jar := newLazyJar()
	base, err := url.Parse("http://example.com/")
	if err != nil {
		t.Fatal(err)
	}

	jar.SetCookies(base, nil)
	if jar.allocated() {
		t.Fatal("jar allocated for an empty SetCookies call")
	}
}

func TestLazyJarIsPerClient(t *testing.T) {
	first := newLazyJar()
	second := newLazyJar()
	base, err := url.Parse("http://example.com/")
	if err != nil {
		t.Fatal(err)
	}

	first.SetCookies(base, []*http.Cookie{{Name: "session", Value: "1", Path: "/"}})
	if cookies := second.Cookies(base); len(cookies) != 0 {
		t.Fatalf("second jar sees %v, want no cookies", cookies)
	}
}
