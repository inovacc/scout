# Scout vs Playwright — feature gap analysis

Source: Playwright `microsoft/playwright@main` documented surface (113 guides, 54 API classes, 12 test-runner classes, 26 packages; 91k★). Compared against Scout's current capabilities. ❌ = Scout lacks; 🟡 = partial/different; ✅ = parity or Scout-ahead.

## A. Structural / architectural (not "features" — would be rebuilds)
- ❌ **Cross-engine: Firefox + WebKit/Safari.** Playwright ships `playwright-browser-{firefox,webkit}` + bundled drivers. Scout is CDP-only (Chromium family). Biggest single gap.
- ❌ **Multi-language bindings.** Playwright: Node, Python, Java, .NET (`packages/playwright-{client,...}`, `intro-{python,java,csharp}`). Scout is Go-only (CLI + MCP + npm shim).
- ❌ **Component testing** (`test-components`, `playwright-ct-react/vue/react17`). Mount + test React/Vue/Svelte components. Scout has none.

## B. Test framework + tooling (Playwright Test) — mostly ❌
- ❌ **Test runner** (`@playwright/test`, `test`, `testconfig`, `testproject`): the whole runner. Scout is an automation lib/CLI, not a test framework.
- ❌ **Fixtures** (`test-fixtures`, `fixtures`, `workerinfo`, `testinfo`).
- ❌ **Projects / parallelism / sharding / retries / timeouts** (`test-parallel`, `test-sharding`, `test-retries`, `test-timeouts`, `test-projects`).
- ❌ **Reporters API** (`test-reporters`, `reporter`, `suite`, `testcase`, `testresult`, `teststep`) — HTML/JSON/JUnit reporters.
- ❌ **UI Mode** (`test-ui-mode`) — interactive watch/run UI.
- ❌ **Trace Viewer** (`trace-viewer`, `tracing` class) — time-travel DOM+network+console per action. Scout has HAR + markdown reports, not this.
- ❌ **Inspector / debugger** (`debug`, `debugger`) — pause + step + pick-locator.
- 🟡 **Codegen** (`codegen`) — record→code. Scout has `flow capture`/`guide`/`flow-porter` (records flows→scripts), but not polished test-code codegen.
- ❌ **Test Agents** (`test-agents-js`) — Playwright's new AI agents for generating/healing tests.
- ❌ **Global setup/teardown, parameterize, web-server, sharding config** (`test-global-setup-teardown`, `test-parameterize`, `test-webserver`).

## C. Locators & assertions — 🟡/❌ (the `feat/playwright-dx` branch targets these)
- 🟡 **Locators** (`locators`, `locator`, `framelocator`, `other-locators`): `getByRole/Text/Label/Placeholder/TestId/AltText/Title`, frame locators, chaining/filtering. Scout has CSS/XPath/text + an `aria` pkg; the getBy* Locator API is **in progress on `feat/playwright-dx`**.
- ❌ **Web-first retrying assertions** (`test-assertions`, `locatorassertions`, `pageassertions`, `apiresponseassertions`, `snapshotassertions`, `genericassertions`): `expect(locator).toBeVisible()` etc. that auto-retry. **In progress on the branch.**
- 🟡 **Actionability** (`actionability`): Scout's `Element.Click()` already auto-waits (scroll→hover→interactable→enabled). Parity on actions; gap is the *assertion* retry model.
- 🟡 **Custom selector engines** (`selectors`) — register your own selector engine. Scout has fixed strategies.

## D. Emulation — 🟡 (branch targets several)
- 🟡 **Device descriptors + geolocation/locale/timezone/permissions/color-scheme** (`emulation`): Scout has device emulation + stealth(timezone/fingerprint) + touch; clean geo/locale/permission/color-scheme APIs are **in progress on the branch**.
- ❌ **Clock API** (`clock`, `clock` class) — mock `Date`/`setTimeout`/timers, fast-forward time. Scout has nothing.
- 🟡 **Touch events** (`touch-events`, `touchscreen`) — Scout has touch emulation + ADB (dormant after localize).

