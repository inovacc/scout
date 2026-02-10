# Feature Requests

## Completed Features

### Browser Automation Core
- **Status:** Completed
- **Description:** Full browser lifecycle, page navigation, element interaction, JS evaluation, screenshots, PDF, network control, stealth mode, device emulation, DOM traversal.

### Scraping Toolkit
- **Status:** Completed
- **Description:** Struct-tag extraction engine, table/metadata extraction, form detection and filling, rate limiting with retry, pagination (click/URL/scroll/load-more), search engine integration (Google/Bing/DDG), BFS crawling with sitemap parser.

### Window Control & Session Management
- **Status:** Completed
- **Description:** Window state control (minimize, maximize, fullscreen, restore), window bounds get/set, localStorage/sessionStorage access, save/load full session state (URL, cookies, storage). Implemented in `window.go` and `storage.go`.

### HAR Network Recording
- **Status:** Completed
- **Description:** Capture HTTP traffic via Chrome DevTools Protocol events, export as HAR 1.2 format. `NetworkRecorder` with functional options for body capture toggle and creator metadata. Implemented in `recorder.go`.

### Keyboard Input
- **Status:** Completed
- **Description:** Page-level keyboard control with `KeyPress(key)` for single keys and `KeyType(keys...)` for sequences. Uses rod `input.Key` constants. Added to `page.go`.

### gRPC Remote Control
- **Status:** Completed
- **Description:** Multi-session browser control via gRPC with 25+ RPCs covering session lifecycle, navigation, element interaction, query, capture, forensic recording, and event streaming. Includes bidirectional interactive streaming. Implemented in `grpc/server/`, `cmd/server/`, `cmd/client/`.

### Retry with Backoff (Core Methods)
- **Priority:** P3
- **Status:** Completed (via RateLimiter)
- **Description:** Built-in retry logic for transient navigation and element-finding failures, with configurable backoff strategy. Implemented as `RateLimiter.Do()` and `Page.NavigateWithRetry()`.

## Proposed Features

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
