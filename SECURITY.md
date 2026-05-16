# Security Policy

## Supported Versions

Fleet Commander is currently in pre-1.0 development. Only the latest tagged
release on `main` receives security fixes.

| Version | Supported |
| ------- | --------- |
| latest  | ✅ |
| < latest | ❌ |

## Reporting a Vulnerability

Please **do not file a public GitHub issue** for security vulnerabilities.

Report security issues privately by one of:

1. **GitHub private vulnerability reporting** — preferred. Open the
   repository's *Security* tab and choose *Report a vulnerability*.
2. **Email** the maintainer listed in the repository's GitHub profile.

When reporting, please include:

- A description of the issue and the impact.
- Steps to reproduce, or a proof-of-concept.
- The affected version (or commit SHA).
- Any suggested mitigation, if you have one.

We aim to acknowledge reports within **3 business days** and to ship a fix or
mitigation within **30 days** for critical issues. We will credit you in the
release notes unless you ask us not to.

## Known Threat Model

Fleet Commander spawns local AI agents in git worktrees and exposes a local
HTTP server (`fleet hangar`) that can attach to tmux sessions. Specifically:

- The hangar server should bind to `127.0.0.1` by default. If you opt into
  LAN exposure, you must trust every host on that network.
- Agent worktrees execute code from prompts that may include untrusted text
  (e.g., text from issues). Treat agent output as untrusted input until a
  human has reviewed it.
- Squadron context channels (`.fleet/context.json`) are world-readable on the
  local filesystem. Do not paste secrets into them.

If you find a deviation from the above, that is a security issue — please
report it.
