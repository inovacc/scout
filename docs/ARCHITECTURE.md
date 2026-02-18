# Architecture

## System Overview

```mermaid
flowchart TB
    subgraph Library["package scout (core library)"]
        Browser["Browser\n(browser.go)"]
        Page["Page\n(page.go)"]
        Element["Element\n(element.go)"]
        Recorder["NetworkRecorder\n(recorder.go)"]
        Options["Option / With*()\n(option.go)"]
        Eval["EvalResult\n(eval.go)"]

        subgraph Scraping["Scraping Toolkit"]
            Extract["Extract\n(extract.go)"]
            Form["Form / FormWizard\n(form.go)"]
            Paginate["PaginateBy*\n(paginate.go)"]
            Search["Search\n(search.go)"]
            Crawl["Crawl\n(crawl.go)"]
            Map["Map\n(map.go)"]
            Markdown["Markdown\n(markdown.go)"]
            RateLimit["RateLimiter\n(ratelimit.go)"]
            Batch["BatchScrape\n(batch.go)"]
            Recipe["Recipe\n(recipe/)"]
            Swagger["Swagger/OpenAPI\n(swagger.go)"]
            Extension["Extension DL\n(extension.go)"]
            LLM["LLM Extraction\n(llm*.go)"]
        end

        subgraph Infrastructure["Infrastructure"]
            Bridge["Bridge\n(bridge.go)"]
            Network["HijackRouter\n(network.go)"]
            Storage["Storage / Session\n(storage.go)"]
            Window["WindowState\n(window.go)"]
            Selector["Selector helpers\n(selector.go)"]
        end
    end

    subgraph gRPC["gRPC Service Layer"]
        Proto["scout.proto\n(grpc/proto/)"]
        PB["scoutpb\n(generated)"]
        Server["ScoutServer\n(grpc/server/)"]
        TLS["mTLS / Pairing\n(tls.go, pairing.go)"]
        Platform["Platform Defaults\n(platform_*.go)"]
    end

    subgraph Identity["Identity & Discovery"]
        DeviceID["DeviceIdentity\n(pkg/identity/)"]
        Discovery["mDNS Discovery\n(pkg/discovery/)"]
        StealthPkg["Stealth\n(pkg/stealth/)"]
    end

    subgraph Commands["Unified CLI (cmd/scout/)"]
        CLI["scout CLI\n(Cobra)"]
        Daemon["Daemon\n(auto-start gRPC)"]
        Sessions["Session Tracking\n(~/.scout/)"]
    end

    subgraph External["External"]
        Chrome["Chromium / Chrome\n/ Brave / Edge"]
        Rod["go-rod/rod"]
    end

    Options -->|configures| Browser
    Browser -->|creates| Page
    Page -->|finds| Element
    Page -->|attaches| Recorder
    Page -->|uses| Extract
    Page -->|uses| Form
    Page -->|uses| Search
    Browser -->|uses| Crawl
    Browser -->|uses| Map
    Page -->|uses| Markdown
    Browser -->|uses| Paginate
    RateLimit -->|throttles| Page

    Server -->|uses| Browser
    Server -->|uses| Page
    Server -->|uses| Recorder
    TLS -->|secures| Server
    Platform -->|configures| Server
    PB -->|implements| Server
    CLI -->|manages| Sessions
    CLI -->|auto-starts| Daemon
    Daemon -->|starts| Server
    CLI -->|calls| PB

    DeviceID -->|authenticates| TLS
    Discovery -->|finds peers| Server
    StealthPkg -->|patches| Page

    Rod -->|CDP protocol| Chrome
    Browser -->|wraps| Rod
```

## Core Library Flow

