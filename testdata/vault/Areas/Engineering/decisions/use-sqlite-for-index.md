---
title: use-sqlite-for-index
date: 2026-03-01
tags: [decision, acme]
status: active
---

# use-sqlite-for-index

## Decision
Use SQLite (via modernc.org/sqlite pure-Go driver) for the ricket search index.

## Rationale
- Zero external dependencies — ships inside the binary
- FTS5 available if needed later
- Single file at .ricket/index.db — easy to delete and rebuild

## Consequences
- Must rebuild index on first run and after vault modifications
- No concurrent write access (acceptable — one MCP server per vault)

## Alternatives Considered
- Bleve: pure Go but heavy dependency tree
- PostgreSQL: overkill, requires external process

## Links
- [[ricket-go-rewrite]]
