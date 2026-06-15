# Playwright-style DX layer for Scout

**Goal:** Close the day-to-day ergonomics gap with Playwright by adding, to Scout's Go engine, a Locators API + actionability auto-waiting + web-first retrying assertions + device presets + emulation. (Out of scope — architectural rebuilds: cross-engine Firefox/WebKit, multi-language bindings, component testing.)

**Layering:** new code lives in `internal/engine` (alongside `Page`/`Element`), re-exported through the `pkg/scout` facade. Real-browser+httptest tests (no mocks), skip without Chromium.

## 1. Locators (lazy, auto-waiting)

`Page` constructors returning `*Locator`:
```go
func (p *Page) GetByRole(role string, opts ...LocatorOption) *Locator
func (p *Page) GetByText(text string, opts ...LocatorOption) *Locator
func (p *Page) GetByLabel(text string, opts ...LocatorOption) *Locator
func (p *Page) GetByPlaceholder(text string, opts ...LocatorOption) *Locator
func (p *Page) GetByTestID(id string) *Locator                 // [data-testid="id"]
func (p *Page) GetByAltText(text string, opts ...LocatorOption) *Locator
func (p *Page) GetByTitle(text string, opts ...LocatorOption) *Locator
func (p *Page) Locate(cssSelector string) *Locator
```
`LocatorOption`: `WithExact(bool)` (default substring, case-insensitive), `WithName(string)` (accessible-name filter for GetByRole).

A `Locator` is lazy — it resolves at action time via an injected JS engine `__scoutLocate(spec)` returning the matching node, mapped to `*Element`. Role resolution uses an implicit-role→selector map (button/link/textbox/checkbox/heading/…) + accessible-name match (text / aria-label / title / placeholder / alt). Multiple matches → `First()`/`Nth(i)`/`Last()`.

Actions (each auto-waits for actionability first):
```go
Click(); Fill(s); Type(s); Press(key); Hover(); Check(); Uncheck(); SelectOption(vals...)
TextContent() (string,error); InnerText(); GetAttribute(name); InputValue()
IsVisible()/IsEnabled()/IsChecked() (bool,error); Count() (int,error)
First()/Nth(i)/Last() *Locator; WaitFor(state) ; Element() (*Element,error); Elements()
```

## 2. Actionability auto-waiting

Internal `waitActionable(loc, timeout)`: poll (default 50ms) until the element resolves AND is visible AND enabled (not `disabled`) AND stable (bounding box identical across two animation frames); scroll into view; else error after timeout (default = browser timeout, ~30s for actions). Every Locator action funnels through it.

## 3. Web-first retrying assertions

```go
func Expect(l *Locator, opts ...ExpectOption) *LocatorAssertions   // WithTimeout default 5s
func (a *LocatorAssertions) Not() *LocatorAssertions
ToBeVisible(); ToBeHidden(); ToBeEnabled(); ToBeDisabled(); ToBeChecked()
ToHaveText(s); ToContainText(s); ToHaveValue(s); ToHaveCount(n); ToHaveAttribute(name,val)
```
Each polls until satisfied or timeout; returns a descriptive `error` (`expect: ToHaveText: got %q want %q after %s`). Pairs with `Not()` for negation.

## 4. Device presets

```go
type Device struct { UserAgent string; Viewport Viewport; DeviceScaleFactor float64; IsMobile, HasTouch bool }
var Devices map[string]Device   // "iPhone 13","iPhone SE","Pixel 5","iPad (gen 7)","Galaxy S9+","Desktop Chrome","Desktop Safari", ...
func WithDevice(name string) Option          // launch-time: viewport+UA+DPR+touch+mobile
func (p *Page) EmulateDevice(name string) error   // runtime via CDP setDeviceMetricsOverride + setTouchEmulationEnabled + UA
```

## 5. Emulation (CDP wrappers on Page)

```go
SetGeolocation(lat,lon,accuracy float64) error / ClearGeolocation()
SetTimezone(tz string) error          // Emulation.setTimezoneOverride
SetLocale(locale string) error        // Emulation.setLocaleOverride
SetColorScheme(scheme string) error   // Emulation.setEmulatedMedia prefers-color-scheme
GrantPermissions(perms []string, origin string) error   // Browser.grantPermissions
SetOffline(offline bool) error        // Network.emulateNetworkConditions
```
Plus launch options where natural: `WithLocale`, `WithTimezone`, `WithColorScheme`, `WithGeolocation`.

## Files
`internal/engine/locator.go`, `internal/engine/expect.go`, `internal/engine/devices.go`, `internal/engine/emulation.go` (+ `_test.go` each); facade re-exports regenerated/added in `pkg/scout/scout.go`.

## Acceptance
Build clean; new types in the facade; for each feature a real-browser test against a local httptest page that exercises the happy path (skips without Chromium); `go vet` clean.
