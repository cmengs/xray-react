Collector changes (feat/collector-poc)

This branch introduces two related improvements to the collector:

1) Scoped API token middleware
- The collector now supports a scoped API token middleware that only protects specific routes.
- By default the middleware protects the /ingest and /api prefixes. The token is read from the
  COLLECTOR_API_TOKEN environment variable and is expected to be sent as a Bearer token in the
  Authorization header (e.g. Authorization: Bearer <token>).
- If COLLECTOR_API_TOKEN is not set the middleware will allow requests through (useful for
  local testing). NOTE: For production, always set the environment variable.

Example (curl):

  curl -X POST http://localhost:8080/ingest -H "Authorization: Bearer $COLLECTOR_API_TOKEN" -d '{"event":"hit"}'

2) connection_id tracking
- Each ingest request now receives a generated connection_id (UUID). The collector tracks active connections
  in-memory using a mutex-protected map and exposes the active connections count at /api/active_connections.
- The connection_id is also recorded with saved events in storage so you can trace events back to a connection.
- The ingest handler returns the connection id in the X-Connection-Id response header.

3) Storage schema update (sqlite)
- The SQLite events table has been updated to include a connection_id TEXT column.
- Migration: the code ensures the events table exists with the connection_id column on startup (CREATE TABLE IF NOT EXISTS).

Notes
- These changes intentionally scope the token check only to routes that handle ingestion and API operations. Public/non-sensitive routes can remain unprotected.
- Review environment configuration for COLLECTOR_API_TOKEN before deploying to production.

