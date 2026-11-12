// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

// Package admission rejects configurations that exceed the process safety
// budget before any request, connection or goroutine is created.
package admission

import (
	"fmt"
	"strings"

	"github.com/gac982/siafu/internal/config"
)

const (
	baseMemoryMB        = 32
	perVUKiB            = 50
	perInFlightKiB      = 55
	absoluteMaxVUs      = 20_000
	absoluteMaxInFlight = 20_000
	absoluteMaxMB       = 1024
	defaultBudgetMB     = 450
	defaultMaxVUs       = 2_000
	defaultMaxInFlight  = 1_000
)

// Violation is one configuration limit that was exceeded.
type Violation struct {
	Field      string
	Configured int
	Maximum    int
	Unit       string
	BudgetMB   int
	Absolute   bool
}

// Error lists every admission violation.
type Error struct {
	Violations []Violation
}

// Error renders an actionable, human-readable rejection message.
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("Configuración rechazada: excede los límites de seguridad.\n")
	actions := make([]string, 0, len(e.Violations))
	for _, v := range e.Violations {
		fmt.Fprintf(&b, "\n  %s configurado: %s %s\n", v.Field, formatInt(v.Configured), v.Unit)
		if v.Absolute {
			fmt.Fprintf(&b, "  máximo absoluto permitido: %s %s\n", formatInt(v.Maximum), v.Unit)
			fmt.Fprintf(&b, "  exceso: %s %s\n", formatInt(v.Configured-v.Maximum), v.Unit)
			actions = append(actions, fmt.Sprintf("reduce %s (máximo absoluto %s %s)", v.Field, formatInt(v.Maximum), v.Unit))
		} else {
			fmt.Fprintf(&b, "  máximo permitido con limits.memory_budget_mb=%d: %s %s\n",
				v.BudgetMB, formatInt(v.Maximum), v.Unit)
			fmt.Fprintf(&b, "  exceso: %s %s\n", formatInt(v.Configured-v.Maximum), v.Unit)
			actions = append(actions, fmt.Sprintf("reduce %s o aumenta limits.memory_budget_mb (máximo %d)", v.Field, absoluteMaxMB))
		}
	}
	b.WriteString("\nAcción correctiva: ")
	b.WriteString(strings.Join(actions, "; "))
	b.WriteString(".\n")
	b.WriteString("No se ejecutó ninguna petición.")
	return b.String()
}

// Check validates the configured concurrency against the memory budget and
// the absolute safety caps.
func Check(cfg *config.Config) error {
	budget := cfg.Limits.MemoryBudgetMB
	if budget <= 0 {
		budget = defaultBudgetMB
	}

	var violations []Violation
	budgetOK := budget <= absoluteMaxMB
	if !budgetOK {
		violations = append(violations, Violation{
			Field:      "limits.memory_budget_mb",
			Configured: budget,
			Maximum:    absoluteMaxMB,
			Unit:       "MiB",
			Absolute:   true,
		})
	}

	if cfg.Load.Mode == config.ModeOpen {
		maxInFlight := cfg.Load.OpenLoop.MaxInFlight
		if maxInFlight <= 0 {
			maxInFlight = defaultMaxInFlight
		}
		if maxInFlight > absoluteMaxInFlight {
			violations = append(violations, Violation{
				Field:      "open_loop.max_in_flight",
				Configured: maxInFlight,
				Maximum:    absoluteMaxInFlight,
				Unit:       "in-flight",
				Absolute:   true,
			})
		} else if budgetOK {
			maxAllowed := MaxInFlightForBudget(budget)
			if maxInFlight > maxAllowed {
				violations = append(violations, Violation{
					Field:      "open_loop.max_in_flight",
					Configured: maxInFlight,
					Maximum:    maxAllowed,
					Unit:       "in-flight",
					BudgetMB:   budget,
				})
			}
		}
	} else {
		maxVUs := cfg.Load.MaxVUs
		if maxVUs <= 0 {
			maxVUs = defaultMaxVUs
		}
		if maxVUs > absoluteMaxVUs {
			violations = append(violations, Violation{
				Field:      "load.max_vus",
				Configured: maxVUs,
				Maximum:    absoluteMaxVUs,
				Unit:       "VUs",
				Absolute:   true,
			})
		} else if budgetOK {
			maxAllowed := MaxVUsForBudget(budget)
			if maxVUs > maxAllowed {
				violations = append(violations, Violation{
					Field:      "load.max_vus",
					Configured: maxVUs,
					Maximum:    maxAllowed,
					Unit:       "VUs",
					BudgetMB:   budget,
				})
			}
		}
	}

	if len(violations) == 0 {
		return nil
	}
	return &Error{Violations: violations}
}

// MaxVUsForBudget returns the conservative VU capacity for a memory budget.
// It reserves 32 MiB for the process and 50 KiB per VU.
func MaxVUsForBudget(budgetMB int) int {
	if budgetMB <= baseMemoryMB {
		return 0
	}
	return (budgetMB - baseMemoryMB) * 1024 / perVUKiB
}

// MaxInFlightForBudget returns the conservative open-loop capacity for a
// memory budget. It reserves 32 MiB for the process and 55 KiB per worker.
func MaxInFlightForBudget(budgetMB int) int {
	if budgetMB <= baseMemoryMB {
		return 0
	}
	return (budgetMB - baseMemoryMB) * 1024 / perInFlightKiB
}

// EstimateMB returns the conservative process estimate for the configured VUs.
func EstimateMB(maxVUs int) int {
	if maxVUs < 0 {
		maxVUs = 0
	}
	return baseMemoryMB + (maxVUs*perVUKiB+1023)/1024
}

// EstimateOpenMB returns the conservative process estimate for open-loop.
func EstimateOpenMB(maxInFlight int) int {
	if maxInFlight < 0 {
		maxInFlight = 0
	}
	return baseMemoryMB + (maxInFlight*perInFlightKiB+1023)/1024
}

func formatInt(n int) string {
	if n < 0 {
		return "-" + formatInt(-n)
	}
	raw := fmt.Sprintf("%d", n)
	if len(raw) <= 3 {
		return raw
	}
	var parts []string
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	return strings.Join(parts, " ")
}
