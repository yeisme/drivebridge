# DriveBridge threat model

## Assets

- File bytes, fixed versions, object keys and stable file references.
- User sessions, UserKey credentials, Agent grants and handoff grants.
- Share links, receiver sessions and download records.
- Instance key, per-space data keys and encrypted metadata.

## Trust boundaries

1. Browser and CLI callers to the DriveBridge HTTP service.
2. DriveBridge service to SQLite/GORM or PostgreSQL metadata.
3. DriveBridge service to S3-compatible object storage and OneDrive.
4. DriveBridge preview worker to uploaded media.
5. DriveBridge desktop host to the Pinax owner process.

## Required controls

- Origin and CSRF checks for browser mutations.
- Owner, tenant, space, prefix and action checks on every file or Agent path.
- Canonical path validation and SSRF/redirect rejection for provider requests.
- Quota, body-size, timeout and concurrency budgets for uploads, downloads and previews.
- Revocable server-side download capabilities; provider URLs are experimental.
- Transactional object reference accounting and GC rechecks.
- Checked cryptographic randomness, AEAD error propagation and separate instance-key backup.
- Central user freeze/disable/archive revocation for sessions, keys, grants and shares.
- Sandboxed media conversion with no network and bounded resources.

The current pre-1.0 release is a local/self-hosted candidate. Real cloud,
multi-tenant and production deployment claims require the corresponding
release gate evidence.
