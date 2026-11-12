// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
)

// lazyJar implements http.CookieJar but only allocates the underlying
// cookiejar.Jar when the target sets a cookie. Most load tests never use
// cookies, so this removes per-VU state without changing session semantics.
type lazyJar struct {
	mu  sync.Mutex
	jar *cookiejar.Jar
}

func newLazyJar() *lazyJar { return &lazyJar{} }

func (j *lazyJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if len(cookies) == 0 {
		return
	}

	j.mu.Lock()
	if j.jar == nil {
		j.jar, _ = cookiejar.New(nil)
	}
	jar := j.jar
	j.mu.Unlock()

	if jar != nil {
		jar.SetCookies(u, cookies)
	}
}

func (j *lazyJar) Cookies(u *url.URL) []*http.Cookie {
	j.mu.Lock()
	jar := j.jar
	j.mu.Unlock()
	if jar == nil {
		return nil
	}
	return jar.Cookies(u)
}

func (j *lazyJar) allocated() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.jar != nil
}
