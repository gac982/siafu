// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/gac982/siafu/internal/orchestrator"
	"github.com/gac982/siafu/internal/results"

	"github.com/gac982/siafu/internal/safetext"
)

type comparison struct {
	A     results.IndexEntry `json:"a"`
	B     results.IndexEntry `json:"b"`
	Delta comparisonDelta    `json:"delta"`
}

type comparisonDelta struct {
	P95MS          float64 `json:"p95_ms"`
	RPSAvg         float64 `json:"rps_avg"`
	ErrorRate      float64 `json:"error_rate"`
	Requests       int64   `json:"requests"`
	VerdictChanged bool    `json:"verdict_changed"`
}

func runRuns(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, runsUsage)
		return 2
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, runsUsage)
		return 0
	case "list":
		return runRunsList(args[1:], stdout, stderr)
	case "show":
		return runRunsShow(args[1:], stdout, stderr)
	case "compare":
		return runRunsCompare(args[1:], stdout, stderr)
	case "prune":
		return runRunsPrune(args[1:], stdout, stderr)
	case "reindex":
		return runRunsReindex(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "subcomando desconocido: %s\n\n", args[0])
		fmt.Fprint(stderr, runsUsage)
		return 2
	}
}

const runsUsage = `uso: siafu runs <subcomando> [opciones]

Subcomandos:
  list                    lista las corridas guardadas
  show <id>               muestra el reporte de una corrida
  compare <a> <b>         compara dos corridas
  prune --keep N          conserva las N corridas más nuevas
  reindex                 reconstruye index.jsonl

Opciones comunes:
  --results-dir DIR       directorio de resultados
  --json                  salida JSON (list, show, compare)
  -h, --help              muestra esta ayuda
`

var runsValueFlags = map[string]bool{
	"--results-dir": true, "-results-dir": true,
	"--limit": true, "-limit": true,
	"--mode": true, "-mode": true,
	"--verdict": true, "-verdict": true,
	"--q": true, "-q": true,
	"--keep": true, "-keep": true,
}

func newRunsFlags(name string, stderr io.Writer) (*flag.FlagSet, *string, *bool) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("results-dir", "", "directorio de resultados")
	asJSON := flags.Bool("json", false, "salida JSON")
	return flags, dir, asJSON
}

