# Third-party licenses

This project is licensed under the MIT License (see `LICENSE`). It links to
and vendors (via the Go module cache) third-party packages with their own
terms. You are responsible for complying with those terms when you build,
modify, or redistribute this software.

Notable direct dependencies:

| Module | SPDX | Notes |
|--------|------|--------|
| [go.mau.fi/whatsmeow](https://github.com/tulir/whatsmeow) | **MPL-2.0** | WhatsApp Web multidevice client; core protocol implementation. |
| [google.golang.org/protobuf](https://github.com/protocolbuffers/protobuf-go) | BSD-3-Clause | Protocol buffers runtime. |
| [golang.org/x/time](https://cs.opensource.google/go/x/time) | BSD-3-Clause | Rate limiting. |
| [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) | MIT | SQLite driver (CGO). |
| [github.com/mdp/qrterminal/v3](https://github.com/mdp/qrterminal) | MIT | QR code terminal rendering. |

Run the following for a full dependency listing (including transitive modules):

```bash
go list -m -json all | jq -r '.Path + " " + (.License // "unknown")'
```

(Requires `jq`; license metadata depends on your Go version and module cache.)
