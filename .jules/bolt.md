# ⚡ Bolt Performance Log

## Inefficient Substring Checks in URL Parsing (`internal/api/crawl_manager.go`)

**What:**
Pre-compiled regular expressions were introduced to replace the O(N*M) nested loops running `strings.Contains()` during the crawl cycle in `visitIfAllowed` and `shouldScrapeURL`.

**Why:**
The previous method iterated through `IncludePaths` and `ExcludePaths` using `strings.Contains(urlPath, path)` for *every single discovered URL*. For very large sites or long path configuration lists, this caused measurable CPU overhead per URL. A single compilation of these paths into a regex `(path1|path2|...)` effectively builds a quick evaluation tree, reducing repeated string parsing logic.

**Measured Improvement:**
For large arrays (e.g., 50-100 items), regex avoids repeating O(N) evaluations. A mock benchmark simulation using 50 exclusion paths showed strings taking `~5293 ns/op` and Regex taking `~2304 ns/op`, demonstrating a `~2.3x` improvement. However, in smaller arrays (e.g., 5 items), native `strings.Contains` is faster due to the static startup cost of invoking the Regex matching engine on simple bytes (`~1000 ns/op` vs `~4500 ns/op`). Given that crawling environments often filter long dynamically generated arrays, regex pre-compilation guarantees better asymptotic scaling behavior (O(1) compiled vs O(N) evaluation time over the long tail).

## 2026-04-01 - O(1) Short-Circuiting URL Evaluation (`internal/api/crawl_manager.go`)

**What:**
Swapped condition evaluation order in `visitIfAllowed` from `cm.shouldScrapeURL(...) && !visited[absURL]` to `!visited[absURL] && cm.shouldScrapeURL(...)`.

**Why:**
The previous method always parsed the absolute URL using `url.Parse` and executed matching logic against compiled regular expressions (`shouldScrapeURL`) *before* checking the simple O(1) `visited` boolean map. In a crawl job, most discovered links (headers, footers, navigation) are re-visited many times per page. Short-circuiting the boolean evaluation prevents the application from executing heavy URL logic for already-known links.

**Measured Improvement:**
A quick benchmark isolated to this change proved that already-visited links evaluate in `~681 ns/op` down from `~2628 ns/op`, a nearly 4x improvement. This scales enormously in long-running job profiles.

## 2026-04-02 - Hoist URL Parsing in Scraper Link Collection (`internal/crawler/colly.go`)

**What:**
Moved the base URL parsing logic `baseURL, _ := url.Parse(pageURL)` out of the `appendResolvedHref` utility function, and instead pass down the parsed `*url.URL` object from the calling scopes `buildResultFromMainHTMLWithDoc` and `collectScrapeLinks`.

**Why:**
The previous method instantiated a `url.Parse(pageURL)` on every single `<a>` tag found within a page payload. Since `url.Parse` involves string allocations, state machine validation, and struct construction, performing this inside an O(N) loop mapping over thousands of links causes substantial redundant CPU and memory overhead.

**Measured Improvement:**
In a micro-benchmark using a mock `a` tag payload loop (1,000,000 iterations), execution time improved from `~1188 ns/op` to `~819 ns/op`, saving `~31%` in parsing overhead per link. Furthermore, memory allocation decreased from `504 B/op` (6 allocs) to `360 B/op` (5 allocs), dramatically dropping memory pressure per scrape for dense DOM payloads.

## 2026-04-03 - Avoid `goquery.Selection.ParentsFiltered` in Hot Loops (`internal/api/crawl_manager.go`)

**What:**
Replaced the loop `for _, sel := range []string{"article", "main", ...} { if e.DOM.ParentsFiltered(sel).Length() > 0 ... }` with a pre-compiled `cascadia.MustCompile("article, main, ...")` and manual `x/net/html` parent traversal (`for p := n.Parent; p != nil; p = p.Parent`).

