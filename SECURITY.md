# Security Policy

## Supported versions

Security fixes target the latest `0.2.x` release. Older versions may not receive fixes.

## Reporting a vulnerability

Please use the repository's **private security advisory** feature (Security → Advisories → Report a vulnerability). Do not open a public issue with exploit details.

Include:

- the version or commit you tested;
- the affected component (CLI, results store, web UI, TUI, HTTP client);
- the impact and the minimum steps to reproduce;
- whether the issue requires a local user, a malicious YAML, or a shared results directory;
- any proposed fix or test.

Do not test against third-party systems without explicit permission. A load test can cause an outage.

## Scope

In scope:

- Siafu source code and build/release scripts;
- CLI flags, menu/TUI, result store, local web UI and HTTP client;
- secret redaction, permission handling and path validation.

Out of scope:

- vulnerabilities in third-party targets you choose to test;
- the demo server (`examples/demo`), which is not production software;
- `--web-dir` and other development-only conveniences;
- social engineering, physical access, or denial of service against shared infrastructure.

## Local web UI

`siafu serve` is read-only, has no authentication, and listens on loopback by default. Do not expose it to untrusted networks. The interactive menu always binds to `127.0.0.1`; the CLI warns when a non-loopback address is requested.

## Safe harbor

We will not pursue legal action for good-faith research that respects this policy, avoids privacy violations and service disruption, and gives us reasonable time to fix the issue before disclosure.
