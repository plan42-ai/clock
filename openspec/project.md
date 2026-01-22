# Project Context

## Purpose
Provide testable time abstractions for Go code, including fake clocks and helpers that make time-dependent logic deterministic.

## Tech Stack
- Go 1.24
- Standard library time utilities with small helper dependencies
- Test support with testify

## Project Conventions

### Code Style
Use idiomatic Go names and gofmt formatting. `make lint` runs golangci-lint; keep public APIs small and focused on clock abstractions.

### Architecture Patterns
Single Go module exposing interfaces and concrete clock implementations. Fake clocks allow manual advancement while production code can swap in real time sources.

### Testing Strategy
`make test` runs the suite with the race detector. Tests validate parsing helpers and fake clock behaviors to ensure determinism.

### Git Workflow
Standard feature branches merged via PRs. Tag releases with `make tag` to publish versioned modules.

## Domain Context
Consumers should depend on the clock interfaces so they can inject fake time in tests. Avoid introducing global time references that bypass the provided abstractions.

## Important Constraints
- API stability matters for downstream Plan42 libraries; avoid breaking changes without version bumps.
- Keep implementations race-free as they may be used in concurrent code paths.

## External Dependencies
None; purely local library functionality.
