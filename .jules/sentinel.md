## 2026-04-02 - Log Injection Prevention
**Vulnerability:** Log injection via unsanitized URL paths.
**Learning:** Middleware functions in Go (standard HTTP or Gin) frequently log request details. If paths containing newlines are logged directly using `log.Printf` without sanitization, an attacker can inject arbitrary log entries.
**Prevention:** Sanitize inputs (e.g., replace or remove `\n` and `\r`) before writing them to logs.

## 2026-04-06 - Error Information Disclosure Prevention
**Vulnerability:** Information disclosure via raw error messages.
**Learning:** The API returned raw `err.Error()` responses directly to the client via JSON for all errors (including 500 Internal Server Errors). This could leak sensitive internal structure details like DB queries or stack traces.
**Prevention:** Map server errors (HTTP 5xx) to generic error strings in centralized API response utilities like `writeJSON`.
## 2026-04-08 - SSRF Protection via Custom Transport
**Vulnerability:** The crawler was vulnerable to Server-Side Request Forgery (SSRF) because it would blindly fetch user-supplied URLs, potentially exposing internal metadata endpoints (e.g., 169.254.169.254) or internal network resources.
**Learning:** Implementing URL validation *before* the request (e.g., using a simple DNS lookup and string check) introduces a Time-of-Check to Time-of-Use (TOCTOU) vulnerability (DNS rebinding) and massive performance regressions if placed in hot loops like link discovery.
**Prevention:** Apply a custom `http.RoundTripper` with an overridden `DialContext` to validate IP addresses at the exact moment the TCP connection is established, preventing DNS rebinding and avoiding synchronous blocking in the main extraction loops.

## 2026-04-13 - [Login Timing Attack]
**Vulnerability:** Timing attack possible in the `Login` function due to returning early if the username does not exist.
**Learning:** `bcrypt.CompareHashAndPassword` is an expensive operation that takes significantly longer than looking up a user in the database. Returning early when the user is not found allows an attacker to enumerate valid usernames based on response times. Also returning a distinct error when the password does not match versus user not found is an information leak issue, they must both return generic "invalid credentials".
**Prevention:** Always perform a dummy bcrypt comparison or ensure the time taken is relatively constant even when the user is not found to prevent username enumeration. Furthermore, return exactly the same generic error string for both cases.
## 2024-05-24 - Prevent SSRF TOCTOU (DNS Rebinding) while Preserving Happy Eyeballs
**Vulnerability:** The codebase had a Server-Side Request Forgery (SSRF) vulnerability that prevented TOCTOU through DNS rebinding but did so by entirely replacing the HTTP connection `DialContext` function with manual IP dialing. This bypassed native mechanisms such as Happy Eyeballs for IPv4/IPv6 fallbacks and potentially impacted connection reliability and proxy logic.
**Learning:** Overriding `DialContext` completely to validate IPs after DNS resolution breaks features implemented inside the native `net.Dialer.DialContext`, like Happy Eyeballs.
**Prevention:** To prevent DNS rebinding correctly and securely while maintaining core dialer functionality, configure `net.Dialer.Control` instead. This intercept hook validates the exact resolved IP immediately prior to the connection being created, effectively preventing TOCTOU.
## 2024-05-24 - ParseIP vs ParseIPAddr in Go
**Vulnerability:** A previously implemented SSRF prevention checked the resolved host using `net.ParseIP`. However, an attacker could supply an IPv6 address with a zone identifier (e.g., `fe80::1%en0`). Go's `net.ParseIP` doesn't handle zone identifiers and returns `nil`. If `isPrivateIP` evaluates `nil` as `false` (or safe), this could allow a bypass for SSRF protections against internal/loopback services by sending `http://[::1%25lo]/`.
**Learning:** `net.ParseIP` returns `nil` for any string that is not strictly a valid IP without a zone ID. You must always explicitly handle the `nil` case as an error or bypass, or securely strip the zone ID if applicable.
**Prevention:** Explicitly check if `ip == nil` when parsing IPs, and reject the connection if parsing fails during an SSRF check.
