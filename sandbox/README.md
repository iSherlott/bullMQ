# sandbox (process)

This package provides a BullMQ-like process sandbox primitive for Go.

## Parent side
- `Child`, `ChildPool`

## Child side
Build a child binary that calls `sandbox.RunChild`.

Protocol: JSON objects on stdin/stdout. Logs must go to stderr.
