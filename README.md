# Siafu

<p><img src="assets/logo.png" width="180" alt="Siafu logo"></p>

[English](README.en.md) · **Español**

![Go version](https://img.shields.io/badge/go-1.27%2B-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-AGPL--3.0--or--later-blue)
![Platforms](https://img.shields.io/badge/platforms-linux%20%7C%20macOS-lightgrey)
![Status](https://img.shields.io/badge/status-v0.4.2-orange)

**Pruebas de carga por terminal, configuradas con YAML y escritas en Go.**

Siafu simula usuarios concurrentes contra páginas web y APIs HTTP/HTTPS, muestra el avance en una TUI en vivo y entrega un reporte con umbrales y diagnóstico. Incluye un **modo cerrado** (VUs que esperan la respuesta) y un **modo open-loop** opcional (tasa de llegada controlada) para buscar el límite de un backend. No necesitas JavaScript, Node, un navegador ni servicios externos: un binario, un archivo YAML y el destino.

```bash
./bin/siafu run examples/demo.yaml
```

> **Aviso de uso responsable:** usa Siafu únicamente contra sistemas que poseas o para los que tengas autorización explícita. Una prueba de carga puede saturar un servicio real.

---

## Tabla de contenido

- [Qué es Siafu](#qué-es-siafu)
- [Por qué existe](#por-qué-existe)
- [Características](#características)
- [Vista de la TUI](#vista-de-la-tui)
- [Requisitos](#requisitos)
- [Compilar desde el código fuente](#compilar-desde-el-código-fuente)
- [Instalar](#instalar)
- [Inicio rápido](#inicio-rápido)
- [Uso de la CLI](#uso-de-la-cli)
- [Menú interactivo](#menú-interactivo)
- [Atajos de la TUI](#atajos-de-la-tui)
- [Códigos de salida](#códigos-de-salida)
- [Configuración YAML](#configuración-yaml)
- [Plantillas y variables](#plantillas-y-variables)
- [Métricas y veredicto](#métricas-y-veredicto)
- [Diagnóstico](#diagnóstico)
- [Modo CI](#modo-ci)
- [Resultados y web local](#resultados-y-web-local)
- [Servidor de demostración](#servidor-de-demostración)
- [Arquitectura](#arquitectura)
- [Desarrollo y pruebas](#desarrollo-y-pruebas)
- [Limitaciones conocidas](#limitaciones-conocidas)
- [Solución de problemas](#solución-de-problemas)
- [Seguridad](#seguridad)
- [Roadmap](#roadmap)
- [Contribuir](#contribuir)
- [Licencia](#licencia)

---

## Qué es Siafu

Siafu es una herramienta de línea de comandos para medir cómo responde un sitio web o una API bajo carga concurrente.

Tú describes la prueba en YAML:

- el host de destino;
- cuántos usuarios virtuales quieres y cómo sube/baja la carga (modo cerrado), o qué tasa de llegada buscas (modo open-loop);
- qué pasos ejecuta cada usuario (login, consulta, detalle, etc.);
- qué respuestas se consideran correctas;
- qué latencia, tasa de error y throughput mínimo aceptas.

Siafu ejecuta la prueba, muestra las métricas en tiempo real y termina con un veredicto:

- **APROBADA** si se cumplen todos los umbrales;
- **FALLIDA** si alguno no se cumple;
- **SIN UMBRALES** si el YAML no define criterios;
- **ABORTADA** si detienes la ejecución;
- **ERROR** si no se completó ninguna petición.

El objetivo es responder rápido a preguntas como:

- ¿Cuántas peticiones por segundo aguanta mi API?
- ¿La latencia p95 se mantiene por debajo de 300 ms con 50 usuarios?
- ¿Aparecen errores 5xx o 429 cuando subo la carga?
- ¿El cuello de botella está en el servidor o en mi máquina de pruebas?
- ¿El último cambio degradó el login o el listado de productos?

## Por qué existe

Herramientas como k6 son muy potentes, pero modelar escenarios exige escribir JavaScript. Siafu apunta a un caso más acotado:

- configuración versionable y revisable en YAML;
- validación estricta antes de ejecutar;
- ejecución desde la terminal, sin infraestructura;
- resultados explicables, con umbrales y diagnóstico determinista;
- un solo binario para Linux y macOS.

No pretende reemplazar a k6, JMeter, Gatling ni a un servicio de carga distribuida. Pretende ser la forma más rápida de lanzar una prueba de humo o de capacidad contra una web o una API, sin escribir código.

## Características

- **Escenarios multi-paso:** login → consulta → detalle, con cookies por usuario virtual.
- **HTTP/3 sobre QUIC:** `target.http_version: 3` explícito, con TLS 1.3 y métricas de protocolo.
- **WebSocket:** conexiones por VU con `connect`, `send`, `receive` y `close`, checks de mensaje y métricas de protocolo.
- **Usuarios virtuales con rampa:** etapas de subida, meseta y bajada, con think time.
- **Modo open-loop opcional:** tasa de llegada en iteraciones/s, pool acotado por `max_in_flight`, descarte controlado y reporte del punto de fallo por etapa.
- **Guardas de memoria:** admisión previa, presupuesto de análisis, watchdog de RSS y límites de logs/lotes/conexiones.
- **Persistencia y web local:** historial JSON por corrida y UI mobile-first con gráficos SVG, sin dependencias de frontend.
- **Variables y secretos:** `${ENV:TOKEN}`, `${VU}`, `${ITER}` y extracción de tokens desde JSON o cabeceras.
- **Checks por respuesta:** código HTTP, subcadena en el cuerpo y duración máxima.
- **Métricas en vivo:** VUs, RPS, p50/p90/p95/p99, errores, checks, iteraciones y bytes.
- **Métricas del generador:** CPU, RAM, disco y red del equipo que ejecuta Siafu.
- **Umbrales:** p95, p99, tasa de error y RPS mínimo con veredicto y código de salida.
- **Diagnóstico:** reglas deterministas para detectar saturación, rate limiting, errores de red, TLS, CPU del generador y más.
- **TUI de cuatro colores:** dashboard en vivo y reporte final desplazable.
- **Modo texto:** `--no-tui` para CI, cron o terminales no interactivas.
- **Sin JavaScript:** no se renderiza el DOM ni se descargan recursos de una página.
- **Un binario:** sin runtime, sin Node, sin servicios en la nube.
- **Tests continuos:** unitarios, integración con `httptest`, TUI sin PTY, E2E con servidor demo y `-race`.

## Vista de la TUI

Esquema del dashboard en ejecución (los valores son ilustrativos):

```text
┌ SIAFU · Demo local ───────────────────────────── http://127.0.0.1:8080 ┐
│ EJECUTANDO  00:12 / 00:12 · etapa 2/3 · VUs 20/20  ██████████            │
├─ GENERADOR ─────────────────────────┬─ CARGA ────────────────────────────┤
│ CPU    34%                          │ VUs        20/20     RPS   142.3   │
│ RAM    6.2 GB / 16.0 GB · 39%       │ p95        210 ms    p99   480 ms  │
│ DISCO  61% · 142 GB libres          │ Errores    0.8%      Checks 99.2%  │
│ RED    ↓ 12.4 MB/s · ↑ 3.1 MB/s     │ Peticiones 18 420    Iter. 3 905   │
├─ TENDENCIA ─────────────────────────┴───────────────────────────────────┤
│ RPS  ▁▂▃▄▅▆▇█▇▆▅▄▃▂ 142      p95  ▁▂▂▃▄▅▆▇▇▆▅▄▃ 210 ms                  │
├─ LOG ──────────────────────────────────────────────────────────────────┤
│ 10:42:01 INFO  etapa 2/3                                   │
│ 10:42:03 WARN  products/listado → 500 (status)              │
└ Ctrl+D detener   X logs   ? ayuda   Ctrl+C salir   ↑↓ desplazar ────────┘
```

La paleta usa exactamente cuatro colores:

| Color | Hex | Uso |
|---|---|---|
| `Ink` | `#0E1116` | Fondo de paneles |
| `Bone` | `#E6E1D6` | Texto primario |
| `Ember` | `#FC4C02` | Acento, progreso, fallos |
| `Ash` | `#7A8394` | Texto secundario y bordes |

Pensada para terminales oscuras con truecolor; Lipgloss degrada a 256 colores cuando hace falta.

## Requisitos

| Requisito | Detalle |
|---|---|
| Go | **1.27 o superior** (lo fija `go.mod`) |
| Sistema | Linux o macOS, amd64 o arm64 |
| Herramientas | `git`; `make` es opcional pero recomendado |
| Red | Acceso al destino HTTP/HTTPS que quieras probar |
| Terminal | TUI interactiva opcional; `--no-tui` no necesita TTY |

Siafu compila con `CGO_ENABLED=0`: el binario no depende de librerías del sistema.

> Windows no está soportado en v0.1: las métricas de host y los objetivos de compilación están pensados para Linux y macOS.

## Compilar desde el código fuente

### 1. Clona el repositorio

```bash
git clone https://github.com/proyect-octopus/siafu.git
cd siafu
```

### 2. Comprueba la versión de Go

```bash
go version
# go version go1.27.x ...
```

Si tu versión es anterior a 1.27, actualiza Go antes de continuar.

### 3. Compila con Make

```bash
make build
```

El binario queda en:

```text
bin/siafu
```

La compilación inyecta la versión, el commit y la fecha en el binario mediante `-ldflags`.

### 4. Verifica el binario

```bash
./bin/siafu version
./bin/siafu --help
```

Salida esperada (los valores pueden variar):

```text
[SIAFU logo]
siafu v0.4.2 (8eb96fa) built 2026-11-12T23:39:17Z · linux/amd64
```

En una terminal, `version` imprime el logo en color; si la salida no es una TTY o defines `NO_COLOR`, se muestra en blanco y negro.

### 5. Compila sin Make (alternativa)

Si no tienes `make`:

```bash
mkdir -p bin
CGO_ENABLED=0 go build -trimpath -o bin/siafu ./cmd/siafu
./bin/siafu version
```

Ese binario funciona, pero la versión aparecerá como `dev`; para inyectar metadatos usa el Makefile o pásalos tú:

```bash
VERSION=$(git describe --tags --always --dirty)
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w \
    -X github.com/proyect-octopus/siafu/internal/version.Version=$VERSION \
    -X github.com/proyect-octopus/siafu/internal/version.Commit=$COMMIT \
    -X github.com/proyect-octopus/siafu/internal/version.Date=$DATE" \
  -o bin/siafu ./cmd/siafu
```

### 6. Compilación cruzada

```bash
make build-all
```

Genera:

```text
bin/siafu-linux-amd64
bin/siafu-linux-arm64
bin/siafu-darwin-amd64
bin/siafu-darwin-arm64
```

### 7. Ejecuta la suite de pruebas

```bash
make check
```

Esto ejecuta `gofmt`, `go vet` y `go test -race ./...`. También puedes ejecutar el E2E completo:

```bash
make e2e
```

El E2E compila el binario, arranca el servidor demo, ejecuta una prueba que debe aprobar y otra que debe fallar.

## Instalar

### Instalar en el `GOPATH`/`GOBIN`

```bash
make install
```

o directamente:

```bash
CGO_ENABLED=0 go install ./cmd/siafu
```

Asegúrate de que `$(go env GOPATH)/bin` esté en tu `PATH`.

### Instalar desde un repositorio publicado

Cuando el repositorio esté publicado y etiquetado:

```bash
go install github.com/proyect-octopus/siafu/cmd/siafu@latest
```

> Si tu fork usa otra ruta de módulo, cambia `github.com/proyect-octopus/siafu` por la tuya.

## Inicio rápido

La forma más rápida de ver Siafu funcionando no requiere ningún servicio externo: incluye un servidor HTTP local de demostración.

### Terminal 1: servidor demo

```bash
make demo
```

El servidor escucha en `http://127.0.0.1:8080` y expone páginas, productos, login con token y endpoints lentos/erróneos.

### Terminal 2: prueba de carga

Valida primero el YAML:

```bash
./bin/siafu validate examples/demo.yaml
```

Ejecuta la prueba con TUI:

```bash
./bin/siafu run examples/demo.yaml
```

Al terminar verás un reporte con métricas, umbrales y diagnóstico. La prueba de demo está pensada para aprobar (`APROBADA`).

Para ejecutarla sin TUI:

```bash
./bin/siafu run --no-tui examples/demo.yaml
```

Para ver un fallo controlado:

```bash
./bin/siafu run --no-tui examples/failing.yaml
```

`examples/failing.yaml` define un `min_rps` imposible; la salida debe ser `FALLIDA` y el código de salida `1`.

## Uso de la CLI

```text
siafu <comando> [opciones]

Comandos:
  run <archivo.yaml>       ejecuta una prueba de carga
  validate <archivo.yaml>  valida la configuración sin ejecutar
  runs <subcomando>        historial de corridas (list, show, compare, prune, reindex)
  serve                    sirve la interfaz web local de resultados
  menu                     abre el menú interactivo
  version                  muestra la versión y la plataforma

Opciones:
  -h, --help               muestra la ayuda
```

### Menú interactivo

Para quien prefiere no recordar comandos y flags:

```bash
siafu          # abre el menú si hay terminal
siafu menu     # forma explícita
```

Es una TUI de pantalla completa con la paleta de Siafu: se navega con `↑/↓` o `j/k`, se elige con `Enter` y se vuelve con `Esc`/`q`; `Ctrl+C` sale. El menú permite:

1. **Ejecutar una prueba:** elegir un YAML, ver su resumen, ajustar semilla, etiqueta, directorio, no guardar, sin TUI o debug, y ejecutar.
2. **Ver corridas anteriores:** lista paginada, filtros por modo/veredicto/texto, detalle y comparación con selección por cursor.
3. **Abrir la web de resultados:** puerto configurable, apertura automática del navegador y `s`/`Esc` para detener el servidor y volver.
4. **Validar un YAML** sin ejecutar carga.
5. **Mantenimiento:** conservar solo N corridas (con previsualización y confirmación `SI`) y reindexar.
6. **Ajustes y versión:** versión y directorio de resultados de la sesión.

El menú sanea las secuencias de terminal de los textos que muestra y limita la longitud de sus campos. El YAML se sigue escribiendo fuera; el menú no lo genera ni lo edita. La CLI por flags (`run`, `validate`, `runs`, `serve`) no cambia. Sin TTY, `siafu` sin argumentos mantiene la ayuda y el código 2; `siafu menu` explica que necesita una terminal.

### `siafu run`

```bash
siafu run <archivo.yaml> [--no-tui] [--seed N] [--debug] [--results-dir DIR] [--no-save] [--tag TAG]
```

| Flag | Descripción |
|---|---|
| `--no-tui` | Fuerza salida de texto; útil para CI. |
| `--seed N` | Fija la semilla de think time y reproducibilidad. Sobrescribe el YAML. |
| `--debug` | Activa logs de depuración del orquestador. |
| `--results-dir DIR` | Directorio del historial; por defecto `.siafu/runs`. |
| `--tag TAG` | Etiqueta libre para filtrar la corrida en el historial. |
| `--no-save` | No guarda la corrida. |

Los flags pueden ir antes o después del archivo:

```bash
siafu run --no-tui examples/demo.yaml
siafu run examples/demo.yaml --no-tui
```

Si la salida no es una TTY, Siafu usa automáticamente el modo texto.

### `siafu validate`

Valida sintaxis, campos desconocidos, reglas semánticas, plantillas y referencias a variables sin ejecutar carga:

```bash
siafu validate examples/api.yaml
```

Ejemplo de salida:

```text
OK  API de ejemplo
  1 escenario(s), 2 paso(s)
  duración 1m0s, pico 20 VUs
  umbrales: p95≤400ms · p99≤1.2s · errores≤1.00% · rps≥10.0
```

En modo open-loop el resumen cambia a la tasa y al pool:

```text
OK  Estrés API open-loop
  1 escenario(s), 1 paso(s)
  modo open · duración 30s · in-flight 8000 · pico 20000 iteraciones/s
  umbrales: p95≤2s · errores≤0.00%
```

Si hay errores, se acumulan todos con la ruta del campo:

```text
configuración inválida:
- scenarios[0].steps[1].request.url: referencia desconocida "${token}"
- thresholds.latency_p99: debe ser mayor o igual que latency_p95
```

### `siafu version`

```bash
siafu version
```

Muestra versión, commit, fecha de compilación, sistema operativo y arquitectura.

### Señales

- En la TUI, `Ctrl+D` detiene la prueba de forma ordenada y muestra el reporte parcial.
- En la TUI, `Ctrl+C` sale inmediatamente con código 130.
- En modo texto, `SIGINT`/`SIGTERM` abortan y emiten un reporte parcial `ABORTADA` con código 130.

## Atajos de la TUI

| Tecla | Acción |
|---|---|
| `Ctrl+D` | Detener la prueba y mostrar el reporte (`ABORTED`). |
| `Ctrl+C` | Salir inmediatamente (130 mientras la prueba está en curso). |
| `X` | Alternar logs a pantalla completa. |
| `?` | Mostrar u ocultar la ayuda. |
| `↑` / `↓` | Desplazar logs o reporte. |
| `PgUp` / `PgDn` | Desplazamiento rápido. |
| `Q` / `Esc` | Salir desde la pantalla de reporte. |

La TUI se adapta al ancho:

- dos columnas en terminales anchas;
- dos columnas compactas en anchos medios;
- una columna por debajo de 56 columnas;
- oculta la tendencia si no hay altura suficiente;
- oculta el panel de log si queda menos de tres líneas y permite abrirlo con `X`.

## Códigos de salida

| Código | Significado |
|---|---|
| `0` | Prueba aprobada o sin umbrales definidos. |
| `1` | Algún umbral incumplido. |
| `2` | Error de configuración, validación, uso o rechazo por límites de seguridad. |
| `3` | Error fatal de ejecución, cero peticiones completadas o límite de memoria alcanzado. |
| `130` | Prueba abortada por el usuario o una señal. |

Estos códigos están pensados para que CI pueda distinguir un fallo de rendimiento (`1`) de un YAML inválido (`2`) o de un problema de ejecución (`3`).

## Configuración YAML

Un archivo mínimo:

```yaml
version: 1
name: "Mi primera prueba"

target:
  base_url: "https://api.example.com"
  timeout: 10s

load:
  think_time: 300ms
  stages:
    - duration: 30s
      target_vus: 10
    - duration: 1m
      target_vus: 50
    - duration: 30s
      target_vus: 0

scenarios:
  - name: consulta
    steps:
      - name: listar
        request:
          method: GET
          url: /items
        expect:
          status: 200
          body_contains: "items"

thresholds:
  latency_p95: 400ms
  error_rate: 0.01
```

### Raíz

| Campo | Tipo | Obligatorio | Descripción |
|---|---|---|---|
| `version` | int | sí | Debe ser `1`. |
| `name` | string | sí | Nombre de la prueba (1–80 caracteres). |
| `target` | objeto | sí | Destino HTTP y opciones del cliente. |
| `load` | objeto | sí | Etapas, think time y límites. |
| `vars` | map | no | Variables globales inmutables. |
| `scenarios` | lista | sí | 1–20 escenarios, cada uno con pasos. |
| `thresholds` | objeto | no | Criterios de aceptación. |
| `limits` | objeto | no | Presupuestos de memoria, análisis, logs y conexiones. |

### `target`

| Campo | Default | Reglas |
|---|---|---|
| `base_url` | — | Obligatorio. `http` o `https`, con host, sin query ni fragmento. No admite plantillas. |
| `timeout` | `30s` | Entre `1s` y `300s`; aplica a la petición completa. |
| `http_version` | `auto` | `auto`, `1.1`, `2` o `3`. Solo se aplica a HTTPS; `2` exige HTTP/2 y `3` exige HTTP/3. |
| `headers` | `{}` | Cabeceras comunes. Admiten plantillas; las del paso tienen prioridad. |
| `tls.insecure_skip_verify` | `false` | No verifica el certificado. Solo para pruebas controladas. |
| `tls.ca_file` | `""` | Ruta a un PEM de CA para el destino. Se resuelve relativa al YAML. |

Siafu negocia **HTTP/2 sobre HTTPS** cuando `http_version` es `auto` o `2`; si el servidor no lo ofrece, `auto` cae a HTTP/1.1 y `2` falla con un error de protocolo. `1.1` fuerza el comportamiento anterior. En `http://` se usa HTTP/1.1 (h2c no está soportado). También mantiene keep-alive, proxy desde `HTTP_PROXY`/`HTTPS_PROXY` y redirecciones de hasta 10 saltos.

Con `http_version: 3` Siafu usa **HTTP/3 sobre QUIC** (UDP y TLS 1.3) y falla si el servidor no negocia HTTP/3. `3` no está disponible sobre `http://` (h3c no está soportado), no usa Alt-Svc automático en `auto` y no pasa por `HTTP_PROXY`/`HTTPS_PROXY`, porque los proxies HTTP actuales no transportan QUIC. El handshake de WebSocket sigue usando HTTP/1.1 aunque el destino sea `3`.

### `load`

| Campo | Default | Reglas |
|---|---|---|
| `stages` | — | 1–50 etapas. La duración total no puede superar 6 horas. |
| `stages[].duration` | — | Mayor que 0. |
| `stages[].target_vus` | — | 0–`max_vus`; al menos una etapa debe superar 0. |
| `max_vus` | `2000` | Entre 1 y 20 000. |
| `think_time` | `0` | Escalar (`500ms`) o rango `{min,max}`. |
| `seed` | aleatoria | Entero de 64 bits para reproducir think time. |
| `mode` | `closed` | `closed` (VUs) u `open` (tasa de llegada). |
| `open_loop.stages` | — | Obligatorio en `open`: 1–50 etapas `{duration, target_rate}`. |
| `open_loop.max_in_flight` | — | Obligatorio en `open`: 1–20 000. |
| `open_loop.on_overload` | `drop` | Solo `drop`: sin cola; al saturar se descarta. |
| `open_loop.on_analysis_exhausted` | `skip` | `skip` o `fail`. |

La rampa es lineal: cada etapa interpola desde el objetivo anterior (0 al inicio) hasta su `target_vus`. Una etapa final con `target_vus: 0` baja la carga antes de terminar.

**Reparto de escenarios:** cada VU elige un escenario por turnos (`id % número_de_escenarios`). En v0.1 no hay pesos; cada escenario recibe aproximadamente la misma cantidad de VUs.

### Modo open-loop

`load.mode: open` cambia el modelo: en vez de VUs que esperan la respuesta antes de la siguiente iteración, se define una **tasa de llegada** de iteraciones por segundo y un pool de workers acotado. El modo cerrado sigue siendo el default.

```yaml
load:
  mode: open
  open_loop:
    stages:
      - {duration: 1m, target_rate: 1000}
      - {duration: 2m, target_rate: 5000}
    max_in_flight: 8000
    on_overload: drop
    on_analysis_exhausted: skip
```

- `target_rate` se mide en **iteraciones/s**. Con un solo paso coincide con RPS; con varios pasos, el RPS real es mayor.
- Cada worker tiene su propia sesión y cookie jar. En v1 la sesión es del worker, no de un usuario único entre iteraciones.
- Si no hay worker libre, la iteración se **descarta** (`dropped_iterations`) y se reporta. No hay cola ilimitada.
- `max_in_flight` acota workers e iteraciones en vuelo. El pico nunca lo supera.
- `think_time` no se usa en este modo; el rate controla el ritmo.
- El reporte muestra tasa objetivo, tasa alcanzada media, tasa media en la etapa objetivo, in-flight pico, descartes y el primer punto de fallo por etapa.
- Los errores del backend no abortan la prueba; se registran y el reporte los separa de los descartes del generador.
- `on_analysis_exhausted`: `skip` omite `body_contains`/`extract` cuando el presupuesto de análisis está lleno; `fail` marca el paso con `analysis_budget`.
- Memoria: 32 MiB + 55 KiB por in-flight. Con 450 MiB el máximo admitido es 7 782; con 600 MiB, 10 575. Medido: 8 000 in-flight ≈ 467 MiB. Para 8 000 usa `memory_budget_mb: 600`.

### `limits`

Presupuestos de seguridad del proceso. La admisión se evalúa **antes** de crear conexiones o lanzar VUs: si `load.max_vus` excede la capacidad del presupuesto, Siafu rechaza el YAML con código `2` y no ejecuta ninguna petición.

| Campo | Default | Reglas |
|---|---|---|
| `memory_budget_mb` | `450` | Entre 1 y 1024. Presupuesto de RSS del proceso. |
| `warn_at_percent` | `80` | Entre 50 y 95. Porcentaje del presupuesto que genera un aviso. |
| `analysis_budget_mb` | `64` | Entre 1 y 128. Bytes máximos retenidos por análisis de cuerpos. |
| `max_log_lines` | `1000` | Entre 100 y 10 000. Líneas del ring de logs de la TUI. |
| `max_batch_samples` | `64` | Entre 1 y 512. Muestras por lote antes de enviarlas a métricas. |
| `max_idle_conns` | `1024` | Entre 1 y 4096. Conexiones keep-alive ociosas por host. |

La capacidad de VUs se estima con una reserva de 32 MiB para el proceso y 50 KiB por VU. Con `memory_budget_mb: 450` el máximo seguro es **8 560 VUs**; con `1024` es **19 849 VUs**. Para ejecutar 15 000 VUs usa `memory_budget_mb: 1024`, porque los picos transitorios de la rampa pueden superar los 800 MiB. El tope absoluto es **1024 MiB** y **20 000 VUs**.

Medido en el equipo de referencia: 10 000 VUs usan ~290 MiB en estado estable y 15 000 VUs alcanzan entre **699 y 967 MiB** de pico (HWM) durante la rampa. El watchdog mide el RSS actual, no el HWM, y tolera picos breves; el presupuesto no es un techo duro del HWM. Usa rampas progresivas y no ajustes el presupuesto al valor mínimo.

Si el RSS supera `memory_budget_mb` durante dos muestras consecutivas (≈2 s), Siafu detiene la prueba para evitar un OOM y termina con código `3`.

### Capacidad y límites del sistema

Medido con el demo local (respuestas pequeñas, HTTP/1.1, 16 vCPU) y el perfil `examples/stress-15000.yaml`:

| VUs | RSS pico | Conexiones establecidas | RPS promedio | Errores |
|---:|---:|---:|---:|---:|
| 2 000 | 108 MiB | 1 024 | 13 589 | 0 |
| 5 000 | 283 MiB | 4 355 | 33 129 | 0 |
| 10 000 | 550 MiB | 9 040 | 56 572 | 0 |
| 15 000 | 699–967 MiB | 9 971 | 55 623 | 0 |

La rampa importa: los picos transitorios del heap durante una rampa agresiva pueden superar el RSS estable. Usa rampas progresivas y un presupuesto con margen. Para 15 000 VUs el perfil declara `memory_budget_mb: 1024`. Las conexiones se cuentan en el lado cliente; el think time y el pool de conexiones idle reutilizan conexiones, por eso el pico establecido puede ser menor que el número de VUs.

**Puertos:** el rango efímero por defecto (`32768-60999`) ofrece 28 232 puertos por IP y destino. 15 000 conexiones caben con margen; 50 000 no caben en una sola IP y requieren HTTP/2 o ejecución distribuida.

**Límites del sistema:** ejecuta `bash scripts/preflight.sh` (o `make preflight`) antes de una prueba grande. Comprueba:

- `ulimit -n` ≥ 32 768;
- `net.ipv4.ip_local_port_range` con ≥ 20 000 puertos;
- `net.core.somaxconn` ≥ 4 096;
- `net.ipv4.tcp_max_syn_backlog` ≥ 2 048;
- `net.netfilter.nf_conntrack_max` ≥ 262 144 si hay firewall/NAT.

El harness completo se ejecuta con `make stress` (cerrado) o `make stress-open` (open-loop); compila, valida el preflight, arranca el demo y mide RSS/conexiones/descartes. Para una línea base: `VUS=10000 make stress`.

Los cuerpos grandes quedan fuera de este objetivo: 15 000 VUs con `body_contains` o `extract` sobre respuestas de 1 MiB multiplican la memoria por el presupuesto de análisis. Usa respuestas pequeñas o reduce la concurrencia.

### `scenarios[].steps[]`

| Campo | Default | Reglas |
|---|---|---|
| `name` | — | Obligatorio, único en el escenario. |
| `request.method` | `GET` | `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`. |
| `request.url` | — | Obligatorio. Absoluta o relativa a `base_url`. |
| `request.headers` | `{}` | Cabeceras del paso; admiten plantillas. |
| `request.body` | `""` | Cuerpo de la petición; admite plantillas. |
| `expect` | 2xx/3xx | Checks de la respuesta. |
| `extract` | `{}` | Variables a extraer de la respuesta. |
| `websocket` | — | Acción WebSocket; sustituye a `request`. Ver abajo. |

### `expect`

| Campo | Default | Descripción |
|---|---|---|
| `status` | 200–399 | Entero o lista de enteros (100–599). |
| `body_contains` | `""` | Subcadena a buscar en los primeros 1 MiB del cuerpo. |
| `max_duration` | `0` (off) | Duración máxima aceptable para ese paso. |

Los tres checks configurados se evalúan siempre. Un check fallido marca la petición como fallida, pero el escenario continúa con el siguiente paso. La extracción solo se ejecuta si todos los checks pasaron.

### `extract`

```yaml
extract:
  token: "json:data.token"
  product_id: "json:items.0.id"
  session: "header:X-Session-Id"
```

- `json:<ruta>` navega objetos y arrays con puntos (`items.0.id`).
- `header:<Nombre>` lee una cabecera de respuesta.
- Las variables extraídas se guardan como texto.
- Se borran al comenzar cada iteración: cada vuelta debe volver a extraer lo que necesite.
- Si la extracción falla, la iteración actual se aborta.
- Una clave extraída no puede colisionar con una variable global.

### WebSocket

Un paso `websocket` sustituye a `request` y ejecuta una acción sobre una conexión con nombre, propia de cada VU:

```yaml
- name: conectar
  websocket: {action: connect, url: /ws, connection: market}
- name: enviar
  websocket: {action: send, connection: market, text: '{"type":"subscribe"}'}
- name: recibir
  websocket: {action: receive, connection: market, expect: {message_contains: "subscribed"}}
- name: cerrar
  websocket: {action: close, connection: market}
```

| Campo | Default | Reglas |
|---|---|---|
| `action` | — | `connect`, `send`, `receive` o `close`. |
| `connection` | `default` | Nombre por VU; `[A-Za-z0-9_-]{1,32}`. |
| `url` | — | Solo en `connect`; relativa a `base_url` o `ws://`/`wss://`. |
| `headers` | `{}` | Solo en `connect`; plantillas permitidas. |
| `subprotocols` | `[]` | Solo en `connect`. |
| `text` | `""` | Solo en `send`; exactamente uno de `text`/`binary_base64`. |
| `binary_base64` | `""` | Solo en `send`; mensaje binario. |
| `timeout` | `target.timeout` | Deadline de handshake o mensaje; 1s–300s. |
| `expect.status` | `101` | Solo en `connect`. |
| `expect.message_contains` | `""` | Solo en `receive`; texto. |
| `expect.max_duration` | `0` (off) | Connect, send o receive. |
| `close_code` | `1000` | Solo en `close`; 1000–4999. |

- El handshake usa **HTTP/1.1**, aunque `target.http_version` sea `auto`, `2` o `3`.
- `wss://` usa `target.tls` (`ca_file` o `insecure_skip_verify`).
- Las conexiones se cierran al terminar cada iteración, con o sin error.
- `message_contains` solo aplica a mensajes de texto; el envío binario está soportado, pero los checks binarios quedan para después.
- `extract` no está soportado en pasos WebSocket v1.

### `vars` y plantillas

```yaml
vars:
  user_email: "load+${ENV:RUN_ID}@example.com"
  user_agent: "siafu/${ENV:VERSION}"
```

| Sintaxis | Significado |
|---|---|
| `${nombre}` | Variable global o extraída del VU. |
| `${ENV:NOMBRE}` | Variable de entorno, resuelta en ejecución. |
| `${ENV:NOMBRE:-valor}` | Variable de entorno con valor por defecto. |
| `${VU}` | Número de usuario virtual (1-based). |
| `${ITER}` | Número de iteración del VU (1-based). |
| `$${` | Escape: produce `${` literal. |

Las variables globales son inmutables y se evalúan en cada expansión. No pueden referenciar otras variables globales; sí pueden usar `ENV`, `VU` e `ITER`.

### `thresholds`

| Campo | Ejemplo | Regla |
|---|---|---|
| `latency_p95` | `400ms` | p95 observado ≤ valor. |
| `latency_p99` | `1200ms` | p99 observado ≤ valor. |
| `error_rate` | `0.01` | Fracción de peticiones fallidas ≤ valor (0–1). |
| `min_rps` | `20` | RPS promedio ≥ valor. |

Si no defines umbrales, el veredicto es `SIN UMBRALES` y el código de salida es `0`.

### Ejemplo de API autenticada

```yaml
version: 1
name: "API autenticada"

target:
  base_url: "https://api.example.com"
  timeout: 10s
  headers:
    Accept: "application/json"

load:
  think_time:
    min: 200ms
    max: 600ms
  stages:
    - duration: 15s
      target_vus: 5
    - duration: 30s
      target_vus: 20
    - duration: 15s
      target_vus: 0

vars:
  user_email: "load+${ENV:RUN_ID}@example.com"

scenarios:
  - name: login_and_profile
    steps:
      - name: login
        request:
          method: POST
          url: /login
          headers:
            Content-Type: application/json
          body: |
            {"email": "${user_email}", "password": "${ENV:LOAD_PASSWORD}"}
        expect:
          status: 200
        extract:
          token: "json:token"

      - name: profile
        request:
          method: GET
          url: /me
          headers:
            Authorization: "Bearer ${token}"
        expect:
          status: 200
          body_contains: "email"

thresholds:
  latency_p95: 400ms
  latency_p99: 1200ms
  error_rate: 0.01
  min_rps: 10
```

Ejecuta con:

```bash
export RUN_ID="$(date +%s)"
export LOAD_PASSWORD="tu-password-de-prueba"
./bin/siafu validate examples/api.yaml
./bin/siafu run examples/api.yaml
```

### Ejemplo de páginas web

```yaml
version: 1
name: "Sitio público"

target:
  base_url: "https://example.com"
  timeout: 15s

load:
  think_time: 800ms
  stages:
    - duration: 20s
      target_vus: 10
    - duration: 40s
      target_vus: 30
    - duration: 10s
      target_vus: 0

scenarios:
  - name: home
    steps:
      - name: portada
        request:
          method: GET
          url: /
        expect:
          status: 200
          body_contains: "<html"

  - name: about
    steps:
      - name: acerca
        request:
          method: GET
          url: /about
        expect:
          status: [200, 304]

thresholds:
  latency_p95: 800ms
  error_rate: 0.02
```

Siafu no descarga CSS, imágenes ni ejecuta JavaScript: una “página” es la petición HTTP que tú declares.

## Plantillas y variables

### Ciclo de vida

- Cada VU tiene su propio `http.Client`, su cookie jar y su mapa de variables extraídas.
- Las cookies persisten durante toda la prueba.
- Las variables extraídas se limpian al inicio de cada iteración.
- Las variables globales de `vars` son inmutables y se evalúan en cada expansión.
- `ITER` empieza en 1 y se incrementa después de cada intento, incluso si la iteración se aborta.
- Una variable global no puede referenciar otra variable global.

### Errores que abortan una iteración

- Error de transporte (DNS, dial, TLS, timeout, lectura).
- Plantilla sin resolver (por ejemplo, falta `${ENV:TOKEN}`).
- Extracción fallida.

Un check fallido (status, cuerpo o duración) **no** aborta la iteración; continúa con el siguiente paso.

### Secretos

- Los valores de `${ENV:...}` se resuelven en ejecución, no al validar.
- Siafu no escribe cuerpos de petición ni de respuesta en los logs.
- No se registran valores de cabeceras sensibles (`Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, `X-Auth-Token`, etc.).
- Los parámetros de query con nombres sensibles (`token`, `key`, `secret`, `password`, `signature`, `session`, etc.) se enmascaran.
- Los valores de variables de entorno referenciadas se reemplazan por `***` en los mensajes de log.

## Métricas y veredicto

### Peticiones

- **Peticiones completadas:** peticiones que terminaron de leer/descartar la respuesta. Las canceladas por una parada no entran en los totales ni en la latencia.
- **Fallidas:** peticiones con al menos un check fallido o un error de transporte/plantilla/extracción.
- **Tasa de error:** fallidas / completadas.
- **Checks:** checks aprobados / checks evaluados.
- **Iteraciones:** pasadas completas por un escenario; una iteración es fallida si alguna petición falló.

### Latencia

Siafu mide desde antes de enviar la petición hasta terminar de leer/descartar el cuerpo. Incluye conexión, TLS, espera de cabeceras y transferencia.

Percentiles calculados con HDR Histogram: p50, p90, p95, p99, mínimo, máximo y media.

### RPS

- **Promedio:** peticiones completadas / duración planificada.
- **Reciente:** peticiones de los últimos 5 segundos / 5.
- **Pico:** mejor ventana de 5 segundos y el instante en que ocurrió.

### Métricas open-loop

- **Tasa objetivo:** iteraciones/s que pide el scheduler en el instante actual (o el pico de las etapas).
- **Tasa alcanzada:** iteraciones/s medias durante la corrida.
- **Tasa media objetivo:** media de la etapa con mayor `target_rate`; es la cifra comparable con el objetivo en una meseta.
- **In-flight pico:** máximo de iteraciones simultáneas; nunca supera `max_in_flight`.
- **Descartes:** iteraciones que el generador no pudo despachar por falta de worker. No son errores del backend; el diagnóstico los reporta como saturación del generador.
- **Punto de fallo:** primera etapa cuya tasa de error supera el 1 %; los descartes se muestran aparte.
- **Análisis omitido:** pasos en los que `body_contains`/`extract` no se evaluaron por presupuesto de análisis agotado con `on_analysis_exhausted: skip`.

### Cuerpo de respuesta

- Si hay `body_contains` o `extract`, se analizan hasta **1 MiB**.
- Si no hay checks de cuerpo ni extracción, se descartan hasta **10 MiB**.
- Superar el límite marca `body_too_large` y cierra la conexión.
- Los cuerpos se leen descomprimidos si el servidor usa gzip (comportamiento de Go).

### Métricas del generador

Las tarjetas CPU, RAM, disco y red son del **equipo que ejecuta Siafu**, no del destino. El reporte lo indica explícitamente. Sirven para detectar si la máquina de pruebas se quedó sin CPU antes que el servidor.

### Veredicto

| Estado | Condición | Salida |
|---|---|---|
| `APROBADA` | Hay umbrales y todos se cumplen. | 0 |
| `FALLIDA` | Al menos un umbral falla. | 1 |
| `SIN UMBRALES` | No hay umbrales y hubo peticiones. | 0 |
| `ABORTADA` | El usuario detuvo la prueba. | 130 |
| `ERROR` | Cero peticiones completadas o error fatal. | 3 |

### Ejemplo de reporte

```text
SIAFU · Demo local
Destino: http://127.0.0.1:8080
Inicio: 2026-11-07 13:05:01
Duración: planificada 12.00s / real 12.04s
Modo: cerrado · pico 20 VUs · Semilla: 405
Veredicto: APROBADA

RESUMEN GLOBAL
  Peticiones:      18420
  Fallidas:        148 (0.80%)
  Iteraciones:     6140 (fallidas 148)
  RPS:             promedio 1535.0 · pico 1890.2
  Latencia:        p50 12ms · p90 35ms · p95 48ms · p99 90ms
  Checks:          18420/18420 (100.00%)
  Canceladas:      0

POR ESCENARIO
  home
    peticiones 6000 · fallidas 0 (0.00%) · iteraciones 6000
  products
    peticiones 6000 · fallidas 0 (0.00%) · iteraciones 3000

CÓDIGOS DE ESTADO
  200: 18420

UMBRALES
  ✓ latencia p95: 48ms (esperado ≤ 500ms)
  ✓ tasa de error: 0.80% (esperado ≤ 1.00%)

DIAGNÓSTICO
  · Todos los umbrales se cumplen.

GENERADOR
  CPU:  promedio 34.2% · máximo 51.0%
  RAM:  máximo 22.1%
  RED:  pico ↓ 12.4 MB/s · ↑ 3.1 MB/s
  (métricas del equipo que ejecutó siafu, no del destino)
```

En modo open-loop el reporte añade la tasa y los descartes:

```text
MODO OPEN LOOP
  Tasa objetivo:   20000.0 iteraciones/s (pico)
  Tasa alcanzada:  19999.1 iteraciones/s (media) · 19999.1 peticiones/s
  Tasa media objetivo: 19998.8 iteraciones/s (objetivo 20000.0/s)
  In-flight pico:  221
  Descartes:       0
  Punto de fallo:  sin fallo registrado
  ETAPAS
    etapa 1 · objetivo 1000.0/s · alcanzado 999.9/s · peticiones 5000 · fallidas 0 · descartes 0
    etapa 2 · objetivo 5000.0/s · alcanzado 4999.8/s · peticiones 25000 · fallidas 0 · descartes 0
```

## Diagnóstico

Al terminar, Siafu aplica reglas deterministas y explica las causas probables. No usa IA opaca ni puntuaciones arbitrarias.

| Señal | Interpretación |
|---|---|
| Cero peticiones | URL, conectividad, TLS o configuración incorrecta. |
| Errores de plantilla | Falta `${ENV:...}` o hay una referencia sin resolver. |
| DNS/dial/TLS/timeout | Problema de red, certificado, firewall o timeout. |
| 429 dominante | El destino aplica rate limiting. |
| 5xx dominante | Posible saturación o error del servidor. |
| 4xx dominante | Revisar `expect.status`, datos o autenticación. |
| p95 alto + RPS decreciente | Saturación del destino. |
| p95 alto sin errores | Red lenta o procesamiento lento. |
| `min_rps` no alcanzado | La prueba no generó suficiente carga; en open-loop revisar `target_rate` y `max_in_flight`. |
| Descartes open-loop | El generador no sostuvo la tasa; reducir `target_rate` o aumentar `max_in_flight`. |
| Análisis omitido | El presupuesto de análisis se agotó; `body_contains`/`extract` no se evaluaron en esos pasos. |
| CPU del generador alta | El cuello de botella puede ser la máquina de pruebas. |
| `body_too_large` | Respuestas por encima de los límites de análisis. |
| Límite de memoria alcanzado | El RSS superó `limits.memory_budget_mb`; la prueba se detuvo para evitar un OOM. |

## Modo CI

`--no-tui` o una salida no interactiva usan texto plano:

```bash
siafu run --no-tui loadtest.yaml
```

- El estado periódico va a **stdout**.
- Los logs van a **stderr**.
- El reporte final va a **stdout**.
- El código de salida permite romper el pipeline.

Ejemplo de GitHub Actions:

```yaml
name: load-test

on:
  workflow_dispatch:

jobs:
  load:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.27.x"

      - name: Build
        run: |
          go build -o siafu ./cmd/siafu

      - name: Run load test
        env:
          RUN_ID: ${{ github.run_id }}
          LOAD_PASSWORD: ${{ secrets.LOAD_PASSWORD }}
        run: |
          ./siafu validate examples/api.yaml
          ./siafu run --no-tui examples/api.yaml
```

En GitLab CI, CircleCI, Jenkins o cron, el patrón es el mismo: ejecutar el binario con `--no-tui` y respetar el código de salida.

## Resultados y web local

Cada corrida se guarda por defecto en `.siafu/runs/` (ignorado por Git):

```text
.siafu/runs/
  index.jsonl
  20261109-093354-demo-a1b2/
    run.json        # resumen versionado (schema_version 1)
    series.jsonl    # un punto por segundo: rps, p95/p99, errores, in-flight, target/achieved, dropped, cpu/mem
    summary.txt     # reporte de texto
```

Flags de `run`:

- `--results-dir DIR` (o `SIAFU_RESULTS_DIR`) cambia el directorio.
- `--tag TAG` etiqueta la corrida.
- `--no-save` no persiste (útil para CI o pruebas rápidas).

Historial:

```bash
siafu runs list
siafu runs list --json --limit 10 --mode open --verdict PASS
siafu runs show <id>
siafu runs compare <a> <b>
siafu runs prune --keep 50 --dry-run
siafu runs reindex
```

Interfaz web local:

```bash
siafu serve --addr 127.0.0.1:8080 --results-dir .siafu/runs
```

`--web-dir DIR` sirve los assets desde disco durante el desarrollo. Abre `http://127.0.0.1:8080`. Incluye lista de corridas con filtros y búsqueda, detalle con RPS/p95/p99/errores/host/open-loop, y comparación de dos corridas con deltas y gráficos superpuestos. La navegación usa enlaces (`#/runs/<id>`, `#/compare?a=...&b=...`).

La escritura es atómica y hace `fsync` best-effort de archivos y directorios; no es una base de datos transaccional. `run` puede crear `.siafu/runs`; los comandos de lectura (`runs list`, `serve`) no lo crean. La API y los assets aceptan `HEAD`.

La UI es HTML/CSS/JS sin dependencias, mobile first, con gráficos SVG propios y sin peticiones externas. El servidor solo escucha en localhost por defecto y es de solo lectura. JSON es la fuente de verdad; **no usa SQLite** en v1. No se guardan headers, cuerpos, query sensible ni secretos; el destino se redacta y el YAML se identifica por nombre y hash. Los umbrales de una corrida abortada se muestran como “No evaluados”; hay un botón **Refrescar**; los SVG incluyen `<desc>` y líneas de referencia cuando el YAML define p95/error/RPS (p99 queda como mejora posterior).

## Servidor de demostración

`examples/demo` es un objetivo HTTP pequeño y determinista para ejemplos, desarrollo y E2E. No requiere dependencias.

```bash
make demo
# o
make demo-build
./bin/siafu-demo -addr 127.0.0.1:8080
```

| Método | Ruta | Respuesta |
|---|---|---|
| `GET` | `/` | HTML con `<html>`. |
| `GET` | `/api/products` | Lista JSON con `products` e `items`. |
| `GET` | `/api/products/{id}` | Producto 1–3. |
| `POST` | `/api/login` | JSON `{"email":...}` → `{"token":"token-N"}`. |
| `GET` | `/api/me` | Requiere `Authorization: Bearer token-N`. |
| `GET` | `/api/slow?ms=200` | Duerme hasta 5 s. |
| `GET` | `/api/error` | `500` controlado. |

El servidor demo no está pensado para producción: guarda tokens en memoria y no tiene autenticación real.

## Arquitectura

```text
cmd/siafu                  binario principal
internal/cli               subcomandos, flags, señales y códigos de salida
internal/config            esquema YAML, defaults y validación
internal/admission         rechazo previo de configuraciones fuera del presupuesto
internal/vars              compilación/render de ${...}
internal/redact            enmascarado de secretos
internal/httpx             cliente HTTP/1.1/HTTP/2/HTTP/3, WebSocket, checks y extracción
internal/metrics           muestras, HDR, RPS y snapshots
internal/memguard          vigilancia del RSS y parada por límite de memoria
internal/results           esquema, almacén atómico e índice de corridas
internal/web               servidor local, API JSON y UI embebida
internal/sysmetrics        CPU/RAM/disco/red del generador
internal/engine            VUs, planificador de etapas y parada
internal/report            umbrales, veredicto, diagnóstico y texto
internal/tui               dashboard y reporte Bubble Tea
internal/logbuf            ring de logs con redacción y rate limit
internal/orchestrator      cableado de motor + métricas + TUI/modo plano
examples/demo              servidor objetivo de ejemplo
scripts/e2e.sh             prueba de extremo a extremo
```

Flujo simplificado:

```text
YAML → validación → plan compilado → VUs concurrentes → lotes de muestras
     → agregador → snapshots → TUI / modo texto → reporte + veredicto
```

Decisiones de diseño:

- Una goroutine por VU, con cookie jar y variables propias.
- Un transporte compartido para keep-alive: `http.Transport` para HTTP/1.1/HTTP/2 y `http3.Transport` para HTTP/3, más un transporte HTTP/1.1 separado para WebSocket.
- Un único agregador dueño del estado de métricas (sin locks por petición).
- La UI consume snapshots inmutables; no conoce el motor.
- El reporte se serializa a JSON versionado y se persiste por corrida.
- Diagnóstico por reglas explícitas y testeables.

## Desarrollo y pruebas

### Comandos

| Comando | Qué hace |
|---|---|
| `make build` | Compila `bin/siafu`. |
| `make build-all` | Compila Linux/macOS × amd64/arm64. |
| `make install` | Instala con `go install`. |
| `make test` | `go test ./...`. |
| `make race` | `go test -race ./...`. |
| `make lint` | `go vet` + comprobación de `gofmt`. |
| `make check` | lint + race; la puerta de calidad. |
| `make vuln` | `govulncheck` sobre el código fuente y el binario. |
| `make coverage` | Cobertura y total. |
| `make demo` | Compila y ejecuta el servidor demo. |
| `make demo-build` | Compila solo `bin/siafu-demo`. |
| `make e2e` | Demo + binario + PASS/FAIL de extremo a extremo (cerrado y open-loop). |
| `make tui-smoke` | Prueba PTY de la TUI: teclas tempranas, `Ctrl+D` y `Q`. |
| `make web-smoke` | Guarda una corrida, levanta `serve` y valida API/assets. |
| `make preflight` | Comprueba `ulimit`, puertos efímeros, backlog y conntrack. |
| `make stress` | Estrés cerrado de 15 000 VUs contra el demo. |
| `make stress-open` | Estrés open-loop de 1k a 20k iteraciones/s contra el demo. |
| `make clean` | Borra `bin/` y `coverage.out`. |

Comprobación de navegador **dev-only** (no entra en `make check`):

```bash
npm install playwright
npx playwright install firefox
make build
node scripts/web-browser-check.js
```

### Estrategia de pruebas

- **Unitarias:** configuración, plantillas, checks, extracción, redacción, histogramas, umbrales y diagnóstico.
- **Integración HTTP:** `httptest` con latencia, 429/500, timeouts, TLS, redirecciones, cuerpos truncados y cookies.
- **Motor:** rampas, parada ordenada, aborto, 500 VUs y ausencia de deadlocks.
- **Open-loop:** interpolación de tasa, precisión ±5 % en meseta, tope de in-flight, descartes, parada acotada y presupuesto de análisis.
- **Estrés opt-in:** 15 000 VUs cerrados, 1M de peticiones open-loop y 8 000 in-flight (`SIAFU_STRESS=1`).
- **Persistencia:** esquema JSON, guardado atómico, índice, consultas y path traversal.
- **Web:** API con `httptest`, assets sin URLs externas y smoke de `serve`.
- **TUI:** `Update`/`View` sin PTY, a 120×40, 80×24 y 60×20.
- **E2E:** binario real contra el servidor demo, con configuración que aprueba y que falla.

### Añadir código

- Mantén la separación `internal/` y la dirección de dependencias.
- Añade una prueba que falle antes de implementar.
- Ejecuta `make check` antes de un commit.
- Usa cabeceras SPDX en archivos nuevos.
- Los commits siguen Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).

## Limitaciones conocidas

- **HTTP/1.1, HTTP/2 y HTTP/3 sobre HTTPS, más WebSocket sobre HTTP/1.1.** No hay h2c, gRPC ni WebSocket sobre HTTP/2 extendido todavía.
- **Sin navegador.** No se ejecuta JavaScript ni se descargan recursos del DOM.
- **Sin exportación CSV.** Hay historial JSON y comparación de corridas, pero no exportación CSV/Parquet todavía.
- **Sin carga distribuida.** Todos los VUs salen de la máquina local.
- **Web local, sin autenticación.** `siafu serve` escucha en loopback y es de solo lectura; no está pensado para exposición pública.
- **Sin SQLite en v1.** El historial usa `index.jsonl`; SQLite queda como mejora futura.
- **Sesión por worker en open-loop.** En v1 cada worker mantiene su propia sesión; no hay un almacén de usuarios únicos entre iteraciones.
- **Punto de fallo open-loop por tasa de error.** El p95 por etapa no se desglosa; el p95 del reporte es global.
- **`on_analysis_exhausted: skip`** puede omitir `body_contains`/`extract`; el reporte muestra los pasos omitidos y un aviso.
- **Sin reintentos automáticos.** Un fallo de red cuenta como fallo; no se reintenta.
- **Sin pesos por escenario.** El reparto es round-robin.
- **Sin mTLS.** Solo CA personalizada e `insecure_skip_verify`.
- **Windows no soportado** en v0.1.
- **Límites de cuerpo:** 1 MiB para análisis, 10 MiB para descarte.
- **Límites de carga:** 2 000 VUs por defecto, 20 000 máximo, 6 horas; open-loop 8 000 in-flight por defecto en el perfil, 20 000 máximo y 1024 MiB de presupuesto.
- **`ca_file`** define el pool de confianza del destino; en v0.1 no se combina con el pool del sistema.
- Las métricas de host se muestran como `0` si una métrica no está disponible; no abortan la prueba.

## Solución de problemas

### La prueba termina con código 130 y `ABORTADA`

Detuviste la prueba con `Ctrl+D` o una señal, o el contexto se canceló. Es el comportamiento esperado para una ejecución incompleta.

### Código 3 y cero peticiones

Siafu no pudo completar ninguna petición. Revisa:

- que `base_url` sea correcta y accesible;
- DNS, firewall y proxy;
- que el servidor acepte el protocolo configurado en `target.http_version`;
- que `timeout` no sea demasiado bajo;
- los logs de stderr.

### Código 2 y configuración rechazada

El mensaje de admisión muestra el valor configurado, el máximo permitido, el exceso y la acción correctiva. Ajusta `load.max_vus` o aumenta `limits.memory_budget_mb`; no se ejecutó ninguna petición.

### Código 3 por límite de memoria

Siafu detuvo la prueba porque el RSS superó `limits.memory_budget_mb` en dos muestras consecutivas. Reduce `load.max_vus`, reduce `limits.analysis_budget_mb` o aumenta `limits.memory_budget_mb` (máximo 1024). Para 15 000 VUs usa `memory_budget_mb: 1024`.

### Error de TLS con un certificado autofirmado

Para un entorno controlado:

```yaml
target:
  tls:
    insecure_skip_verify: true
```

O indica la CA:

```yaml
target:
  tls:
    ca_file: "./certificados/ca.pem"
```

### “variable de entorno ... no definida”

Exporta la variable antes de ejecutar:

```bash
export LOAD_PASSWORD="..."
```

`validate` no resuelve variables de entorno; el error aparece al ejecutar.

### El destino responde 429

Está aplicando rate limiting. Reduce VUs o la pendiente de la rampa, o coordina con el equipo del servicio. El diagnóstico lo reportará como causa probable.

### La latencia es alta pero el servidor está sano

Comprueba la tarjeta **GENERADOR** y el resumen de host. Si la CPU local supera el 85–95 %, la máquina de pruebas puede ser el cuello de botella. Ejecuta desde otra máquina o reduce el logging.

### `body_too_large`

La respuesta supera el límite de análisis (1 MiB) o de descarte (10 MiB). Siafu cierra la conexión y marca la petición como fallida. Ajusta el endpoint o evita `body_contains`/`extract` en respuestas grandes.

### No veo la TUI

La salida no es una TTY o pasaste `--no-tui`. Usa una terminal interactiva o revisa el modo texto.

### Quiero ver más detalle

Ejecuta con `--debug` y revisa stderr. La TUI conserva las últimas 1 000 líneas de log; `X` las muestra a pantalla completa.

## Seguridad

- **Autorización:** no ejecutes pruebas contra sistemas de terceros sin permiso. La carga puede causar indisponibilidad.
- **Secretos:** usa `${ENV:...}` para credenciales. No las escribas en el YAML versionado.
- **Logs:** Siafu no registra cuerpos de petición/respuesta y enmascara cabeceras, query sensible y valores de entorno referenciados.
- **Superficie de red:** el binario se conecta a las URLs que declares (incluidas absolutas), a las redirecciones y al proxy del entorno.
- **Límites:** los topes de VUs, in-flight, duración, memoria y cuerpo existen para evitar accidentes, no para sustituir tu criterio.
- **Dependencias:** las librerías usadas son compatibles con AGPLv3 (MIT/BSD/Apache-2.0). `make vuln` ejecuta `govulncheck` sobre fuente y binario; CI lo hace en cada PR y a diario.
- **Reporte de vulnerabilidades:** sigue [SECURITY.md](SECURITY.md); no abras un issue público con detalles de explotación.
- **AGPL y red:** si modificas Siafu y lo ofreces como servicio en red, la sección 13 de la AGPL exige ofrecer el código correspondiente a los usuarios.

## Roadmap

Ideas previstas para versiones posteriores, sin compromiso de orden:

- exportación CSV/Parquet y comparación avanzada entre ejecuciones;
- escenarios con pesos;
- tienda de sesiones por usuario para open-loop;
- auto-tuning de la tasa objetivo y p95 por etapa;
- gRPC;
- ejecución distribuida;
- mTLS y auth flows más ricos;
- más checks (JSONPath, cabeceras, esquemas);
- perfiles y plantillas reutilizables;
- integración con Prometheus/Grafana y artefactos de CI.

## Contribuir

Lee [CONTRIBUTING.md](CONTRIBUTING.md) y el [código de conducta](CODE_OF_CONDUCT.md). En resumen:

1. Abre un issue describiendo el caso de uso o el fallo.
2. Haz un fork y crea una rama (`feat/...` o `fix/...`).
3. Sigue TDD: escribe primero el test que falla y luego el fix.
4. Ejecuta `make check`, `make e2e` y, si tocas dependencias, `make vuln`.
5. Envía un Pull Request con una descripción clara y el razonamiento de las decisiones.

No se aceptan contribuciones incompatibles con AGPL-3.0-or-later. Los archivos nuevos deben incluir:

```go
// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors
```

## Licencia

Siafu se distribuye bajo la **GNU Affero General Public License, versión 3 o posterior** (`AGPL-3.0-or-later`).

- Texto completo: [`LICENSE`](LICENSE)
- Licencia oficial: <https://www.gnu.org/licenses/agpl-3.0.html>

En resumen: puedes usar, estudiar, modificar y distribuir Siafu, pero si distribuyes una versión modificada o la ofreces como servicio en red, debes poner el código correspondiente a disposición de los usuarios bajo la misma licencia. No hay garantía.

---

Hecho con Go, Bubble Tea, Lipgloss, gopsutil y HDR Histogram.