## E. Network — 🟡/❌
- 🟡 **Request interception / mocking** (`mock`, `route`, `request`, `response`): Scout has hijack/block/intercept; Playwright's `route()`/`fulfill()` mocking is cleaner.
- ❌ **HAR record→replay / route-from-HAR** (`mock`, `routeFromHAR`): Scout records HAR but can't *replay/mock* from it.
- ❌ **WebSocketRoute** (`websocketroute`) — intercept/mock WS messages. Scout monitors WS (`ws_listen`) but can't route/mock.
- 🟡 **API testing** (`api-testing`, `apirequest`, `apirequestcontext`, `apiresponse`): Scout has `flow` (REST/GraphQL replay), not the request-fixture + response-assertion model.
- 🟡 **Service worker interception** (`service-workers`).

## F. Capture / media — 🟡/❌
- ✅ **Screenshots, PDF** (`screenshots`) — parity.
- ❌ **Video recording** (`videos`, `video`, `screencast`) — record context/page video. Scout has screenshots/HAR, not video.
- 🟡 **Visual snapshots as assertions** (`test-snapshots`, `snapshotassertions`): Scout has a visual `monitor`/baseline pkg, but not test-integrated `toHaveScreenshot()`.
- 🟡 **ARIA snapshots** (`aria-snapshots`): Scout has `aria.Capture`+`Diff` (close!) but not as `toMatchAriaSnapshot()` assertions.

## G. Context / auth / storage — 🟡
- 🟡 **BrowserContext isolation model** (`browser-contexts`, `browsercontext`) — N isolated contexts per browser for parallel isolation. Scout has sessions/incognito.
- 🟡 **storageState save/load** (`auth`, `webstorage`) — export/import cookies+storage for auth reuse. Scout has `.scoutprofile` + vault (different shape).

## H. Misc capabilities
- ❌ **JS/CSS coverage** (`coverage`) — collect coverage. Scout has none.
- ❌ **Selenium Grid** (`selenium-grid`) — connect to a Grid. (Scout just *removed* its remote/grid surface by design.)
- ❌ **WebView2** (`webview2`). 🟡 **Android** (`mobile-api/android*`) — Scout had ADB (dormant after localize); Playwright has a full Android API. ✅ **Electron** (`electron-api`) — Scout has `WithElectronApp`.
- ❌ **CLI install/`getting-started-cli`**, **VS Code extension** (`getting-started-vscode`).
- ✅ **Raw CDP** (`cdpsession`) — Scout has CDP access (rod/proto).
- ✅ **MCP server** (`getting-started-mcp`) — both have one (Scout's is a documented strength).
- 🟡 **Downloads / dialogs / file chooser** (`downloads`, `dialogs`, `filechooser`) — Scout has uploads; download/dialog handling is partial.

## Where Scout is AHEAD of Playwright (for balance)
- ✅ **Stealth / anti-bot** (17 evasions) + **fingerprint rotation** — Playwright is detectable; has none.
- ✅ **Scraper framework** (20 platforms), **LLM extraction**, **search-engine integration**, **research presets**.
- ✅ **Encrypted vault**, **subprocess plugin system**, **single static Go binary** (no Node runtime).

## Priority to close (highest value, feasible in Go/CDP)
1. **Locators + web-first assertions** — already started (`feat/playwright-dx`).
2. **Emulation APIs** (geo/locale/timezone/permissions/color-scheme) — started.
3. **Clock API** (mock time) — high-value, self-contained CDP feature, none today.
4. **Video recording** (CDP screencast → file).
5. **HAR replay / route-from-HAR** + **WebSocketRoute** (mocking).
6. **storageState** export/import + **coverage** API.
(Not worth it / out of scope: cross-engine, multi-language, full test runner + Trace Viewer UI, component testing.)
