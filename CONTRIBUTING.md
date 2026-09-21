# Contributing to Byakugan Core

Thank you for contributing. Keep changes within the headless public API and
engine scope described in `README.md`.

Before opening a pull request:

```sh
test -z "$(gofmt -l .)"
go mod verify
go vet ./...
go test -mod=readonly ./...
```

Add focused tests for behavior changes and update the API documentation when a
request or response changes. Keep the module dependency-light. A new dependency
must have a compatible license and an entry in `THIRD_PARTY_NOTICES.md`.

Do not commit credentials, customer data, payment data, private keys, local
environment files, or generated binaries. Report suspected vulnerabilities by
following `SECURITY.md`, not through a public issue.

By submitting a contribution, you agree that it may be distributed under the
Apache License 2.0 in this repository. Confirm that you have the right to submit
the work. All commits must include the contributor's Developer Certificate of
Origin sign-off:

```text
Signed-off-by: Your Name <you@example.com>
```

Use `git commit -s` to add the sign-off. Maintainers may request smaller commits
or additional evidence before merging.