```mermaid
sequenceDiagram
    participant User as User Code
    participant Scout as scout.Browser
    participant Rod as go-rod
    participant Chrome as Chromium

    User->>Scout: New(WithHeadless(true), ...)
    Scout->>Rod: rod.New().Launcher(...)
    Rod->>Chrome: Launch headless browser
    Chrome-->>Rod: WebSocket connection
    Rod-->>Scout: *rod.Browser
    Scout-->>User: *scout.Browser

    User->>Scout: browser.NewPage(url)
    Scout->>Rod: browser.Page(url)
    Rod->>Chrome: Target.createTarget
    Chrome-->>Rod: Page created
    Rod-->>Scout: *rod.Page
    Scout-->>User: *scout.Page

    User->>Scout: page.Element("h1")
    Scout->>Rod: page.Element("h1")
    Rod->>Chrome: DOM.querySelector
    Chrome-->>Rod: Element node
    Rod-->>Scout: *rod.Element
    Scout-->>User: *scout.Element

    User->>Scout: el.Text()
    Scout->>Rod: el.Text()
    Rod->>Chrome: Runtime.evaluate
    Chrome-->>Rod: "Hello World"
    Scout-->>User: "Hello World", nil

    User->>Scout: browser.Close()
    Scout->>Rod: browser.Close()
    Rod->>Chrome: Browser.close
```

## HAR Network Recording

```mermaid
sequenceDiagram
    participant User as User Code
    participant Rec as NetworkRecorder
    participant Page as scout.Page
    participant Rod as rod.Page
    participant Chrome as CDP Events

    User->>Rec: NewNetworkRecorder(page, opts...)
    Rec->>Rod: page.EachEvent(...)
    Note over Rec,Chrome: Listening for CDP events

    User->>Page: page.Navigate(url)
    Chrome-->>Rec: NetworkRequestWillBeSent
    Rec->>Rec: Store pending entry

    Chrome-->>Rec: NetworkResponseReceived
    Rec->>Rec: Update entry with response + timing

    Chrome-->>Rec: NetworkLoadingFinished
    alt CaptureBody enabled
        Rec->>Rod: NetworkGetResponseBody
        Rod->>Chrome: Fetch body
        Chrome-->>Rec: Response body
    end
    Rec->>Rec: Move to completed entries

    User->>Rec: ExportHAR()
    Rec-->>User: HAR 1.2 JSON, entry count
```

## gRPC Remote Control

```mermaid
sequenceDiagram
    participant Client as scout CLI
    participant gRPC as gRPC Transport
    participant Server as ScoutServer
    participant Session as session
    participant Scout as scout Library
    participant Chrome as Chromium

    Client->>gRPC: CreateSession(headless, stealth, ...)
    gRPC->>Server: CreateSession()
    Server->>Scout: scout.New(opts...)
    Scout->>Chrome: Launch browser
    Server->>Scout: browser.NewPage(url)
    Server->>Server: wireEvents(session)
    Server-->>Client: session_id, url, title

    par Event Stream
        Client->>gRPC: StreamEvents(session_id)
        loop CDP Events
            Chrome-->>Server: Network/Console/Page events
            Server->>Server: session.broadcast(event)
            Server-->>Client: BrowserEvent
        end
    and User Commands
        Client->>gRPC: Navigate(session_id, url)
        gRPC->>Server: Navigate()
        Server->>Scout: page.Navigate(url)
        Scout->>Chrome: Page.navigate
        Server-->>Client: NavigateResponse(url, title)

        Client->>gRPC: Click(session_id, selector)
        gRPC->>Server: Click()
        Server->>Scout: page.Element(selector)
        Server->>Scout: el.Click()
        Server-->>Client: Empty
    end

    Client->>gRPC: ExportHAR(session_id)
    Server->>Session: recorder.ExportHAR()
    Server-->>Client: HARResponse(data, count)

    Client->>gRPC: DestroySession(session_id)
    Server->>Scout: browser.Close()
    Server-->>Client: Empty
```

## Bidirectional Interactive Stream

```mermaid
sequenceDiagram
    participant Client as scout client REPL
    participant Stream as Interactive Stream
    participant Server as ScoutServer
    participant Scout as scout Library

    Client->>Stream: Open Interactive()
    Client->>Stream: Command{Navigate, url}
    Stream->>Server: executeCommand()
    Server->>Scout: page.Navigate(url)

    par Events flow back
        Scout-->>Server: CDP events
        Server-->>Stream: BrowserEvent
        Stream-->>Client: BrowserEvent
    end

    Client->>Stream: Command{Type, selector, text}
    Server->>Scout: page.Element(selector).Input(text)

    Client->>Stream: Command{Click, selector}
    Server->>Scout: page.Element(selector).Click()

    alt Command fails
        Server-->>Stream: BrowserEvent{Error}
        Stream-->>Client: ErrorEvent
        Note over Client,Server: Stream continues (not broken)
    end

    Client->>Stream: CloseSend()
    Stream-->>Client: EOF
```

