// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"fmt"
	"io"

	"github.com/gac982/siafu/internal/admission"
	"github.com/gac982/siafu/internal/config"
)

func runValidate(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "uso: siafu validate <archivo.yaml>")
		return 2
	}

	cfg, err := validateFile(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	fmt.Fprintf(stdout, "OK  %s", cfg.Summary())
	return 0
}

func validateFile(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	if err := admission.Check(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