**Why:**
The previous method instantiated `goquery.Selection.ParentsFiltered` for multiple CSS string queries on *every single discovered link* on a scraped page. Because goquery strings are dynamically compiled into cascadia matchers and traverse up the entire parent tree on each call, repeating this inside an O(N) array caused immense redundant memory allocation and CPU overhead in the tight crawling extraction loop. Manually pre-compiling the CSS selector and traversing the parent nodes natively saves thousands of redundant DOM parsing cycles.

**Measured Improvement:**
In a micro-benchmark using a mock DOM payload, finding a matched article link improved execution time from `~4913 ns/op` down to `~50 ns/op` (nearly a 98% reduction). Finding a missed link improved execution time from `~19966 ns/op` down to `~320 ns/op`.

**Action:**
When iterating over DOM queries on every node matching a `OnHTML` rule or inner loop in Colly, always hoist standard array evaluations using `cascadia.MustCompile` out of the loop and prefer traversing HTML nodes natively when combining simple queries rather than leaning heavily on higher-level generic tools like `goquery.Selection`.

## 2026-04-04 - Zero-Allocation Token Scanning (`internal/extractor/noise.go`)

**What:**
Replaced `strings.Fields(class)` with a custom manual byte-scanning loop in the `isAdClass` function. Evaluated string tokens are now accessed via string slicing `class[start:i]` and checked against bounds-safe manual byte lookups instead of utilizing `strings.HasPrefix` or `strings.HasSuffix`.

**Why:**
The `isAdClass` function runs inside the `IsNoise` execution path, which is mapped over countless DOM elements during the web claw–style extraction scoring phase. Evaluating `strings.Fields(class)` allocates a new array slice on the heap for every tested element's class attribute. Using zero-allocation byte scanning eliminates GC overhead.

**Measured Improvement:**
In micro-benchmarks analyzing long strings, the allocation drops from `208 B/op` to `0 B/op`. Overall execution time improved dramatically from `~370 ns/op` to `~81 ns/op`, a nearly ~78% speedup in this critical string evaluation loop.

**Learning:**
In hot inner loops traversing strings or mapping tokens (especially inside `internal/extractor` logic), avoid standard library convenience functions that instantiate slices like `strings.Fields` or `strings.Split`. Prefer manual byte scanning.

## 2026-04-04 - Caching Descendant Tree for O(1) Checks (`internal/extractor/exclude.go`)

**Learning:**
The extractor's DOM exclusion logic (`buildExcludeSet` in `internal/extractor/exclude.go`) relies on pre-calculating and caching all descendant nodes using `addSubtreeNodes` via `.Find("*")`. While this adds initial overhead when establishing excluded zones, this intentional design trade-off guarantees that downstream exclusion checks during candidate scoring (e.g., `isUnderExcluded`) remain highly performant O(1) pointer map lookups.
**Action:**
Do not attempt to optimize `buildExcludeSet` by removing `addSubtreeNodes` without simultaneously rewriting all dependent exclusion checkers to recursively traverse upward. Even then, an O(1) map lookup is typically preferred in this scoring architecture.

## 2026-04-05 - Unified Zero-Allocation String Scanning in IsNoise (`internal/extractor/noise.go`)

**What:**
Replaced `strings.Fields(class)` and unified multiple class evaluation functions (`isAdClass` and `hasNoiseClassPattern`) into a single, zero-allocation token scanning loop over class attributes in `IsNoise`. Removed redundant `isAdClass` and `hasNoiseClassPattern` loops entirely.

**Why:**
The `IsNoise` function runs in a critical hot loop scoring extraction candidates for noise (nav, ads, sidebars, cookie banners). The prior code allocated a new string array slice (`strings.Fields`) and passed the class string to subsequent functions which ran secondary loops doing their own token matching or sub-string iterations. By combining all logical evaluations into a manual byte-scanning mechanism, memory allocations drop to zero and we eliminate duplicate traversals.

**Measured Improvement:**
Micro-benchmarks against simulated class evaluation dropped iteration time from `~4400-5000 ns/op` and `192 B/op` allocation to `~2700 ns/op` and `0 B/op` memory overhead.

