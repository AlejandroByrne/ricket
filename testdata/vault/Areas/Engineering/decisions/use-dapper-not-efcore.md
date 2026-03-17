---
title: use-dapper-not-efcore
date: 2026-01-15
tags: [decision, acme]
status: active
---

# use-dapper-not-efcore

## Decision
Use Dapper for all database access. Do not introduce EF Core.

## Rationale
- We need full control over SQL for performance tuning
- EF Core migrations have caused prod incidents in the past
- Dapper is battle-tested and predictable

## Consequences
- All queries are raw SQL — developers must know SQL well
- No auto-migrations — schema changes are explicit scripts

## Links
- [[sql-server-indexing]]
