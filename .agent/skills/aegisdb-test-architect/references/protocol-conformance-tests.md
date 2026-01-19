# Protocol Conformance Tests

## Purpose
Ensure REST, gRPC, MCP, and A2A adhere to contract semantics.

## General rules
- Validate status codes, error structures, and headers.
- Assert schema shape and field semantics, not just presence.
- Keep golden data stable; normalize timestamps and IDs.
- Use deterministic clocks.

## REST
- Use httptest servers.
- Verify pagination, filtering, and error responses.
- Check content-type and request validation.

## gRPC
- Use bufconn for in-memory transport.
- Validate codes and status details.
- Test streaming behavior when applicable.

## MCP and A2A
- Validate subscription flows and telemetry payloads.
- Ensure heartbeat and metrics reflect server state.
- Use timeouts for waits and cleanup.

## AegisDB focus areas
- Wormhole telemetry parity across REST, gRPC, MCP, and A2A
- Schema and stats resources in MCP

## Pitfalls
- Avoid brittle ordering assumptions in JSON or maps.
- Avoid relying on wall clock time.
