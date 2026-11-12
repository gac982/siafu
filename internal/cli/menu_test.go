// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunWithIOWithoutTTY(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runWithIO(nil, strings.NewReader(""), &out, &errOut, false); code != 2 {
		t.Fatalf("code = %d; quiero 2", code)
	}
	if !strings.Contains(errOut.String(), "uso: siafu") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestRunWithIOMenuWithoutTTY(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runWithIO([]string{"menu"}, strings.NewReader(""), &out, &errOut, false); code != 2 {
		t.Fatalf("code = %d; quiero 2", code)
	}
	if !strings.Contains(errOut.String(), "terminal") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestRunWithIOMenuTTY(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runWithIO([]string{"menu"}, strings.NewReader("q\n"), &out, &errOut, true); code != 0 {
		t.Fatalf("code = %d; quiero 0", code)
	}
}

func TestRunWithIONoArgsTTY(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runWithIO(nil, strings.NewReader("q\n"), &out, &errOut, true); code != 0 {
		t.Fatalf("code = %d; quiero 0", code)
	}
}
