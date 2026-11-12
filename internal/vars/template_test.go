// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package vars

import (
	"strings"
	"sync"
	"testing"
)

func mustCompile(t *testing.T, s string) Template {
	t.Helper()
	tpl, err := Compile(s)
	if err != nil {
		t.Fatalf("Compile(%q) error = %v", s, err)
	}
	return tpl
}

func TestRenderPlainAndEscaped(t *testing.T) {
	cases := map[string]string{
		"hola":      "hola",
		"$${x}":     "${x}",
		"a $5 b":    "a $5 b",
		"pre ${v}":  "pre x",
		"${v}-${v}": "x-x",
	}
	for input, want := range cases {
		scope := Scope{Extracted: map[string]string{"v": "x"}}
		got, err := mustCompile(t, input).Render(scope)
		if err != nil {
			t.Fatalf("Render(%q) error = %v", input, err)
		}
		if got != want {
			t.Errorf("Render(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRenderBuiltins(t *testing.T) {
	got, err := mustCompile(t, "${VU}:${ITER}").Render(Scope{VU: 3, Iter: 7})
	if err != nil {
		t.Fatal(err)
	}
	if got != "3:7" {
		t.Errorf("got %q, want 3:7", got)
	}
}

func TestRenderGlobalVarUsesIterAndEnv(t *testing.T) {
	scope := Scope{
		VU:      3,
		Iter:    7,
		Globals: map[string]Template{"who": mustCompile(t, "vu-${VU}-${ENV:SUFFIX}")},
		LookupEnv: func(k string) (string, bool) {
			if k == "SUFFIX" {
				return "x", true
			}
			return "", false
		},
	}
	got, err := mustCompile(t, "hola ${who}").Render(scope)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hola vu-3-x" {
		t.Errorf("got %q", got)
	}
}

func TestRenderEnvDefault(t *testing.T) {
	got, err := mustCompile(t, "${ENV:MISSING:-fallback}").Render(Scope{
		LookupEnv: func(string) (string, bool) { return "", false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "fallback" {
		t.Errorf("got %q, want fallback", got)
	}
}

func TestRenderMissingEnv(t *testing.T) {
	_, err := mustCompile(t, "${ENV:MISSING}").Render(Scope{
		LookupEnv: func(string) (string, bool) { return "", false },
	})
	if err == nil || !strings.Contains(err.Error(), "MISSING") {
		t.Fatalf("error = %v, want missing env error", err)
	}
}

func TestRenderMissingVar(t *testing.T) {
	_, err := mustCompile(t, "${nope}").Render(Scope{})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("error = %v, want missing variable error", err)
	}
}

func TestGlobalCannotReferenceGlobal(t *testing.T) {
	scope := Scope{
		Globals: map[string]Template{
			"a": mustCompile(t, "${b}"),
			"b": mustCompile(t, "x"),
		},
	}
	_, err := mustCompile(t, "${a}").Render(scope)
	if err == nil || !strings.Contains(err.Error(), "b") {
		t.Fatalf("error = %v, want global-to-global reference error", err)
	}
}

func TestCompileErrors(t *testing.T) {
	for _, in := range []string{"${", "${}", "${ENV:}", "${bad name}", "${ENV:BAD NAME}"} {
		if _, err := Compile(in); err == nil {
			t.Errorf("Compile(%q) error = nil, want error", in)
		}
	}
}

func TestRefs(t *testing.T) {
	tpl := mustCompile(t, "a ${x} b ${ENV:Y:-z} ${VU} ${x}")
	refs := tpl.Refs()
	if len(refs) != 4 {
		t.Fatalf("refs = %+v, want 4", refs)
	}
	if refs[0].Kind != KindVar || refs[0].Name != "x" {
		t.Errorf("refs[0] = %+v", refs[0])
	}
	if refs[1].Kind != KindEnv || refs[1].Name != "Y" || !refs[1].HasDefault || refs[1].Default != "z" {
		t.Errorf("refs[1] = %+v", refs[1])
	}
	if refs[2].Kind != KindBuiltin || refs[2].Name != "VU" {
		t.Errorf("refs[2] = %+v", refs[2])
	}
}

func TestConcurrentRender(t *testing.T) {
	tpl := mustCompile(t, "vu-${VU}-${ITER}")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got, err := tpl.Render(Scope{VU: i, Iter: i + 1})
			if err != nil || got == "" {
				t.Errorf("render error = %v", err)
			}
		}(i)
	}
	wg.Wait()
}
