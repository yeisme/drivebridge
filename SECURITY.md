# Security policy

DriveBridge handles private files and Agent credentials. Report suspected
vulnerabilities privately to the repository owner before opening a public
issue. Do not include credentials, private URLs, file contents or exploit
payloads in public reports.

The current pre-1.0 release is a local/self-hosted candidate. It is not a
claim of production cloud security or multi-tenant readiness. Presigned object
URLs, generic rclone, remote mode and Scaena delivery remain experimental until
the support matrix says otherwise.

Security work is tested against CSRF/Origin checks, path and SSRF boundaries,
quota enforcement, session and grant revocation, share expiry/revocation,
object lifecycle, cryptographic randomness and media-worker isolation.
