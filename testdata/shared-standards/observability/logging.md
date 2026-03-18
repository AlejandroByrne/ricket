---
title: Logging Standards
tags: [standard, logging, observability]
---

# Logging Standards

All services must follow structured logging using JSON format.

## Levels

- **ERROR** — unrecoverable failures
- **WARN** — degraded but functional
- **INFO** — significant business events
- **DEBUG** — diagnostic detail (never in production)
