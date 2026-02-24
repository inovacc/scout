package scout

import (
	"encoding/json"
	"fmt"
)

// FrameworkInfo describes a detected frontend framework on the current page.
type FrameworkInfo struct {
	Name    string `json:"name"`              // e.g. "React", "Vue", "Angular", "Svelte", "Next.js"
	Version string `json:"version,omitempty"` // version string if detectable
	SPA     bool   `json:"spa"`              // true if the page appears to be a single-page application
}

// detectFrameworkJS is the JavaScript that probes for framework-specific globals and DOM markers.
const detectFrameworkJS = `() => {
	const results = [];

	// React
	const reactRoot = document.querySelector('[data-reactroot]') ||
		document.querySelector('#__next') ||
		document.querySelector('#root');
	if (reactRoot && (reactRoot._reactRootContainer || reactRoot.__reactFiber$)) {
		let version = '';
		try { version = window.React && React.version || ''; } catch(e) {}
		results.push({name: 'React', version: version});
	} else if (window.__REACT_DEVTOOLS_GLOBAL_HOOK__ && window.__REACT_DEVTOOLS_GLOBAL_HOOK__.renderers && window.__REACT_DEVTOOLS_GLOBAL_HOOK__.renderers.size > 0) {
		let version = '';
		try { version = window.React && React.version || ''; } catch(e) {}
		results.push({name: 'React', version: version});
	}

	// Next.js
	if (window.__NEXT_DATA__ || document.querySelector('#__next')) {
		let version = '';
		try { version = window.__NEXT_DATA__ && window.__NEXT_DATA__.buildId || ''; } catch(e) {}
		if (window.next && window.next.version) version = window.next.version;
		results.push({name: 'Next.js', version: version});
	}

	// Vue 3
	if (window.__VUE__) {
		let version = '';
		try { version = window.__VUE__ && window.__VUE__.version || ''; } catch(e) {}
		results.push({name: 'Vue', version: version});
	}
	// Vue 2
	else if (window.Vue) {
		let version = '';
		try { version = window.Vue.version || ''; } catch(e) {}
		results.push({name: 'Vue', version: version});
	}
	// Vue instance on element
	else {
		const vueEl = document.querySelector('[data-v-]') ||
			document.querySelector('[data-vue-app]') ||
			document.querySelector('#app');
		if (vueEl && (vueEl.__vue__ || vueEl.__vue_app__)) {
			results.push({name: 'Vue', version: ''});
		}
	}

	// Nuxt
	if (window.__NUXT__ || window.$nuxt) {
		let version = '';
		try { if (window.__NUXT__ && window.__NUXT__.config) version = window.__NUXT__.config.public && window.__NUXT__.config.public.version || ''; } catch(e) {}
		results.push({name: 'Nuxt', version: version});
	}

	// Angular
	const ngVersion = document.querySelector('[ng-version]');
	if (ngVersion) {
		results.push({name: 'Angular', version: ngVersion.getAttribute('ng-version') || ''});
	} else if (window.ng || window.getAllAngularRootElements) {
		let version = '';
		try { version = window.ng && window.ng.coreTokens && '' || ''; } catch(e) {}
		results.push({name: 'Angular', version: version});
	}

	// AngularJS (1.x)
	if (window.angular) {
		let version = '';
		try { version = window.angular.version && window.angular.version.full || ''; } catch(e) {}
		results.push({name: 'AngularJS', version: version});
	}

	// Svelte
	const svelteEl = document.querySelector('[class*="svelte-"]');
	if (svelteEl) {
		results.push({name: 'Svelte', version: ''});
	}

	// SvelteKit
	if (window.__sveltekit_data || document.querySelector('[data-sveltekit-]')) {
		results.push({name: 'SvelteKit', version: ''});
	}

	// Remix
	if (window.__remixContext) {
		results.push({name: 'Remix', version: ''});
	}

	// Gatsby
	if (document.querySelector('#___gatsby')) {
		results.push({name: 'Gatsby', version: ''});
	}

	// Astro
	if (document.querySelector('[data-astro-cid]') || document.querySelector('astro-island')) {
		results.push({name: 'Astro', version: ''});
	}

	// Ember
	if (window.Ember) {
		let version = '';
		try { version = window.Ember.VERSION || ''; } catch(e) {}
		results.push({name: 'Ember', version: version});
	}

	// Backbone
	if (window.Backbone) {
		let version = '';
		try { version = window.Backbone.VERSION || ''; } catch(e) {}
		results.push({name: 'Backbone', version: version});
	}

	// jQuery (not a framework but commonly detected)
	if (window.jQuery || window.$) {
		let version = '';
		try { version = window.jQuery && window.jQuery.fn && window.jQuery.fn.jquery || ''; } catch(e) {}
		results.push({name: 'jQuery', version: version});
	}

	// Determine if SPA
	const isSPA = results.some(r =>
		['React', 'Vue', 'Angular', 'AngularJS', 'Svelte', 'Next.js', 'Nuxt', 'SvelteKit', 'Remix', 'Gatsby', 'Ember'].includes(r.name)
	);

	return JSON.stringify({frameworks: results, spa: isSPA});
}`

