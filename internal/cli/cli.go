// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package cli implements the siafu command line interface.
package cli

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/gac982/siafu/internal/tui"
	"github.com/gac982/siafu/internal/version"
)

const usage = `uso: siafu <comando> [opciones]

Comandos:
  run <archivo.yaml>       ejecuta una prueba de carga
  validate <archivo.yaml>  valida la configuración sin ejecutar
  runs <subcomando>        historial de corridas (list, show, compare, prune, reindex)
  serve                    sirve la interfaz web local de resultados
  menu                     abre el menú interactivo
  version                  muestra la versión y la plataforma

Opciones:
  -h, --help               muestra esta ayuda
`

// Run dispatches one CLI invocation and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	isTTY := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	return runWithIO(args, os.Stdin, stdout, stderr, isTTY)
}

func runWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer, isTTY bool) int {
	if len(args) == 0 {
		if isTTY {
			return runMenu(stdin, stdout, stderr)
		}
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "menu":
		if !isTTY {
			fmt.Fprintln(stderr, "el menú necesita una terminal interactiva")
			return 2
		}
		return runMenu(stdin, stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, tui.Logo(isTTY && tui.ColorEnabled()))
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, version.String())
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "run":
		return runRun(args[1:], stdout, stderr)
	case "runs":
		return runRuns(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "comando desconocido: %s\n\n", args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}
