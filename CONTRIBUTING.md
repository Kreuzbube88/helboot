# Contributing to HELBOOT

Thank you for considering a contribution! HELBOOT is developed as a
professional open-source project: architecture quality always takes
precedence over quick implementation.

## Before you start

- Read [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) to understand the
  component model.
- Significant technical decisions require an Architecture Decision Record
  in [docs/adr/](docs/adr/). Propose the ADR in your pull request.
- Check existing issues and pull requests to avoid duplicate work.

## Development setup

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for prerequisites, how to
run the backend and frontend locally, and how to run tests.

## Pull request guidelines

- Keep pull requests focused: one logical change per PR.
- All visible UI text must go through i18n — no hardcoded strings.
  Add new keys to **both** `en` and `de` locale files.
- New operating system support is added as a **provider** with a YAML
  manifest — never as hardcoded logic in core.
- Add or update tests for anything you change. CI must be green.
- Update documentation affected by your change.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add openSUSE autoyast answer file generation
fix: correct ProxyDHCP option 43 encoding for UEFI clients
docs: describe macvlan limitations on Unraid
test: cover profile version cloning
refactor: extract answer-file renderer interface
```

## Code style

- **Go:** `gofmt`/`go vet` clean; follow standard Go project conventions.
- **TypeScript/React:** formatted with the repository ESLint/Prettier
  configuration.
- Follow SOLID, DRY and KISS. No quick fixes, no duplicated logic,
  and complex logic must be commented.

## Reporting security issues

Please do **not** open public issues for vulnerabilities — see
[SECURITY.md](SECURITY.md).

## License

By contributing you agree that your contributions are licensed under the
[AGPL-3.0](LICENSE), the project's existing license.
