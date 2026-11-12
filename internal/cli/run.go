// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/gac982/siafu/internal/admission"
	"github.com/gac982/siafu/internal/config"
	"github.com/gac982/siafu/internal/orchestrator"
	"github.com/gac982/siafu/internal/report"
	"github.com/gac982/siafu/internal/version"
)

func runRun(args []string, stdout, stderr io.Writer) int {
	_, code := runRunReport(args, stdout, stderr)
	return code
}

func runRunReport(args []string, stdout, stderr io.Writer) (report.Report, int) {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	noTUI := flags.Bool("no-tui", false, "usa salida de texto en vez de la TUI")
	seed := flags.Int64("seed", 0, "semilla reproducible (sobrescribe el YAML)")
	debug := flags.Bool("debug", false, "activa logs de depuración")
	resultsDir := flags.String("results-dir", "", "directorio de resultados")
	noSave := flags.Bool("no-save", false, "no guardar los resultados de la corrida")
	tag := flags.String("tag", "", "etiqueta libre para la corrida")
	if err := flags.Parse(reorderRunArgs(args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return report.Report{}, 0
		}
		return report.Report{}, 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "uso: siafu run <archivo.yaml> [--no-tui] [--seed N] [--debug] [--results-dir DIR] [--no-save] [--tag TAG]")
		return report.Report{}, 2
	}

	cfg, err := config.Load(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return report.Report{}, 2
	}
	if err := admission.Check(cfg); err != nil {
		fmt.Fprintln(stderr, err)
		return report.Report{}, 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rep, code := orchestrator.Run(ctx, cfg, orchestrator.Options{
		NoTUI:      *noTUI,
		Debug:      *debug,
		Seed:       *seed,
		Stdout:     stdout,
		Stderr:     stderr,
		IsTTY:      term.IsTerminal(int(os.Stdout.Fd())),
		ResultsDir: *resultsDir,
		NoSave:     *noSave,
		Tag:        *tag,
		ConfigPath: flags.Arg(0),
		Version:    version.Version,
		Commit:     version.Commit,
	})
	return rep, code
}

// reorderRunArgs moves the positional YAML file to the end so Go's flag
// parser accepts flags both before and after it.
func reorderRunArgs(args []string) []string {
	return reorderFlags(args, map[string]bool{
		"--seed": true, "-seed": true,
		"--results-dir": true, "-results-dir": true,
		"--tag": true, "-tag": true,
	})
}

// reorderFlags moves positional arguments to the end so flags can appear on
// either side. valueFlags lists flags that consume the following argument.
func reorderFlags(args []string, valueFlags map[string]bool) []string {
	flags := make([]string, 0, len(args))
	positional := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case valueFlags[arg]:
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		case strings.HasPrefix(arg, "-"):
			flags = append(flags, arg)
		default:
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
}
