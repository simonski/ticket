# Privacy Policy

**Project:** ticket  
**Version:** 0.1.848  
**Last updated:** 2026-04-26

This document describes what personal data the **ticket** application collects,
how it is used, how long it is retained, and what rights users have over their
data. It applies to self-hosted deployments of the `ticket` server.

> **Note for self-hosters:** You are the data controller. This document is a
> template and starting point. Customise it for your organisation before
> publishing to end users.

---

## Contents

1. [Data categories collected](#data-categories-collected)
2. [Processing purposes and legal basis](#processing-purposes-and-legal-basis)
3. [Retention periods](#retention-periods)
4. [Data storage and security](#data-storage-and-security)
5. [Third-party sharing](#third-party-sharing)
6. [Subject rights](#subject-rights)
7. [Contact](#contact)

---

## Data categories collected

| Category | Examples | Storage location |
|----------|----------|-----------------|
| Account credentials | Username, hashed password (Argon2id — irreversible), email address | SQLite database |
| Session tokens | Random 32-byte tokens, not linked to browser fingerprint | SQLite `sessions` table |
| Ticket content | Titles, descriptions, comments, time entries, labels | SQLite `tickets`, `comments`, `time_entries` tables |
| Audit events | Who created/changed/archived a ticket and when | SQLite `ticket_history` table |
| UI preferences | Theme selection, expanded epics, panel widths | Browser `localStorage` (client-only, not sent to server) |

**Not collected:**
- Analytics, page-view tracking, or telemetry of any kind
- IP addresses (beyond what appears in server logs, if enabled)
- Third-party cookies or tracking pixels

---

## Processing purposes and legal basis

| Purpose | Legal basis (GDPR Art. 6) | Data used |
|---------|--------------------------|-----------|
| User authentication | Legitimate interest / contract performance | Username, password hash, session token |
| Issue tracking | Contract performance | Ticket content, comments, time entries |
| Audit trail | Legitimate interest (accountability) | History events, created_by references |
| Email notifications (if configured) | Legitimate interest / consent | Email address |

---

## Retention periods

| Data type | Default retention | Configurable? |
|-----------|-------------------|---------------|
| Active sessions | 30 days from creation, then auto-expired | Yes — `TICKET_SESSION_EXPIRY_DAYS` env var |
| Inactive sessions (not used) | Purged on server restart / retention sweep | Yes |
| Ticket history events | Indefinite | Yes — `TICKET_HISTORY_RETENTION_DAYS` env var |
| Deleted user account data | Removed / anonymised on account deletion | No — immediate on delete |
| Browser localStorage | Until cleared by the user | — |

To configure retention, set environment variables before starting the server:

```bash
TICKET_SESSION_EXPIRY_DAYS=30
TICKET_HISTORY_RETENTION_DAYS=365
```

---

## Data storage and security

- **Passwords**: hashed with Argon2id (64 MB memory, 4 iterations, 32-byte
  salt). The plaintext password is never stored.
- **Session tokens**: 32 cryptographically random bytes, base64url-encoded.
  Sessions expire after 30 days and are checked on every request.
- **Email field**: optionally encrypted at rest with AES-256-GCM when
  `TICKET_ENCRYPTION_KEY` is set. For production deployments, this environment
  variable **should** be set.
- **Database at rest**: SQLite file stored at `$TICKET_HOME/ticket.db`. For
  production deployments, the disk volume should use full-disk encryption
  (e.g. LUKS on Linux, FileVault on macOS, dm-crypt in Docker).
- **In transit**: enable TLS at the reverse proxy layer (nginx, Caddy, or
  Traefik). The `ticket` server itself serves plain HTTP and should not be
  exposed directly to the internet.
- **Cookies**: `HttpOnly`, `Secure` (when served over HTTPS), `SameSite=Lax`.

---

## Third-party sharing

**No personal data is shared with any third party by default.**

If you configure an LLM agent (e.g. Claude or OpenAI Codex), ticket titles
and descriptions may be sent to that provider's API. Review the LLM
provider's privacy policy before enabling agent features on production data.

---

## Subject rights

Under GDPR (and similar legislation), users have the following rights:

| Right | How to exercise |
|-------|----------------|
| **Access** (Art. 15) | Request a copy of your data from the server administrator |
| **Rectification** (Art. 16) | Update your profile via the web UI or CLI |
| **Erasure** (Art. 17) | Ask the administrator to run `tk user rm -username <name>` — this deletes your account, sessions, memberships, time entries, and messages, and anonymises audit trail references |
| **Portability** (Art. 20) | The administrator can run `tk export -o account-data.json` to produce a JSON snapshot; request the subset relating to your account |
| **Restriction / Objection** (Arts. 18, 21) | Contact the server administrator |

To exercise any right, contact the person or organisation who operates your
`ticket` instance. This is typically your employer or team lead.

---

## Contact

For self-hosted deployments, the data controller is the organisation or
individual operating the server. Replace this section with your organisation's
contact details before publishing.

For questions about the open-source project itself (not personal data in a
deployment), open an issue at https://github.com/simonski/ticket.