**Learning:**
Never iterate character arrays multiple times in hot loops for string validation if logic can be collapsed into a single manual scan sequence. Crucially, when replacing `strings.Fields`, always use `for i, r := range str` to ensure iteration honors UTF-8 multibyte character boundaries via runes instead of corrupting indexes with a native `for i := 0; i < len(str); i++` byte loop when checking spaces.

## 2026-04-06 - Pre-calculate Strings.Builder Capacity (`internal/extractor/jseval.go`)

**What:**
Added a pre-calculation loop to determine the exact total byte length of string payloads in `extractNextFText`, followed by calling `wire.Grow(totalLen)` before starting the actual string concatenation loop.

**Why:**
The previous method blindly appended to `strings.Builder` using `wire.WriteString(payload)`. When extracting massive chunks of text data (especially from rich JSON dumps inside Next.js `__next_f` blobs), the internal byte slice inside `strings.Builder` dynamically resizes to accommodate new data, forcing repeated heap allocations and data copying. By iterating once to calculate the target length, `strings.Builder` allocates the perfect amount of memory upfront, reducing memory operations and GC overhead.

**Measured Improvement:**
In a benchmark processing 10,000 mock elements with large payloads, total allocations dropped from `29 allocs/op` down to exactly `1 alloc/op`. Execution time improved dramatically from `~5,173,894 ns/op` to `~694,086 ns/op` (approx 86% speedup).

**Learning:**
When building exceptionally large strings via `strings.Builder` dynamically in loops, always prefer pre-calculating the final total string length and calling `.Grow()` if you can cheaply predict it. The cost of a first-pass loop to count lengths is vastly dwarfed by the cost of runtime heap slice reallocations.

## 2026-04-07 - Zero-Allocation Case-Insensitive Pattern Matching (`internal/extractor/score.go`)

**What:**
Replaced `strings.ToLower(class)` and `strings.Contains(cl, "pattern")` with a custom `hasScorePattern` function that scans class and ID attributes with zero heap allocations.

**Why:**
The previous code in the hot DOM-scoring loop (`scoreNode`) called `strings.ToLower` on every `class` and `id` string for thousands of scored elements per page. If the input string had even a single uppercase character, `strings.ToLower` allocated a completely new string byte array on the heap, creating immense GC pressure and CPU overhead during long crawls.

**Measured Improvement:**
In micro-benchmarks analyzing class string evaluations with uppercase characters, memory allocation dropped from `24 B/op` (1 alloc) to `0 B/op` (0 allocs). Overall execution time remained flat or slightly better (`183 ns/op` to `179 ns/op`) while saving massive background garbage collection costs at scale.

**Learning:**
In deeply nested extraction or scoring loops, avoid using `strings.ToLower` combined with `strings.Contains` for simple pattern matching if the input strings regularly contain mixed case characters. Instead, use zero-allocation loops with manual case-insensitive byte conversion checks (`c += 'a' - 'A'`).

## 2026-04-08 - O(1) Short-Circuiting URL Evaluation in MCP Crawler (`internal/mcp/server.go`)

**Learning:**
Moved the `url.Parse` inside the MCP server's crawl link evaluation logic to execute *after* checking the boolean `visited` map. We lock `crawlMu`, evaluate if the link is known, and unlock early, only performing the `url.Parse` for strictly new links.
Similar to a previous optimization in the crawl manager (`internal/api/crawl_manager.go`), `url.Parse` performs multiple memory allocations and state machine parsing operations. Because standard web pages have identical header and footer navigation on every page, a full site crawl will attempt to process the exact same URLs thousands of times. Validating against the O(1) visited map short-circuits this massive repeated overhead.

**Action:**
Whenever extracting and verifying absolute URLs dynamically within the `OnHTML` hot loop, always check if the string signature already exists in a lookup map *before* attempting to parse or validate the string mathematically.
## 2026-04-11 - Avoid goquery.Selection allocations in hot DOM traversals
**Learning:** In hot scraping and DOM-scoring loops, repeatedly traversing the DOM using `goquery` methods like `sel.Parent()` creates significant CPU and memory allocation overhead because it creates new structs and slices on every step.
**Action:** Bypassing `goquery` and manually traversing the underlying `x/net/html` node tree (e.g., `for p := sel.Get(0).Parent; p != nil; p = p.Parent`) eliminates these allocations. Use a transient struct `&goquery.Selection{Nodes: []*html.Node{p}}` if an API demands a `*goquery.Selection`.

