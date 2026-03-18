---
title: API Naming Standards
tags: [standard, api, naming]
---

# API Naming Standards

All REST API endpoints must use kebab-case for URL paths and camelCase for JSON fields.

## Rules

1. Use plural nouns for collections: `/api/users`, `/api/projects`
2. Use kebab-case: `/api/user-profiles`, not `/api/userProfiles`
3. JSON fields use camelCase: `firstName`, `lastName`
4. No verbs in URLs — use HTTP methods instead
