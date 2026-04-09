## 2026-04-02 - Log Injection Prevention
**Vulnerability:** Log injection via unsanitized URL paths.
**Learning:** Middleware functions in Go (standard HTTP or Gin) frequently log request details. If paths containing newlines are logged directly using `log.Printf` without sanitization, an attacker can inject arbitrary log entries.
**Prevention:** Sanitize inputs (e.g., replace or remove `\n` and `\r`) before writing them to logs.

## 2026-04-06 - Error Information Disclosure Prevention
**Vulnerability:** Information disclosure via raw error messages.
**Learning:** The API returned raw `err.Error()` responses directly to the client via JSON for all errors (including 500 Internal Server Errors). This could leak sensitive internal structure details like DB queries or stack traces.
**Prevention:** Map server errors (HTTP 5xx) to generic error strings in centralized API response utilities like `writeJSON`.

## 2026-04-09 - SSRF Prevention via Dialer Control
**Vulnerability:** Server-Side Request Forgery (SSRF) allowed the crawler to access internal network services and loopback addresses.
**Learning:** Overriding `DialContext` completely to do manual DNS resolution breaks Go's native connection mechanisms like Happy Eyeballs (concurrent IPv4/IPv6 dialing) and drops dialer timeout configurations.
**Prevention:** Implement `net.Dialer.Control` which allows interception and validation of resolved IP addresses just before connection, preserving native standard library connection features and dual-stack resilience.
