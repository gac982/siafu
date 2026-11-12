// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package vars

import "fmt"

// Scope holds the values available while rendering one request.
type Scope struct {
	VU        int
	Iter      int
	Extracted map[string]string
	Globals   map[string]Template
	LookupEnv func(string) (string, bool)
}

// Resolve returns an extracted variable or renders a global template.
// Global templates are rendered without access to other globals, which
// keeps the schema free of variable-to-variable cycles.
func (s Scope) Resolve(name string) (string, error) {
	if value, ok := s.Extracted[name]; ok {
		return value, nil
	}
	if tpl, ok := s.Globals[name]; ok {
		nested := Scope{VU: s.VU, Iter: s.Iter, LookupEnv: s.LookupEnv}
		return tpl.Render(nested)
	}
	return "", fmt.Errorf("variable %q no definida", name)
}
