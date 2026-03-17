---
title: ricket-go-rewrite
date: 2026-03-01
tags: [project, acme]
status: active
---

# ricket-go-rewrite

## Goal
Rewrite ricket from TypeScript to Go for a single zero-dependency binary.

## Scope
- Core vault operations (read, search, file, create)
- MCP server over stdio
- CLI (init, serve, status)
- SQLite search index

## Progress
- [x] config package
- [x] vault package (frontmatter, template, moc, index, vault)
- [x] git audit package
- [x] MCP server (all 8 tools)
- [x] CLI with interactive init wizard
- [ ] Integration tests

## Links
- [[use-sqlite-for-index]]