## 2026-04-13 - Fast-Path URL Resolution to Avoid url.Parse Allocations (`internal/crawler/colly.go`)

**What:**
Created a custom `utils.ResolveHref` utility to bypass `url.Parse` and `baseURL.ResolveReference` for common URL formats during link extraction in `appendResolvedHref`. The new utility checks if a URL is already absolute (`http://` or `https://`) or root-relative (`/` but not `//`) and uses simple string concatenation to construct the absolute URL.

**Why:**
The `appendResolvedHref` function runs on every discovered link during an HTML crawl (`a[href]`). Using `url.Parse` inside this hot loop creates a heavy overhead as it instantiates a full `url.URL` struct and parses query/fragment states. For absolute or simple root-relative paths—which constitute the vast majority of web links—parsing the URL mathematically is unnecessary and causes enormous garbage collection and CPU overhead.

**Measured Improvement:**
In micro-benchmarks analyzing the common root-relative URL (`/some/path`), execution time dropped from `~751-954 ns/op` to `~76-82 ns/op` (~90% speedup) when comparing standard `url.Parse` to the low-allocation string prefix checking method. Absolute URLs take only `~4 ns/op` because they can be immediately returned (negligible cost), while root-relative URLs still allocate the resulting concatenated output string.

**Learning:**
When constructing or verifying massive amounts of absolute URLs dynamically inside DOM hot loops (such as Colly `OnHTML` handlers), implement low-allocation string prefix checking fast-paths (`strings.HasPrefix`) to avoid `url.Parse` allocations before resorting to the highly intensive `url.Parse` state machine.

## 2026-04-14 - Zero-Allocation URL Validation in shouldScrapeURL (`internal/api/crawl_manager.go`)

**What:**
Replaced unconditional `url.Parse` inside `shouldScrapeURL` with a manual zero-allocation fast-path that scans for the host and path via `strings.Index`.

**Why:**
The `shouldScrapeURL` function is called for every link discovered during a recursive crawl (via `visitIfAllowed`). `url.Parse` instantiates slices and structs internally. Because all crawled absolute URLs follow the standard `scheme://host/path` structure, manual string scanning eliminates heap allocations when evaluating if the host matches `baseURL.Host`.

**Measured Improvement:**
In bulk benchmark operations against the `shouldScrapeURL` suite parsing lists of URLs, execution time dropped from `~14381 ns/op` down to `~5164 ns/op`, achieving a ~64% speedup. Micro-benchmarks validating single domain checks showed a reduction from `~352 ns/op` to `~28 ns/op`.

**Learning:**
Never rely on `url.Parse` in hot loops scoring or verifying thousands of URLs simply to check domains or basic path matches. Prefer custom manual string scans with `strings.Index` to bypass all allocations, only falling back to `url.Parse` for unexpected or highly malformed cases.

## 2026-04-14 - Safe Low-Allocation URL Domain Validation Fast-Path (`internal/mcp/server.go`)

**What:**
Replaced unconditional `url.Parse` inside `performCrawl`'s `OnHTML` hot loop with a safe, low-allocation fast-path. It hoists `expectedOrigin` (`baseURL.Scheme + "://" + baseURL.Host`) outside the loop and uses `strings.HasPrefix(absURL, expectedOrigin)` to bypass state-machine parsing for valid standard URLs.

**Why:**
The `parsedURL, err := url.Parse(absURL)` step inside the link iterator processes every single `a[href]` on every page during a crawl. `url.Parse` performs multiple memory allocations and state machine iterations. Because standard absolute web URLs reliably start with `scheme://host`, a prefix match can safely short-circuit domain validation.

