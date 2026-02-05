# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in sacha, please report it responsibly.

**Do not open a public issue.**

Instead, please email security concerns to the maintainers via [GitHub Security Advisories](https://github.com/Sachamama/sacha/security/advisories/new).

We will acknowledge receipt within 48 hours and aim to provide a fix or mitigation plan within 7 days.

## Scope

Sacha is a read-oriented TUI that interacts with AWS APIs using your local credentials. Security concerns include:

- Credential exposure (e.g., logging or displaying secrets)
- Command injection via user input
- Unexpected data exfiltration
- Dependency vulnerabilities
