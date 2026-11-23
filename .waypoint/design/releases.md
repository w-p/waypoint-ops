# Design: releases and CI

**Status:** 2026-11-22, asked for by the developer ("set up releases… with GitHub CI").

- **CI** (`.github/workflows/ci.yml`): on every push to `main` and every pull request, check
  formatting, `go vet`, `go test -race`, and build for every release target.
- **Releases** (`.github/workflows/release.yml`): pushing a tag `vX.Y.Z` runs the tests, builds
  `waypoint-ops` for Linux and macOS on amd64 and arm64 (static, `CGO_ENABLED=0`, version set
  with `-ldflags -X main.version`), packs each with the README and changelog as
  `waypoint-ops_X.Y.Z_<os>_<arch>.tar.gz`, writes `checksums.txt`, and publishes a GitHub release
  whose notes are the tag's section of `CHANGELOG.md`. A tag with no changelog section fails.
- **Only GitHub's own actions** (`actions/checkout`, `actions/setup-go`) and the `gh` command on
  the runner; no release tool to install or trust. GoReleaser would add packaging (Homebrew,
  deb) when someone needs it.
- **No Windows build:** the process check uses `syscall.Kill`, and the app's home is WSL. A
  Windows build needs a Windows process check first.
- **Versioning:** semantic versions from `v0.1.0`; `0.x` while the design is provisional.
  `waypoint-ops -version` prints it (`dev` for local builds).
