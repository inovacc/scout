# Feature Requests

## Completed Features

### Browser Automation Core
- **Status:** Completed
- **Description:** Full browser lifecycle, page navigation, element interaction, JS evaluation, screenshots, PDF, network control, stealth mode, device emulation, DOM traversal.

### Scraping Toolkit
- **Status:** Completed
- **Description:** Struct-tag extraction engine, table/metadata extraction, form detection and filling, rate limiting with retry, pagination (click/URL/scroll/load-more), search engine integration (Google/Bing/DDG), BFS crawling with sitemap parser.

## Proposed Features

### Session & Local Storage Management
- **Priority:** P1
- **Status:** Proposed
- **Description:** Get/set/clear session storage and local storage. Export and import storage state. Enhanced cookie management with filtering and bulk operations.

### JavaScript Execution Toolkit
- **Priority:** P1
- **Status:** Proposed
- **Description:** High-level patterns for executing JS to extract website information. Script injection, result collection, and common extraction recipes (e.g., all event listeners, computed styles, performance metrics).

### Distributed Crawling (Swarm Mode)
- **Priority:** P2
- **Status:** Proposed
- **Description:** Split crawl workloads across multiple browser instances running on different IPs/proxies. Browser cluster management, shared BFS queue, result aggregation, headless swarm configuration.

### Context Support
- **Priority:** P2
- **Status:** Proposed
- **Description:** Accept `context.Context` on methods for cancellation and deadline propagation, instead of relying solely on rod's timeout mechanism.

### Connection to Existing Browser
- **Priority:** P2
- **Status:** Proposed
- **Description:** Add an option to connect to an already-running browser via WebSocket URL (rod supports `ControlURL`), useful for debugging and reusing browser sessions.

### Page Pool
- **Priority:** P3
- **Status:** Proposed
- **Description:** Page pooling for concurrent scraping workloads, managing a fixed number of pages and recycling them across tasks.

### Retry with Backoff (Core Methods)
- **Priority:** P3
- **Status:** Completed (via RateLimiter)
- **Description:** Built-in retry logic for transient navigation and element-finding failures, with configurable backoff strategy. Implemented as `RateLimiter.Do()` and `Page.NavigateWithRetry()`.
