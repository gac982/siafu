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

## Repository mirror

There are two checkouts with different module paths:

- the public repository, `github.com/gac982/siafu`;
- a private mirror, `github.com/proyect-octopus/siafu`.

Their `.git` histories are intentionally independent: the private mirror keeps
early `doc/` commits that must not be published. Do not add the public remote to
the private mirror, do not push its refs, and never copy a `.git` directory
between them.

Keep tracked content in sync. After applying a change in one checkout, port it
to the other and verify:

```bash
scripts/check-parity.sh /path/to/the/other/checkout
```

The script archives `HEAD` from both repositories, normalizes the two module
paths, and fails when the trees differ. `git archive` only includes tracked
files, so ignored directories such as `doc/` are never compared or copied.

## Security and secrets

- Never commit tokens, passwords, private keys, `.env` files or customer data.
- Use `${ENV:...}` for secrets in test configurations.
- Report vulnerabilities privately according to [SECURITY.md](SECURITY.md); do not open a public issue with exploit details.
- Do not run load tests against systems you do not own or have explicit permission to test.

## License

Contributions must be compatible with AGPL-3.0-or-later. By contributing, you agree that your changes are distributed under the same license.