## Generic Auth Framework

```mermaid
flowchart TB
    subgraph AuthFramework["scraper/auth/"]
        Provider["Provider Interface"]
        Registry["Registry"]
        BrowserAuth["BrowserAuth\n(browser_auth.go)"]
        BrowserCapture["BrowserCapture\n(capture all before close)"]
        OAuth["OAuthServer\n(oauth.go)"]
        Electron["ElectronSession\n(electron.go)"]
        SessionPersist["SaveEncrypted / LoadEncrypted\n(session.go)"]
    end

    subgraph Providers["Provider Implementations"]
        SlackProv["SlackProvider\n(scraper/slack/provider.go)"]
        TeamsProv["TeamsProvider\n(planned)"]
        DiscordProv["DiscordProvider\n(planned)"]
    end

    subgraph CLI["CLI Commands"]
        AuthLogin["scout auth login --provider slack"]
        AuthCapture["scout auth capture --url <any>"]
        AuthStatus["scout auth status"]
        AuthLogout["scout auth logout"]
    end

    Provider -->|implements| SlackProv
    Provider -->|implements| TeamsProv
    Provider -->|implements| DiscordProv
    Registry -->|stores| Provider
    BrowserAuth -->|uses| Provider
    BrowserAuth -->|launches| Scout
    BrowserCapture -->|launches| Scout
    OAuth -->|local callback| BrowserAuth
    Electron -->|CDP connect| Scout

    AuthLogin -->|uses| BrowserAuth
    AuthCapture -->|uses| BrowserCapture
    BrowserAuth -->|saves| SessionPersist
    BrowserCapture -->|saves| SessionPersist

    Scout["scout.Browser\n+ scout.Page"]
```

```mermaid
sequenceDiagram
    participant User as User
    participant CLI as scout CLI
    participant Auth as auth.BrowserAuth
    participant Browser as Chromium
    participant Provider as Provider.DetectAuth

    CLI->>Auth: BrowserAuth(ctx, provider, opts)
    Auth->>Browser: Launch (non-headless)
    Auth->>Browser: Navigate to LoginURL()
    CLI-->>User: "please log in..."

    loop Poll every 2s
        Auth->>Provider: DetectAuth(page)
        Provider-->>Auth: false
    end

    User->>Browser: Complete login
    Auth->>Provider: DetectAuth(page)
    Provider-->>Auth: true
    Auth->>Provider: CaptureSession(page)
    Note over Auth,Browser: Captures cookies, localStorage,<br/>sessionStorage, tokens
    Auth->>Browser: Close
    Auth->>CLI: Session
    CLI->>CLI: SaveEncrypted(session)
    CLI-->>User: "session saved to slack-session.enc"
```

## Scraping Pipeline

```mermaid
flowchart LR
    subgraph Input
        URL[Target URL]
        Config[Options]
    end

    subgraph Rate["Rate Limiting"]
        RL[RateLimiter]
        Backoff[Exponential Backoff]
    end

    subgraph Navigation
        Nav[Navigate]
        Wait[WaitLoad / WaitStable]
    end

    subgraph Extraction
        Extract[Extract via struct tags]
        Table[ExtractTable]
        Meta[ExtractMeta]
        Text[ExtractText/Links]
    end

    subgraph Pagination
        Click[PaginateByClick]
        URLPat[PaginateByURL]
        Scroll[PaginateByScroll]
        LoadMore[PaginateByLoadMore]
    end

    subgraph Output
        Data[Typed Go structs]
        HAR[HAR recording]
    end

    URL --> RL
    Config --> RL
    RL --> Nav
    RL -.->|retry on failure| Backoff
    Backoff -.-> Nav
    Nav --> Wait
    Wait --> Extract
    Wait --> Table
    Wait --> Meta
    Wait --> Text
    Extract --> Click
    Extract --> URLPat
    Extract --> Scroll
    Extract --> LoadMore
    Click --> Data
    URLPat --> Data
    Scroll --> Data
    LoadMore --> Data
    Nav -.-> HAR
```
