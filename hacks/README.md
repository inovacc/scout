# hacks/

Test tools, debug utilities, and development helpers for the Scout project.

These are **not** part of the production build. They exist for manual testing, debugging, and exploratory work during development.

## Contents

| File | Purpose |
|------|---------|
| (add tools here) | |

## Usage

Run any tool directly:
```bash
go run ./hacks/tool-name/
```

## Guidelines

- One directory per tool (e.g., `hacks/ping-test/main.go`)
- Tools should be self-contained and not imported by production code
- Add a brief comment at the top of each tool explaining what it does
- No need for tests — these are throwaway utilities
