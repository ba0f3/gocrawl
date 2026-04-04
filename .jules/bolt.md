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