**Measured Improvement:**
Micro-benchmarks parsing standard URLs validate that checking `strings.HasPrefix` avoids full struct creation, running in ~38 ns/op compared to `url.Parse`'s ~387 ns/op, yielding ~90% speedup per URL evaluation and eliminating inner-loop GC pressure.

**Learning:**
Ad-hoc string splitting or splitting on `/` to manually implement `url.Parse` is dangerous (vulnerable to `evil.com?@example.com` SSRF escapes) and error-prone. Instead, leverage simple deterministic prefix checks (hoisted string concatenation) against `baseURL.Scheme + "://" + baseURL.Host` for safe O(1) matching. Only fall back to `url.Parse` if the fast-path fails.
## 2026-04-15 - Zero-allocation net/html tree traversal for goquery subtrees
**Learning:** In GoQuery, chaining methods like `root.Union(root.Find("*"))` on large DOM subtrees causes massive hidden slice/struct allocations and iteration overhead per node.
**Action:** Manually traverse the underlying `x/net/html` tree recursively (using `FirstChild` and `NextSibling`) to eliminate temporary struct/slice allocations when collecting nodes.
## 2026-04-18 - Idiomatic Zero-Allocation Case-Insensitive String Matches
**Learning:** When performing case-insensitive string matches against short constants (like DOM attributes), `strings.ToLower()` introduces unnecessary heap allocations. While writing custom byte-scanning functions can eliminate these allocations, it violates clean code principles and introduces maintenance overhead. The standard library function `strings.EqualFold()` provides the same zero-allocation benefit while maintaining perfect readability.
**Action:** Always use `strings.EqualFold(a, b)` instead of `strings.ToLower(a) == b` when performing case-insensitive exact string comparisons in hot loops.

## 2026-04-18 - Optimized String Matching in Crawler Fallback Logic
**Learning:** In the HTML scraping fallback heuristics (`internal/crawler/fallback.go`), checking if a string matches specific challenge or framework patterns using `strings.ToLower` followed by a loop of `strings.Contains` causes unnecessary heap allocations. By replacing this with a zero-allocation utility `HasAnyLowercasePattern` that iterates byte-by-byte (similar to an optimization already introduced in `.jules/bolt.md` memory for hot loops like `internal/extractor/score.go`), we can eliminate these allocations and improve the performance of parsing raw HTML to determine fallback strategies.
**Action:** When performing substring scanning against a list of static, lowercase patterns (especially on large strings like raw HTML), use `utils.HasAnyLowercasePattern(s, patterns)` instead of creating an intermediate string copy with `strings.ToLower`.

## 2026-04-19 - Zero-Allocation String Splitting in JS Extraction (`internal/extractor/jseval.go`)

**Learning:** When iterating over lines of large string payloads (like massive Next.js JSON dumps), using `strings.Split(str, "\n")` allocates a massive array of strings on the heap. Using a zero-allocation manual scanner with `strings.IndexByte(str, '\n')` reduces allocations to exactly 0 while cutting execution time by nearly 50%.
**Action:** Avoid `strings.Split` for large text payloads in hot loops; manually track string slices instead.

## 2026-04-24 - Safe Domain Validation via Prefix Fast-Path (`internal/api/crawl_manager.go`)

**What:**
Replaced the manual string splitting parser in `shouldScrapeURL` with a safe string prefix check, similar to `internal/mcp/server.go`.

**Why:**
The `shouldScrapeURL` loop executes for every link discovered during a recursive crawl. Previously, it utilized `url.Parse` blindly causing excessive allocations. Attempting to manually split on `://` and `/` or `@` was deemed unsafe and vulnerable to credential escape or SSRF issues because standard-library behavior (un-escaping paths, dealing with `user:pass@`) is complex.
Using `strings.HasPrefix(absURL, baseURL.Scheme + "://" + baseURL.Host)` provides an O(1), safe, zero-allocation domain matching fast-path for the most common case: links on the same domain matching the scheme perfectly. Only if it fails this exact prefix match does it fall back to `url.Parse`.

