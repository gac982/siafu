# Contributing to Siafu

Thanks for helping improve Siafu.

## Requirements

- Go 1.27+
- `make`
- For security checks: `govulncheck` (`go install golang.org/x/vuln/cmd/govulncheck@v1.9.0`)

## Workflow

1. Open an issue describing the use case or bug.
2. Fork the repository and create a branch:
   - `feat/<topic>` for features;
   - `fix/<topic>` for fixes;
   - `docs/<topic>` for documentation only.
3. Write a failing test first. Every behavior change must include a test that reproduces the bug or pins the new behavior before the implementation. Refactors may use characterization tests instead.
4. Implement the smallest change that makes the test pass.
5. Run the quality gates:
   ```bash
   make check
   make e2e
   make vuln
   ```
6. Open a Pull Request with the problem, the approach, the tests, and the trade-offs.

## Tests

- Prefer real behavior over mocks; use `httptest` for HTTP and the real results store for persistence.
- Keep tests deterministic and free of network access to third parties.
- Use `t.TempDir()` for filesystem tests.
- New behavior without a test will be rejected.

## Style and commits

- Run `gofmt` (or `make fmt`) before committing.
- Keep commits focused; one topic per branch.
- New files must include:
  ```go
  // SPDX-License-Identifier: AGPL-3.0-or-later
  // Copyright (C) 2026 Siafu contributors
  ```

## Security and secrets

- Never commit tokens, passwords, private keys, `.env` files or customer data.
- Use `${ENV:...}` for secrets in test configurations.
- Report vulnerabilities privately according to [SECURITY.md](SECURITY.md); do not open a public issue with exploit details.
- Do not run load tests against systems you do not own or have explicit permission to test.

## License

Contributions must be compatible with AGPL-3.0-or-later. By contributing, you agree that your changes are distributed under the same license.