func parseRunsFlags(flags *flag.FlagSet, args []string) int {
	if err := flags.Parse(reorderFlags(args, runsValueFlags)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	return -1
}

func openRunsStore(dir string, stderr io.Writer) (*results.Store, error) {
	resolved := orchestrator.ResolveResultsDir(orchestrator.Options{ResultsDir: dir})
	store, err := results.OpenStore(resolved)
	if err != nil {
		return nil, err
	}
	store.Warn = func(msg string) { fmt.Fprintf(stderr, "aviso: %s\n", msg) }
	return store, nil
}

func openWritableStore(dir string, stderr io.Writer) (*results.Store, error) {
	resolved := orchestrator.ResolveResultsDir(orchestrator.Options{ResultsDir: dir})
	store, err := results.NewStore(resolved)
	if err != nil {
		return nil, err
	}
	store.Warn = func(msg string) { fmt.Fprintf(stderr, "aviso: %s\n", msg) }
	return store, nil
}

func runRunsList(args []string, stdout, stderr io.Writer) int {
	flags, dir, asJSON := newRunsFlags("runs list", stderr)
	limit := flags.Int("limit", 0, "número máximo de corridas")
	mode := flags.String("mode", "", "filtrar por modo")
	verdict := flags.String("verdict", "", "filtrar por veredicto")
	query := flags.String("q", "", "buscar en nombre, tag o id")
	if code := parseRunsFlags(flags, args); code >= 0 {
		return code
	}
	if flags.NArg() != 0 {
		fmt.Fprint(stderr, "uso: siafu runs list [--json] [--limit N] [--mode M] [--verdict V] [--q TEXTO] [--results-dir DIR]\n")
		return 2
	}
	if *limit < 0 {
		fmt.Fprint(stderr, "limit debe ser mayor o igual que 0\n")
		return 2
	}
	store, err := openRunsStore(*dir, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	entries, err := store.List()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	filtered := filterEntries(entries, *mode, *verdict, *query, *limit)
	if *asJSON {
		if filtered == nil {
			filtered = []results.IndexEntry{}
		}
		return writeJSON(stdout, filtered, stderr)
	}
	printRunsList(stdout, filtered)
	return 0
}

func runRunsShow(args []string, stdout, stderr io.Writer) int {
	flags, dir, asJSON := newRunsFlags("runs show", stderr)
	if code := parseRunsFlags(flags, args); code >= 0 {
		return code
	}
	if flags.NArg() != 1 {
		fmt.Fprint(stderr, "uso: siafu runs show <id> [--json] [--results-dir DIR]\n")
		return 2
	}
	id := flags.Arg(0)
	store, err := openRunsStore(*dir, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *asJSON {
		record, code := loadRun(store, id, stderr)
		if code != 0 {
			return code
		}
		return writeJSON(stdout, record, stderr)
	}
	summary, err := store.Summary(id)
	if err == nil {
		summary = safetext.Multiline(summary)
		fmt.Fprint(stdout, summary)
		if !strings.HasSuffix(summary, "\n") {
			fmt.Fprintln(stdout)
		}
		return 0
	}
	record, code := loadRun(store, id, stderr)
	if code != 0 {
		return code
	}
	fmt.Fprintf(stdout, "%s · %s\n", safetext.Line(record.Name), record.RunID)
	fmt.Fprintf(stdout, "Veredicto: %s · modo %s\n", record.Verdict, record.Mode)
	fmt.Fprintf(stdout, "p95 %.1fms · RPS %.1f · peticiones %d · errores %.2f%%\n",
		record.Global.P95MS, record.Global.RPSAvg, record.Global.Requests, record.Global.ErrorRate*100)
	return 0
}

func runRunsCompare(args []string, stdout, stderr io.Writer) int {
	flags, dir, asJSON := newRunsFlags("runs compare", stderr)
	if code := parseRunsFlags(flags, args); code >= 0 {
		return code
	}
	if flags.NArg() != 2 {
		fmt.Fprint(stderr, "uso: siafu runs compare <a> <b> [--json] [--results-dir DIR]\n")
		return 2
	}
	store, err := openRunsStore(*dir, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	first, code := loadRun(store, flags.Arg(0), stderr)
	if code != 0 {
		return code
	}
	second, code := loadRun(store, flags.Arg(1), stderr)
	if code != 0 {
		return code
	}
	cmp := compareRuns(first, second)
	if *asJSON {
		return writeJSON(stdout, cmp, stderr)
	}
	printComparison(stdout, cmp)
	return 0
}

func runRunsPrune(args []string, stdout, stderr io.Writer) int {
	flags, dir, _ := newRunsFlags("runs prune", stderr)
	keep := flags.Int("keep", -1, "número de corridas a conservar")
	dryRun := flags.Bool("dry-run", false, "mostrar qué se eliminaría sin borrar")
	if code := parseRunsFlags(flags, args); code >= 0 {
		return code
	}
	if flags.NArg() != 0 || *keep < 0 {
		fmt.Fprint(stderr, "uso: siafu runs prune --keep N [--dry-run] [--results-dir DIR]\n")
		return 2
	}
	store, err := openRunsStore(*dir, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *dryRun {
		entries, err := store.List()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if *keep < len(entries) {
			entries = entries[*keep:]
		} else {
			entries = nil
		}
		if len(entries) == 0 {
			fmt.Fprintln(stdout, "no hay corridas para eliminar")
			return 0
		}
		fmt.Fprintf(stdout, "se eliminaría %d corrida(s):\n", len(entries))
		for _, entry := range entries {
			fmt.Fprintf(stdout, "  %s  %s\n", entry.RunID, entry.Name)
		}
		return 0
	}
	removed, err := store.Prune(*keep)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "eliminada(s) %d corrida(s)\n", removed)
	return 0
}

func runRunsReindex(args []string, stdout, stderr io.Writer) int {
	flags, dir, _ := newRunsFlags("runs reindex", stderr)
	if code := parseRunsFlags(flags, args); code >= 0 {
		return code
	}
	if flags.NArg() != 0 {
		fmt.Fprint(stderr, "uso: siafu runs reindex [--results-dir DIR]\n")
		return 2
	}
	store, err := openWritableStore(*dir, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := store.Reindex(); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintln(stdout, "índice reconstruido")
	return 0
}

func compareRuns(first, second results.RunRecord) comparison {
	a := first.IndexEntry()
	b := second.IndexEntry()
	return comparison{
		A: a,
		B: b,
		Delta: comparisonDelta{
			P95MS:          b.P95MS - a.P95MS,
			RPSAvg:         b.RPSAvg - a.RPSAvg,
			ErrorRate:      b.ErrorRate - a.ErrorRate,
			Requests:       b.Requests - a.Requests,
			VerdictChanged: a.Verdict != b.Verdict,
		},
	}
}

func printRunsList(w io.Writer, entries []results.IndexEntry) {
	fmt.Fprintf(w, "%-40s  %-19s  %-7s  %-14s  %-9s  %-8s  %-10s  %s\n",
		"ID", "FECHA", "MODO", "VEREDICTO", "P95", "ERRORES", "RPS", "NOMBRE")
	for _, entry := range entries {
		fmt.Fprintf(w, "%-40s  %-19s  %-7s  %-14s  %-9s  %-8s  %-10.1f  %s\n",
			entry.RunID,
			entry.CreatedAt.Local().Format("2006-01-02 15:04:05"),
			entry.Mode,
			entry.Verdict,
			formatMS(entry.P95MS),
			formatPercent(entry.ErrorRate),
			entry.RPSAvg,
			safetext.Line(entry.Name),
		)
	}
}

func printComparison(w io.Writer, cmp comparison) {
	fmt.Fprintf(w, "A: %s · %s (%s)\n", cmp.A.RunID, safetext.Line(cmp.A.Name), cmp.A.Verdict)
	fmt.Fprintf(w, "B: %s · %s (%s)\n\n", cmp.B.RunID, safetext.Line(cmp.B.Name), cmp.B.Verdict)
	fmt.Fprintf(w, "%-12s %-12s %-12s %s\n", "MÉTRICA", "A", "B", "DELTA")
	fmt.Fprintf(w, "%-12s %-12s %-12s %s\n", "p95", formatMS(cmp.A.P95MS), formatMS(cmp.B.P95MS), signedMS(cmp.Delta.P95MS))
	fmt.Fprintf(w, "%-12s %-12.1f %-12.1f %s\n", "RPS", cmp.A.RPSAvg, cmp.B.RPSAvg, signedFloat(cmp.Delta.RPSAvg))
	fmt.Fprintf(w, "%-12s %-12s %-12s %s\n", "error rate", formatPercent(cmp.A.ErrorRate), formatPercent(cmp.B.ErrorRate), signedPercent(cmp.Delta.ErrorRate))
	fmt.Fprintf(w, "%-12s %-12d %-12d %+d\n", "requests", cmp.A.Requests, cmp.B.Requests, cmp.Delta.Requests)
}

func writeJSON(w io.Writer, value any, stderr io.Writer) int {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func filterEntries(entries []results.IndexEntry, mode, verdict, query string, limit int) []results.IndexEntry {
	filtered := make([]results.IndexEntry, 0, len(entries))
	for _, entry := range entries {
		if mode != "" && !strings.EqualFold(entry.Mode, mode) {
			continue
		}
		if verdict != "" && !strings.EqualFold(entry.Verdict, verdict) {
			continue
		}
		if query != "" && !containsFold(entry.Name+" "+entry.Tag+" "+entry.RunID, query) {
			continue
		}
		filtered = append(filtered, entry)
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func formatMS(ms float64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.2fs", ms/1000)
	}
	return fmt.Sprintf("%.1fms", ms)
}

func formatPercent(rate float64) string {
	return fmt.Sprintf("%.2f%%", rate*100)
}

func signedMS(ms float64) string {
	if ms >= 0 {
		return "+" + formatMS(ms)
	}
	return formatMS(ms)
}

func signedFloat(value float64) string {
	if value >= 0 {
		return fmt.Sprintf("+%.1f", value)
	}
	return fmt.Sprintf("%.1f", value)
}

func signedPercent(rate float64) string {
	if rate >= 0 {
		return fmt.Sprintf("+%.2fpp", rate*100)
	}
	return fmt.Sprintf("%.2fpp", rate*100)
}

func loadRun(store *results.Store, id string, stderr io.Writer) (results.RunRecord, int) {
	record, err := store.Load(id)
	if err == nil {
		return record, 0
	}
	if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
		fmt.Fprintf(stderr, "no existe la corrida %s\n", id)
	} else {
		fmt.Fprintln(stderr, err)
	}
	return results.RunRecord{}, 2
}
