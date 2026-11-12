// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package admission

import (
	"strings"
	"testing"

	"github.com/gac982/siafu/internal/config"
)

func configWith(maxVUs, budgetMB int) *config.Config {
	return &config.Config{
		Load:   config.LoadConfig{MaxVUs: maxVUs},
		Limits: config.Limits{MemoryBudgetMB: budgetMB, WarnAtPercent: 80},
	}
}

func TestMaxVUsForBudget(t *testing.T) {
	if got := MaxVUsForBudget(450); got != 8560 {
		t.Errorf("MaxVUsForBudget(450) = %d, want 8560", got)
	}
	if got := MaxVUsForBudget(300); got != 5488 {
		t.Errorf("MaxVUsForBudget(300) = %d, want 5488", got)
	}
	if got := MaxVUsForBudget(800); got != 15728 {
		t.Errorf("MaxVUsForBudget(800) = %d, want 15728", got)
	}
}

func TestCheckAllows15000At800(t *testing.T) {
	if err := Check(configWith(15000, 800)); err != nil {
		t.Fatalf("Check() = %v, want nil", err)
	}
}

func TestCheckRejects15000At450(t *testing.T) {
	err := Check(configWith(15000, 450))
	if err == nil {
		t.Fatal("Check() = nil, want rejection")
	}
	text := err.Error()
	for _, want := range []string{"15 000", "8 560", "6 440", "Acción correctiva", "No se ejecutó ninguna petición"} {
		if !strings.Contains(text, want) {
			t.Errorf("error = %q, want %q", text, want)
		}
	}
}

func TestCheckRejects18000At800(t *testing.T) {
	err := Check(configWith(18000, 800))
	if err == nil {
		t.Fatal("Check() = nil, want rejection")
	}
	text := err.Error()
	for _, want := range []string{"18 000", "15 728", "2 272", "Acción correctiva", "No se ejecutó ninguna petición"} {
		if !strings.Contains(text, want) {
			t.Errorf("error = %q, want %q", text, want)
		}
	}
}

func TestCheckReportsBothViolations(t *testing.T) {
	err := Check(configWith(25000, 1100))
	if err == nil {
		t.Fatal("Check() = nil, want rejection")
	}
	text := err.Error()
	for _, want := range []string{"limits.memory_budget_mb", "load.max_vus", "20 000", "1 024"} {
		if !strings.Contains(text, want) {
			t.Errorf("error = %q, want %q", text, want)
		}
	}
}

func TestCheckRejectsAboveAbsolute(t *testing.T) {
	err := Check(configWith(25000, 450))
	if err == nil || !strings.Contains(err.Error(), "20 000") {
		t.Fatalf("error = %v, want absolute maximum", err)
	}
}

func TestCheckRejectsLowBudget(t *testing.T) {
	err := Check(configWith(5000, 100))
	if err == nil || !strings.Contains(err.Error(), "1 392") {
		t.Fatalf("error = %v, want budget maximum", err)
	}
}

func TestCheckRejectsBudgetAboveAbsolute(t *testing.T) {
	err := Check(configWith(2000, 1100))
	if err == nil || !strings.Contains(err.Error(), "1 024") {
		t.Fatalf("error = %v, want budget maximum", err)
	}
}

func openConfig(maxInFlight, budgetMB int) *config.Config {
	return &config.Config{
		Load: config.LoadConfig{
			Mode:     config.ModeOpen,
			MaxVUs:   2000,
			OpenLoop: config.OpenLoopConfig{MaxInFlight: maxInFlight},
		},
		Limits: config.Limits{MemoryBudgetMB: budgetMB, WarnAtPercent: 80},
	}
}

func TestMaxInFlightForBudget(t *testing.T) {
	if got := MaxInFlightForBudget(300); got != 4989 {
		t.Errorf("MaxInFlightForBudget(300) = %d, want 4989", got)
	}
	if got := MaxInFlightForBudget(450); got != 7782 {
		t.Errorf("MaxInFlightForBudget(450) = %d, want 7782", got)
	}
	if got := MaxInFlightForBudget(600); got != 10575 {
		t.Errorf("MaxInFlightForBudget(600) = %d, want 10575", got)
	}
}

func TestCheckOpenAllows8000At600(t *testing.T) {
	if err := Check(openConfig(8000, 600)); err != nil {
		t.Fatalf("Check() = %v, want nil", err)
	}
}

func TestCheckOpenRejects8000At450(t *testing.T) {
	err := Check(openConfig(8000, 450))
	if err == nil || !strings.Contains(err.Error(), "7 782") {
		t.Fatalf("error = %v, want budget maximum", err)
	}
}

func TestCheckOpenRejects15000At300(t *testing.T) {
	err := Check(openConfig(15000, 300))
	if err == nil {
		t.Fatal("Check() = nil, want rejection")
	}
	text := err.Error()
	for _, want := range []string{"open_loop.max_in_flight", "15 000", "4 989", "10 011", "Acción correctiva", "No se ejecutó ninguna petición"} {
		if !strings.Contains(text, want) {
			t.Errorf("error = %q, want %q", text, want)
		}
	}
}

func TestCheckOpenRejectsAboveAbsolute(t *testing.T) {
	err := Check(openConfig(25000, 450))
	if err == nil || !strings.Contains(err.Error(), "20 000") {
		t.Fatalf("error = %v, want absolute maximum", err)
	}
}

func TestCheckOpenBudgetAboveAbsolute(t *testing.T) {
	err := Check(openConfig(8000, 1100))
	if err == nil || !strings.Contains(err.Error(), "1 024") {
		t.Fatalf("error = %v, want budget maximum", err)
	}
}
