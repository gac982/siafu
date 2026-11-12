// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration parsed from YAML strings such as "30s".
type Duration struct {
	time.Duration
}

// Value returns the underlying duration.
func (d Duration) Value() time.Duration { return d.Duration }

// UnmarshalYAML parses a duration string.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("duración inválida en la línea %d: se esperaba un texto como \"30s\"", value.Line)
	}
	parsed, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("duración inválida %q en la línea %d: %w", value.Value, value.Line, err)
	}
	d.Duration = parsed
	return nil
}

// ThinkTime is either a fixed duration or a {min,max} range.
type ThinkTime struct {
	Min Duration
	Max Duration
}

// UnmarshalYAML accepts a scalar duration or a mapping with min and max.
func (t *ThinkTime) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		var d Duration
		if err := d.UnmarshalYAML(value); err != nil {
			return err
		}
		t.Min, t.Max = d, d
		return nil
	}

	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("think_time inválido en la línea %d: se esperaba una duración o {min,max}", value.Line)
	}
	for i := 0; i < len(value.Content); i += 2 {
		key, node := value.Content[i], value.Content[i+1]
		if key.Value != "min" && key.Value != "max" {
			return fmt.Errorf("campo desconocido %q en think_time (línea %d)", key.Value, key.Line)
		}
		var d Duration
		if err := d.UnmarshalYAML(node); err != nil {
			return err
		}
		if key.Value == "min" {
			t.Min = d
		} else {
			t.Max = d
		}
	}
	return nil
}

// StatusCodes accepts one HTTP status or a list of them.
type StatusCodes []int

// UnmarshalYAML accepts an integer or a sequence of integers.
func (s *StatusCodes) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		var code int
		if err := value.Decode(&code); err != nil {
			return fmt.Errorf("status inválido en la línea %d: %w", value.Line, err)
		}
		*s = StatusCodes{code}
		return nil
	}
	if value.Kind == yaml.SequenceNode {
		var codes []int
		if err := value.Decode(&codes); err != nil {
			return fmt.Errorf("lista de status inválida en la línea %d: %w", value.Line, err)
		}
		*s = codes
		return nil
	}
	return fmt.Errorf("status inválido en la línea %d: se esperaba un entero o una lista", value.Line)
}
