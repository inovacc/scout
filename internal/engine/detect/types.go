package detect

// FrameworkInfo describes a detected frontend framework on the current page.
type FrameworkInfo struct {
	Name    string `json:"name"`              // e.g. "React", "Vue", "Angular", "Svelte", "Next.js"
	Version string `json:"version,omitempty"` // version string if detectable
	SPA     bool   `json:"spa"`               // true if the page appears to be a single-page application
}

// TechStack describes the full technology stack detected on a page.
type TechStack struct {
	Frameworks   []FrameworkInfo `json:"frameworks,omitempty"`
	CSSFramework string          `json:"css_framework,omitempty"`
	BuildTool    string          `json:"build_tool,omitempty"`
	CMS          string          `json:"cms,omitempty"`
	Analytics    []string        `json:"analytics,omitempty"`
	CDN          string          `json:"cdn,omitempty"`
}

// RenderMode classifies how a page was rendered.
type RenderMode string

const (
	RenderCSR     RenderMode = "CSR" // Client-Side Rendering
	RenderSSR     RenderMode = "SSR" // Server-Side Rendering
	RenderSSG     RenderMode = "SSG" // Static Site Generation
	RenderISR     RenderMode = "ISR" // Incremental Static Regeneration
	RenderUnknown RenderMode = "unknown"
)

// RenderInfo describes the detected rendering mode of the current page.
type RenderInfo struct {
	Mode     RenderMode `json:"mode"`
	Hydrated bool       `json:"hydrated"`
	Details  string     `json:"details,omitempty"`
}

// PWAInfo describes Progressive Web App capabilities detected on the current page.
type PWAInfo struct {
	HasServiceWorker bool            `json:"has_service_worker"`
	HasManifest      bool            `json:"has_manifest"`
	Installable      bool            `json:"installable"`
	HTTPS            bool            `json:"https"`
	PushCapable      bool            `json:"push_capable"`
	Manifest         *WebAppManifest `json:"manifest,omitempty"`
}

// WebAppManifest holds parsed data from the page's web app manifest.
type WebAppManifest struct {
	Name            string `json:"name"`
	ShortName       string `json:"short_name,omitempty"`
	Display         string `json:"display,omitempty"`
	StartURL        string `json:"start_url,omitempty"`
	ThemeColor      string `json:"theme_color,omitempty"`
	BackgroundColor string `json:"background_color,omitempty"`
	Icons           int    `json:"icons"`
}
