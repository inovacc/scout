# Known Issues

## Open Issues

### Taskfile contains inapplicable tasks
- **Severity:** Low
- **Status:** Open
- **Description:** `Taskfile.yml` includes `proto:generate`, `sqlc:generate`, `generate`, `build:dev`, `build:prod`, `run`, and `release` tasks that reference protobuf, sqlc, goreleaser, and `cmd/` artifacts. Scout is a library package with no protobuf, sqlc, goreleaser config, or main package.
- **Workaround:** Use only `task test`, `task check`, `task lint`, `task fmt`, `task vet`, `task deps` tasks.

### Race method does not return matched index
- **Severity:** Medium
- **Status:** Open
- **Description:** `Page.Race()` always returns `-1` as the match index (page.go:478), even though the method signature promises to return the index of the matched selector. The rod `race.Do()` API does not directly expose which selector won.
- **Workaround:** None. Callers cannot determine which selector matched.

### CI build workflow installs unneeded system packages
- **Severity:** Low
- **Status:** Open
- **Description:** `.github/workflows/build.yml` installs X11, Mesa OpenGL, and audio libraries (`xorg-dev`, `libgl1-mesa-dev`, `libasound2-dev`, etc.) on Linux. Scout is a headless browser library that does not require GPU or audio packages.
- **Workaround:** Builds succeed despite the unnecessary packages.

## Resolved Issues

| Issue | Resolution | Date |
|-------|------------|------|
| (none yet) | | |
