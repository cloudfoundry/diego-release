---
title: Design Tenets
expires_at : never
tags: [diego-release, cfdot]
---

## Design Tenets

- Execution is stateless: configuration is specified either as flags or as environment variables.
- Conform to UNIX conventions of successful output on stdout and error messages on stderr.
- For BBS API commands, output is a stream of JSON values, one per line, optimal for processing with `jq` and suitable for processing with `bash` and other line-based UNIX utilities.
