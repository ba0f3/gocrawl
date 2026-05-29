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
## 2024-05-18 - SSRF Protection Bypass via Unparseable IPs
**Vulnerability:** The custom HTTP transport (`SafeTransport`) designed to block private IPs for SSRF protection failed to catch bypasses using IPv6 zone identifiers (e.g., `fe80::1%en0`).
**Learning:** `net.ParseIP` returns `nil` for valid IPs that contain zone identifiers. The original `DialContext` override resolved the IP list manually but the standard library `net.Dialer` could fallback to the original string, resolving it internally but bypassing the application layer checks.
**Prevention:** Rather than overriding `DialContext`, hook directly into `net.Dialer.Control`. When `net.Dialer` calls `Control` just before dialing, `net.ParseIP` will return `nil` for unparseable IPs (like those with zone identifiers). Rejecting connections where the IP cannot be strictly parsed prevents these filter bypasses entirely.

## 2026-04-21 - Missing SSRF Protection in External Clients
**Vulnerability:** External HTTP client in the LLM summarizer was vulnerable to SSRF.
**Learning:** While the primary crawler engine used `SafeTransport` to protect against SSRF, secondary external callers (like the LLM summarize feature) used a default `http.Client`. This allowed an attacker to supply a malicious `LLM_BASE_URL` to probe internal networks.
**Prevention:** Apply `utils.SafeTransport()` consistently across all outgoing HTTP clients in the application, not just the primary crawling engine.

## 2026-04-28 - Insecure Direct Object Reference (IDOR) in Crawl Status
**Vulnerability:** The `/v1/crawl/{id}` endpoint did not verify that the requested crawl job belonged to the currently authenticated user.
**Learning:** Returning a resource simply because a user is authenticated and knows the ID is a classic IDOR. Using UUIDs for IDs mitigates the risk but does not resolve the underlying missing authorization check, meaning any user who guessed or obtained an ID could view others' data.
**Prevention:** In multi-tenant applications, every endpoint retrieving user-specific resources MUST verify ownership (e.g., `job.UserID == currentUser.ID`). When an ownership check fails, return a 404 Not Found (rather than 403 Forbidden) to simultaneously prevent data leakage and ID enumeration.

## 2026-05-14 - Missing Security Headers Mitigation
**Vulnerability:** The application API lacked standard HTTP security headers (e.g. `X-Content-Type-Options`, `X-Frame-Options`, `X-XSS-Protection`, `Content-Security-Policy`), leaving browsers without strict guidance on how to securely handle responses, opening vectors for XSS, MIME-sniffing, and clickjacking attacks.
**Learning:** While CORS policies protect cross-origin fetching, they do not inherently protect the application from client-side execution attacks when endpoints are accessed directly.
**Prevention:** Apply a global middleware (e.g., `GinSecurityHeadersMiddleware`) that sets conservative baseline security headers (like `X-Content-Type-Options: nosniff` and `default-src 'self'`) on all API responses.

## 2023-10-27 - Bcrypt Resource Consumption Prevention
**Vulnerability:** Unbounded username and password lengths in authentication endpoints.
**Learning:** bcrypt is highly sensitive to long inputs, and parsing unbounded JSON strings can cause memory exhaustion. Gin's ShouldBindJSON handles constraints automatically.
**Prevention:** Always add maximum length constraints (e.g. max=72 for bcrypt) to user input binding structs.

## 2026-05-26 - Missing JSON Request Body Limits
**Vulnerability:** DoS risk via unbound JSON request bodies in HTTP streaming (SSE) endpoints.
**Learning:** Directly passing unbounded `http.Request.Body` streams into `json.NewDecoder` reads data into memory without an upper limit, leaving the application vulnerable to Denial of Service via massive payload ingestion, especially since standard HTTP middlewares don't restrict stream bodies dynamically.
**Prevention:** Always wrap `http.Request.Body` with `io.LimitReader(req.Body, MaxBytes)` before passing to memory-allocating parsers like `json.NewDecoder`.
## 2025-05-28 - SSRF Vulnerability in Configuration Fetching
**Vulnerability:** The `fetchWebSocketDebuggerURL` function used `http.DefaultClient.Do(req)` to retrieve the WebSocket debugger URL from the configured `LIGHTPANDA_HTTP_URL` without employing the application's standard SSRF protection (`utils.SafeTransport`).
**Learning:** Even HTTP requests directed at internally configured URLs (like those resolving to remote Chrome debugging endpoints) can be vulnerable to Server-Side Request Forgery if the configuration is influenced or if subsequent redirection occurs. The assumption that only user-provided URLs require SSRF protection is a dangerous anti-pattern.
**Prevention:** Always enforce SSRF protections across *all* outgoing HTTP requests, including internal/configuration requests, by consistently using a hardened `http.Client` equipped with `utils.SafeTransport()`. Never default to `http.DefaultClient` or a non-hardened custom client.
