# Tracked TODOs

This file tracks small, specific TODOs that don't yet justify a full GitHub
issue. When a TODO here grows enough scope or stakeholders, promote it to an
issue and remove it from this list.

## Open

- **Kimi driver hooks** — `internal/driver/kimi_code.go` `InjectHooks` /
  `RemoveHooks` are no-ops. Implement once Kimi publishes a stable hook-config
  schema. Pattern to follow: `internal/hooks` (the Claude Code injector).
  Tracked from `TECH_DEBT_PLAN.md` M8.

## Closed

(Move resolved entries here with a short note and the commit / PR that closed
them, then prune after a release cycle.)
