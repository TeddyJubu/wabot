# Contributing

Thanks for helping improve wabot.

- Open an issue first for larger changes (reconnect semantics, API shape, Docker layout).
- Run `go fmt ./...`, `go test ./...`, and `go vet ./...` before sending a PR.
- Keep the HTTP API backward compatible when possible; document breaking changes in `CHANGELOG.md`.
- Do not commit `wabot.env`, `store.db`, or `sends.log`.

## License

By contributing, you agree your contributions are licensed under the MIT License
(see `LICENSE`). Third-party dependencies (notably **whatsmeow**, MPL-2.0) keep
their own licenses; see `THIRD_PARTY.md`.
