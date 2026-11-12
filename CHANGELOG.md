# Changelog

## v0.4.1 — 2026-11-12

### Corregido

- `httpx.Transport.Close()` libera el socket UDP y el goroutine de `http3.Transport` al terminar cada corrida; antes `CloseIdleConnections` dejaba esos recursos vivos en procesos de larga duración como la TUI.
- El motor usa `Close()` al finalizar la corrida en lugar de solo cerrar conexiones idle.

### Verificación

- `go test ./...`, `-race`, `make check` y `make vuln` verdes.

## v0.4.0 — 2026-11-12

### Añadido

- Soporte de HTTP/3 sobre QUIC con `target.http_version: 3` explícito.
- Transporte `http3.Transport` con TLS 1.3, `ca_file`, `insecure_skip_verify` y métricas `Protocol: "HTTP/3.0"`.
- Modo estricto: `3` falla si el servidor no negocia HTTP/3.
- Tests locales con `http3.Server` en loopback, concurrencia y `-race`.
- Dependencia directa `github.com/quic-go/quic-go v0.65.0`.
- Actualización de dependencias transitivas para `govulncheck`: `golang.org/x/net v0.62.0`, `golang.org/x/text v0.44.0`, `golang.org/x/crypto v0.59.0`.

### Cambiado

- `httpx.Transport` separa el transporte de peticiones (`HTTP`) del transporte HTTP/1.1 para WebSocket (`WS`).
- El handshake WebSocket usa un transporte compartido en lugar de clonar uno por VU.
- El motor cierra las conexiones idle del transporte al terminar la corrida.

### Limitaciones

- `3` no está disponible sobre `http://` (h3c no soportado).
- `auto` no descubre HTTP/3 por Alt-Svc.
- HTTP/3 no pasa por `HTTP_PROXY`/`HTTPS_PROXY`.
- WebSocket sigue usando HTTP/1.1, también con `http_version: 3`.

### Verificación

- `go test ./...`, `-race` en paquetes críticos, `make check`, `make e2e` y `make vuln` verdes.

## v0.3.0 — 2026-11-11

### Añadido

- Soporte de WebSocket cliente con pasos `connect`, `send`, `receive` y `close`.
- Conexiones persistentes por VU, con nombre (`connection`), headers, subprotocolos, timeout, mensajes de texto y binarios.
- Checks de handshake, `message_contains` en texto y `max_duration`; cierre automático al terminar cada iteración.
- Métricas de WebSocket (`Protocol: "WebSocket"`, bytes, duración, errores `ws_*`) en reporte, `run.json` y web.
- Endpoint `/ws` de echo en el demo y `examples/websocket.yaml`.
- Caso WebSocket en `make e2e`.
- Dependencia directa `github.com/coder/websocket v1.8.15`.

### Limitaciones

- El handshake WebSocket usa HTTP/1.1; WebSocket sobre HTTP/2 extendido no está soportado.
- Sin reconexión automática, ping explícito, compresión ni checks de mensajes binarios.
- `extract` no está soportado en pasos WebSocket v1.

### Verificación

- `go test ./...`, `-race` en paquetes críticos, `make check`, `make e2e` y `make vuln` verdes.

## v0.2.0 — 2026-11-11

### Añadido

- Menú interactivo de pantalla completa en la terminal, con navegación por flechas, historial, ejecución, validación, web, mantenimiento y ajustes.
- HTTP/2 sobre HTTPS con `target.http_version: auto | 1.1 | 2`.
  - `auto`: HTTP/2 si el servidor lo ofrece; fallback a HTTP/1.1.
  - `1.1`: fuerza HTTP/1.1.
  - `2`: exige HTTP/2 y falla si el servidor negocia HTTP/1.1.
- Contadores de protocolo en métricas, reporte de texto, `run.json` y UI web.
- `make vuln` y workflow de seguridad con `govulncheck` en CI y ejecución diaria.

### Seguridad

- Saneado de secuencias de terminal en menú, CLI, reporte, TUI y logs.
- Límites de longitud en campos de la TUI (256/4096).
- `prune` borra exactamente las corridas previsualizadas (`PruneIDs`).
- Rutas YAML que empiezan por `-` se normalizan.
- Redacción de credenciales del target en la TUI.
- Redacción de secretos explícitos cortos (`NewExplicit`).

### Cambios de comportamiento

- `target.http_version` usa `auto` por defecto. En objetivos HTTPS puede negociar HTTP/2; usa `http_version: 1.1` para conservar el comportamiento anterior.
- h2c, HTTP/3, gRPC y WebSockets siguen fuera de alcance.

### Documentación

- README ES/EN actualizados con menú, HTTP/2, límites y `make vuln`.
- Referencias al directorio privado `doc/` eliminadas de los README.

### Verificación

- `go test ./...`, `go test -race` en paquetes críticos, `make check`, `make e2e` y `make vuln` verdes.
- `govulncheck`: sin vulnerabilidades conocidas en fuente y binario.

## v0.1.0

- Versión inicial del proyecto.
