// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package httpx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/textproto"
	"strconv"
	"strings"
)

func extractJSON(data []byte, path string) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var value any
	if err := dec.Decode(&value); err != nil {
		return "", fmt.Errorf("JSON inválido: %w", err)
	}

	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			return "", fmt.Errorf("ruta JSON vacía")
		}
		switch current := value.(type) {
		case map[string]any:
			next, ok := current[segment]
			if !ok {
				return "", fmt.Errorf("ruta JSON %q no encontrada en %q", path, segment)
			}
			value = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(current) {
				return "", fmt.Errorf("índice %q inválido para %q", segment, path)
			}
			value = current[index]
		default:
			return "", fmt.Errorf("ruta JSON %q: %q no es un objeto ni una lista", path, segment)
		}
	}

	return jsonValueString(value)
}

func jsonValueString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case json.Number:
		return v.String(), nil
	case bool:
		return strconv.FormatBool(v), nil
	case nil:
		return "", fmt.Errorf("el valor extraído es null")
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("serializar valor extraído: %w", err)
		}
		return string(encoded), nil
	}
}

func extractHeader(headerValues map[string][]string, name string) (string, error) {
	canonical := textproto.CanonicalMIMEHeaderKey(name)
	values, ok := headerValues[canonical]
	if !ok || len(values) == 0 {
		return "", fmt.Errorf("cabecera %q no encontrada", name)
	}
	return values[0], nil
}