// DetectFrameworks inspects the current page for frontend framework markers.
// Returns all detected frameworks. The page should be loaded (call WaitLoad first).
func (p *Page) DetectFrameworks() ([]FrameworkInfo, error) {
	result, err := p.Eval(detectFrameworkJS)
	if err != nil {
		return nil, fmt.Errorf("scout: detect frameworks: %w", err)
	}

	var parsed struct {
		Frameworks []FrameworkInfo `json:"frameworks"`
		SPA        bool           `json:"spa"`
	}

	if err := json.Unmarshal([]byte(result.String()), &parsed); err != nil {
		return nil, fmt.Errorf("scout: parse framework detection: %w", err)
	}

	// Set SPA flag on all entries
	for i := range parsed.Frameworks {
		parsed.Frameworks[i].SPA = parsed.SPA
	}

	return parsed.Frameworks, nil
}

// DetectFramework returns the primary detected framework, or nil if none found.
// When multiple frameworks are detected (e.g. React + Next.js), the meta-framework
// (Next.js, Nuxt, SvelteKit, Remix, Gatsby) takes precedence.
func (p *Page) DetectFramework() (*FrameworkInfo, error) {
	frameworks, err := p.DetectFrameworks()
	if err != nil {
		return nil, err
	}

	if len(frameworks) == 0 {
		return nil, nil
	}

	// Prefer meta-frameworks over base frameworks
	metaFrameworks := map[string]bool{
		"Next.js": true, "Nuxt": true, "SvelteKit": true,
		"Remix": true, "Gatsby": true, "Astro": true,
	}

	for i := range frameworks {
		if metaFrameworks[frameworks[i].Name] {
			return &frameworks[i], nil
		}
	}

	return &frameworks[0], nil
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

// detectPWAJS evaluates in the page to detect PWA capabilities.
// It checks service workers, manifest link, protocol, and push support.
// Manifest content is fetched inline if the link tag exists.
const detectPWAJS = `async () => {
	const result = {
		has_service_worker: false,
		has_manifest: false,
		installable: false,
		https: location.protocol === 'https:',
		push_capable: false,
		manifest: null,
	};

	// Service Worker
	if ('serviceWorker' in navigator) {
		try {
			const regs = await navigator.serviceWorker.getRegistrations();
			result.has_service_worker = regs.length > 0;
		} catch(e) {}
	}

	// Push capability
	if ('PushManager' in window) {
		result.push_capable = true;
	}

	// Manifest link
	const manifestLink = document.querySelector('link[rel="manifest"]');
	if (manifestLink && manifestLink.href) {
		result.has_manifest = true;
		try {
			const resp = await fetch(manifestLink.href);
			if (resp.ok) {
				const m = await resp.json();
				result.manifest = {
					name: m.name || '',
					short_name: m.short_name || '',
					display: m.display || '',
					start_url: m.start_url || '',
					theme_color: m.theme_color || '',
					background_color: m.background_color || '',
					icons: Array.isArray(m.icons) ? m.icons.length : 0,
				};
			}
		} catch(e) {}
	}

	// Installability heuristic: manifest + SW + HTTPS
	result.installable = result.has_manifest && result.has_service_worker && result.https;

	return JSON.stringify(result);
}`

// DetectPWA checks whether the current page is a Progressive Web App.
// Detects service workers, web app manifest, installability, HTTPS, and push capability.
// The page should be loaded (call WaitLoad first).
func (p *Page) DetectPWA() (*PWAInfo, error) {
	if p == nil || p.page == nil {
		return nil, fmt.Errorf("scout: detect pwa: nil page")
	}

	result, err := p.Eval(detectPWAJS)
	if err != nil {
		return nil, fmt.Errorf("scout: detect pwa: %w", err)
	}

	var info PWAInfo
	if err := json.Unmarshal([]byte(result.String()), &info); err != nil {
		return nil, fmt.Errorf("scout: detect pwa: parse: %w", err)
	}

	return &info, nil
}