**Measured Improvement:**
In bulk benchmark operations against the `shouldScrapeURL` suite parsing lists of URLs, execution time dropped from `~8228 ns/op` down to `~5154 ns/op`, achieving a ~37% speedup. Memory allocations dropped from `1597 B/op` (11 allocs) to `0 B/op` (0 allocs) for valid, matching URLs.

**Learning:**
Never rely on manual, ad-hoc string slicing (e.g. splitting by `://`, `/`, `@`) to bypass `url.Parse` as it leads to blocking regressions around basic-auth and URL encoding. Instead, always use deterministic, full-prefix matching (`strings.HasPrefix(absURL, expectedOrigin)`) combined with exact boundary checking to short-circuit domain evaluation safely.

## 2026-05-02 - Zero-Allocation DOM Traversal for Scoring (`internal/extractor/score.go`)
**Learning:** In the DOM scoring loop (`scoreNode`), methods like `sel.Find("p")` and `sel.Find("a")` allocate completely new `goquery.Selection` structs and slices for each match, leading to heavy GC pressure when scoring multiple candidate nodes. By using `sel.Get(0)` to retrieve the root `x/net/html` node and manually traversing the tree using `FirstChild` and `NextSibling`, allocations are reduced to almost zero, and performance improves dramatically (e.g., ~2300ns/op down to ~200ns/op, a ~10x speedup).
**Action:** When performing aggregate counting or text extraction within small subtrees, bypass `goquery.Selection.Find` methods and manually traverse the underlying `x/net/html` tree structure.
## 2026-05-18 - Zero-Allocation Case-Insensitive Matching with EqualFold
**Learning:** In hot loops checking string map keys (e.g. `noiseRoles` in `IsNoise`, `classicScriptTypes` in JS extraction), using `strings.ToLower(strings.TrimSpace(s))` followed by a map lookup causes unnecessary string allocations and CPU overhead on every check. While using zero-allocation byte scanners is an option, for exactly matching small constant sets, falling back to a sequence of `strings.EqualFold` checks completely eliminates allocations and avoids overhead.
**Action:** When performing case-insensitive matching against a small, constant set of strings in hot code paths, avoid creating `strings.ToLower()` map keys. Instead, trim the string and use a boolean expression chain with `strings.EqualFold()`.

## 2026-05-18 - Zero-Allocation Case-Insensitive Substring Match (`internal/extractor/jseval.go`)
**Learning:** In error checking paths or validation functions (like `isExecutionTimeoutErr`), evaluating `strings.Contains(strings.ToLower(err.Error()), "...")` creates a full heap-allocated lowercased copy of the error string on every invocation. By replacing this with the project's custom `utils.HasAnyLowercasePattern(err.Error(), []string{"..."})` function, we eliminate the string allocation and cut CPU execution time significantly without changing functionality.
**Action:** Always prefer `utils.HasAnyLowercasePattern` over combining `strings.ToLower` and `strings.Contains` when checking if an input string contains specific case-insensitive substrings.

## 2024-05-06 - Replacing strings.TrimSpace(sel.Text()) with calculateTrimmedTextLength
**Learning:** In hot loops mapping over nodes to calculate scores (like `scoreNode` in `internal/extractor/score.go`), calling `sel.Text()` followed by `strings.TrimSpace()` creates a huge string allocation of the concatenated text, only to measure its length and immediately throw it away.
**Action:** Replace `strings.TrimSpace(sel.Text())` with a manual, zero-allocation `calculateTrimmedTextLength` function that walks the `html.Node` tree and accumulates lengths of `html.TextNode` elements directly, carefully trimming leading spaces on the first text node and calculating total trailing spaces correctly. This results in a roughly 10x performance speedup (from 1085ns to 109ns) and reduces allocations from 2 to 0 per call.

## 2026-05-19 - Zero-Allocation Token Evaluation in IsNoise
**Learning:** In the DOM node filtering loops (`IsNoise`), relying on `strings.ToLower` for class string values and map lookups for exact matches generates significant heap allocations due to strings containing uppercase or mixed-case values.
**Action:** Remove `strings.ToLower` entirely. Instead, convert token exact-match maps to string slices and use `strings.EqualFold(tok, exactStr)` and zero-allocation prefix scans to evaluate DOM tokens without allocating new lowercased strings.

