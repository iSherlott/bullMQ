# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project adheres to Semantic Versioning.

## [Unreleased]

## [0.2.0] - 2026-02-27
### Added
- Process sandbox runtime under `sandbox/` (JSON protocol over stdin/stdout; stderr reserved for logs).
- `Consumer.ConsumeSandboxed(...)` to process jobs using an external child process.
- BullMQ-style typed errors (`Waiting`, `WaitingChildren`, `Delayed`, `RateLimit`, `Unrecoverable`) to control consumer flow.
- `AsyncFifoQueue` and `Backoffs` utilities (fixed/exponential + jitter).

### Changed
- Internal code organization: Redis persistence moved to `redis/store.go` and state keys/operations split under `state/`.
