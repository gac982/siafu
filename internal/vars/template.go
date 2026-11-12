// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package vars compiles and renders ${...} templates used by the test schema.
package vars

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Kind classifies one template reference.
type Kind int

const (
	// KindVar is a scenario/global variable.
	KindVar Kind = iota
	// KindEnv is an environment variable.
	KindEnv
	// KindBuiltin is VU or ITER.
	KindBuiltin
)

// Ref is one ${...} reference found in a template.
type Ref struct {
	Kind       Kind
	Name       string
	Default    string
	HasDefault bool
}

// Template is a compiled string with literal and reference parts.
type Template struct {
	raw   string
	parts []part
}

type part struct {
	text string
	ref  *Ref
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Compile parses ${NAME}, ${ENV:NAME}, ${ENV:NAME:-default}, ${VU}, ${ITER}
// and the escape $${. Everything else is literal.
func Compile(s string) (Template, error) {
	t := Template{raw: s}
	var literal strings.Builder

	flush := func() {
		if literal.Len() > 0 {
			t.parts = append(t.parts, part{text: literal.String()})
			literal.Reset()
		}
	}

	for i := 0; i < len(s); {
		if s[i] == '$' && i+2 < len(s) && s[i+1] == '$' && s[i+2] == '{' {
			literal.WriteString("${")
			i += 3
			continue
		}
		if s[i] != '$' || i+1 >= len(s) || s[i+1] != '{' {
			literal.WriteByte(s[i])
			i++
			continue
		}

		end := strings.IndexByte(s[i+2:], '}')
		if end < 0 {
			return Template{}, fmt.Errorf("plantilla sin cerrar en el byte %d: falta '}'", i)
		}
		content := s[i+2 : i+2+end]
		ref, err := parseRef(content)
		if err != nil {
			return Template{}, fmt.Errorf("%w (byte %d)", err, i)
		}
		flush()
		t.parts = append(t.parts, part{ref: &ref})
		i += 2 + end + 1
	}

	flush()
	return t, nil
}

func parseRef(content string) (Ref, error) {
	if content == "" {
		return Ref{}, fmt.Errorf("referencia de plantilla vacía")
	}

	if strings.HasPrefix(content, "ENV:") {
		rest := strings.TrimPrefix(content, "ENV:")
		ref := Ref{Kind: KindEnv}
		if idx := strings.Index(rest, ":-"); idx >= 0 {
			ref.Name = rest[:idx]
			ref.Default = rest[idx+2:]
			ref.HasDefault = true
		} else {
			ref.Name = rest
		}
		if !identifier.MatchString(ref.Name) {
			return Ref{}, fmt.Errorf("nombre de variable de entorno inválido %q", ref.Name)
		}
		return ref, nil
	}

	if content == "VU" || content == "ITER" {
		return Ref{Kind: KindBuiltin, Name: content}, nil
	}

	if !identifier.MatchString(content) {
		return Ref{}, fmt.Errorf("nombre de variable inválido %q", content)
	}
	return Ref{Kind: KindVar, Name: content}, nil
}

// Render expands the template using scope.
func (t Template) Render(scope Scope) (string, error) {
	var b strings.Builder
	for _, p := range t.parts {
		if p.ref == nil {
			b.WriteString(p.text)
			continue
		}
		value, err := p.ref.resolve(scope)
		if err != nil {
			return "", err
		}
		b.WriteString(value)
	}
	return b.String(), nil
}

func (r Ref) resolve(scope Scope) (string, error) {
	switch r.Kind {
	case KindBuiltin:
		switch r.Name {
		case "VU":
			return strconv.Itoa(scope.VU), nil
		case "ITER":
			return strconv.Itoa(scope.Iter), nil
		default:
			return "", fmt.Errorf("builtin desconocido %q", r.Name)
		}
	case KindEnv:
		lookup := scope.LookupEnv
		if lookup == nil {
			lookup = os.LookupEnv
		}
		if value, ok := lookup(r.Name); ok {
			return value, nil
		}
		if r.HasDefault {
			return r.Default, nil
		}
		return "", fmt.Errorf("variable de entorno %q no definida", r.Name)
	case KindVar:
		return scope.Resolve(r.Name)
	default:
		return "", fmt.Errorf("tipo de referencia desconocido")
	}
}

// Refs returns every reference in order of appearance.
func (t Template) Refs() []Ref {
	refs := make([]Ref, 0, len(t.parts))
	for _, p := range t.parts {
		if p.ref != nil {
			refs = append(refs, *p.ref)
		}
	}
	return refs
}

// Raw returns the original, uncompiled string.
func (t Template) Raw() string { return t.raw }