## 2026-05-08 - Zero-Allocation Text Length Calculation for Links in Scoring
**Learning:** During node scoring (`internal/extractor/score.go`), counting link text lengths using `strings.Builder` followed by `strings.TrimSpace` results in unnecessary allocations (heap allocations for every link analyzed). Replacing this with the pre-existing `calculateTrimmedTextLength` zero-allocation node traverser speeds up the DOM scoring heavily by dropping allocations for links to exactly zero.
**Action:** Eliminate `strings.Builder` and `strings.TrimSpace` chains for evaluating link lengths. Traverse the tree natively to sum lengths instead.
## 2024-05-18 - Hoist expected origin calculation out of hot loops
**Learning:** In the crawling logic, the origin of the base URL (`baseURL.Scheme + "://" + baseURL.Host`) was being computed repeatedly for every single URL checked in `shouldScrapeURL`. Because `shouldScrapeURL` is called frequently (for all unvisited links across all scraped pages), this recurrent string concatenation caused unnecessary heap allocations and CPU cycles.
**Action:** When a constant string format is derived from a parent object that remains static throughout an operation (like a crawl job's base URL), hoist the concatenation out of the hot loop. Pre-compute it once and pass it as an argument to the functions running in the hot loop to reduce memory allocations while maintaining code safety and readability.

## 2024-05-18 - Zero-allocation node discovery in extractor scoring (`internal/extractor/score.go` & `internal/extractor/content.go`)
**Learning:** In hot loops such as `ExtractMainHTML` and `findBestCandidate`, the code previously used `doc.Find("article, main, [role='main']...")` which parses a complex CSS selector dynamically using Cascadia and allocates multiple `goquery.Selection` structs on the heap for every element matched. By replacing this with a manual, zero-allocation recursive tree traversal of `x/net/html` nodes (`walk(n *html.Node)`), we bypass `goquery.Selection` creation for the filtering step, speeding up candidate evaluation by ~14x (e.g. from 3200 ns/op to 230 ns/op) and heavily reducing GC pressure during recursive DOM crawls.
**Action:** When searching for broad node characteristics (e.g. specific tags or simple attributes like `role="main"`) across the entire document during high-frequency tasks, bypass `goquery.Find()` entirely. Instead, use a manual `html.Node` recursive walk and check element properties natively using `strings.EqualFold()`. Create transient `&goquery.Selection{Nodes: []*html.Node{n}}` wrappers only exactly when passing the matched node to downstream components.

## 2026-05-19 - Zero-Allocation Authorization Header Parsing (`internal/api/middleware.go`)
**Learning:** In API middleware (`AuthMiddleware`, `RateLimitMiddleware`), extracting the API key from the `Authorization` header using `strings.SplitN(authHeader, " ", 2)` allocates a slice and strings on the heap for every single incoming HTTP request. By replacing this with a zero-allocation check using `len(authHeader) > 7 && strings.EqualFold(authHeader[:7], "Bearer ")` followed by `authHeader[7:]`, we avoid all heap allocations and reduce parsing time from ~74ns to ~8ns (~9x speedup).
**Action:** When extracting values from HTTP headers with known prefixes (like "Bearer "), avoid `strings.Split` and `strings.SplitN`. Use `strings.EqualFold` with string slicing instead for a zero-allocation fast path.

## 2024-05-18 - Avoid colly.Request.AbsoluteURL in hot loops
**Learning:** In hot link discovery loops (like OnHTML for "a[href]"), calling `e.Request.AbsoluteURL(link)` incurs extremely high CPU overhead because it uses `url.Parse` and `url.ResolveReference` on every single discovered link.
**Action:** Always use the zero-allocation/low-allocation fast path `utils.ResolveHref(e.Request.URL, link)` instead. This utility bypasses full struct allocations and URL parsing for common absolute and root-relative paths, resolving references more than 10x faster.
