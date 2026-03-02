package scout

import (
	"encoding/json"
	"fmt"
)

// FrameworkInfo describes a detected frontend framework on the current page.
type FrameworkInfo struct {
	Name    string `json:"name"`              // e.g. "React", "Vue", "Angular", "Svelte", "Next.js"
	Version string `json:"version,omitempty"` // version string if detectable
	SPA     bool   `json:"spa"`               // true if the page appears to be a single-page application
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
		SPA        bool            `json:"spa"`
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
		return nil, nil //nolint:nilnil // no frameworks detected is not an error
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

// TechStack describes the full technology stack detected on a page.
type TechStack struct {
	Frameworks   []FrameworkInfo `json:"frameworks,omitempty"`
	CSSFramework string          `json:"css_framework,omitempty"`
	BuildTool    string          `json:"build_tool,omitempty"`
	CMS          string          `json:"cms,omitempty"`
	Analytics    []string        `json:"analytics,omitempty"`
	CDN          string          `json:"cdn,omitempty"`
}

// detectTechStackJS probes the page DOM for CSS frameworks, build tools, CMS, analytics, and CDN.
const detectTechStackJS = `() => {
	const result = {
		css_framework: '',
		build_tool: '',
		cms: '',
		analytics: [],
		cdn: '',
	};

	// --- CSS Frameworks ---
	// Tailwind: count elements with typical Tailwind utility classes
	const twPatterns = ['flex','p-4','bg-blue-500','text-center','mt-2','mb-4','px-4','py-2','rounded','w-full','grid','items-center','justify-center','text-sm','font-bold'];
	let twHits = 0;
	for (const cls of twPatterns) {
		if (document.querySelector('.' + CSS.escape(cls))) twHits++;
	}
	if (twHits >= 3) result.css_framework = 'Tailwind';

	// Bootstrap
	if (!result.css_framework) {
		const hasBootstrapClass = document.querySelector('.container .row') || document.querySelector('[class*="col-"]') || document.querySelector('.btn');
		const hasBootstrapLink = !!Array.from(document.querySelectorAll('link[href]')).find(l => l.href.includes('bootstrap'));
		if (hasBootstrapClass || hasBootstrapLink) result.css_framework = 'Bootstrap';
	}

	// Material UI
	if (!result.css_framework && document.querySelector('[class*="Mui"]')) {
		result.css_framework = 'Material UI';
	}

	// Chakra UI
	if (!result.css_framework && document.querySelector('[class*="chakra-"]')) {
		result.css_framework = 'Chakra UI';
	}

	// Bulma
	if (!result.css_framework) {
		const hasBulmaClass = document.querySelector('.is-primary') || document.querySelector('.column');
		const hasBulmaLink = !!Array.from(document.querySelectorAll('link[href]')).find(l => l.href.includes('bulma'));
		if (hasBulmaClass || hasBulmaLink) result.css_framework = 'Bulma';
	}

	// --- Build Tools ---
	const scripts = Array.from(document.querySelectorAll('script[src]'));
	const scriptSrcs = scripts.map(s => s.src || '');

	if (scriptSrcs.some(s => s.includes('/@vite/') || s.includes('?v='))) {
		result.build_tool = 'Vite';
	} else {
		// Vite also uses type=module with hashed filenames
		const moduleScripts = document.querySelectorAll('script[type="module"][src]');
		for (const s of moduleScripts) {
			if (/\/assets\/.*-[a-zA-Z0-9]{8,}\.js/.test(s.src)) {
				result.build_tool = 'Vite';
				break;
			}
		}
	}

	if (!result.build_tool && scriptSrcs.some(s => s.includes('/chunk-') || s.includes('bundle.js') || s.includes('/static/js/'))) {
		result.build_tool = 'Webpack';
	}
	if (!result.build_tool && scriptSrcs.some(s => s.includes('/parcel-'))) {
		result.build_tool = 'Parcel';
	}
	if (!result.build_tool && scriptSrcs.some(s => s.includes('esbuild'))) {
		result.build_tool = 'esbuild';
	}

	// --- CMS ---
	const allSrcsAndHrefs = Array.from(document.querySelectorAll('[src],[href]')).map(e => e.src || e.href || '');

	if (allSrcsAndHrefs.some(s => s.includes('wp-content') || s.includes('wp-json'))) {
		result.cms = 'WordPress';
	} else if (typeof window.Drupal !== 'undefined' || allSrcsAndHrefs.some(s => s.includes('drupal.js'))) {
		result.cms = 'Drupal';
	} else if (typeof window.Shopify !== 'undefined' || allSrcsAndHrefs.some(s => s.includes('cdn.shopify.com'))) {
		result.cms = 'Shopify';
	} else if (document.querySelector('[data-wf-site]') || document.querySelector('[data-wf-page]')) {
		result.cms = 'Webflow';
	} else if (allSrcsAndHrefs.some(s => s.includes('static.squarespace.com'))) {
		result.cms = 'Squarespace';
	}

	// --- Analytics ---
	if (typeof window.gtag === 'function' || allSrcsAndHrefs.some(s => s.includes('google-analytics.com') || s.includes('googletagmanager.com'))) {
		result.analytics.push('Google Analytics');
	}
	if (typeof window.dataLayer !== 'undefined' && Array.isArray(window.dataLayer) && window.dataLayer.some && window.dataLayer.some(d => d['gtm.start'])) {
		if (!result.analytics.includes('Google Analytics')) result.analytics.push('Google Tag Manager');
	}
	if (typeof window.analytics !== 'undefined' && typeof window.analytics.identify === 'function') {
		result.analytics.push('Segment');
	}
	if (typeof window.mixpanel !== 'undefined') {
		result.analytics.push('Mixpanel');
	}
	if (typeof window.hj === 'function' || allSrcsAndHrefs.some(s => s.includes('hotjar.com'))) {
		result.analytics.push('Hotjar');
	}

	// --- CDN ---
	if (allSrcsAndHrefs.some(s => s.includes('cdnjs.cloudflare.com'))) {
		result.cdn = 'Cloudflare';
	} else if (allSrcsAndHrefs.some(s => s.includes('_next/')) || document.querySelector('meta[name="generator"][content*="Vercel"]')) {
		result.cdn = 'Vercel';
	} else if (document.querySelector('meta[name="generator"][content*="Netlify"]') || allSrcsAndHrefs.some(s => s.includes('netlify'))) {
		result.cdn = 'Netlify';
	} else if (allSrcsAndHrefs.some(s => s.includes('cloudfront.net'))) {
		result.cdn = 'AWS CloudFront';
	}

	return JSON.stringify(result);
}`

// DetectTechStack inspects the current page for CSS frameworks, build tools, CMS,
// analytics, and CDN. Also includes framework detection from DetectFrameworks.
// The page should be loaded (call WaitLoad first).
func (p *Page) DetectTechStack() (*TechStack, error) {
	if p == nil || p.page == nil {
		return nil, fmt.Errorf("scout: detect tech stack: nil page")
	}

	// Get frameworks
	frameworks, err := p.DetectFrameworks()
	if err != nil {
		return nil, fmt.Errorf("scout: detect tech stack: %w", err)
	}

	// Get tech stack details
	result, err := p.Eval(detectTechStackJS)
	if err != nil {
		return nil, fmt.Errorf("scout: detect tech stack: %w", err)
	}

	var stack TechStack
	if err := json.Unmarshal([]byte(result.String()), &stack); err != nil {
		return nil, fmt.Errorf("scout: detect tech stack: parse: %w", err)
	}

	stack.Frameworks = frameworks
	if len(stack.Analytics) == 0 {
		stack.Analytics = nil
	}

	return &stack, nil
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

// detectRenderModeJS evaluates in the page to classify the rendering mode.
const detectRenderModeJS = `() => {
	const r = {mode: 'unknown', hydrated: false, details: ''};

	const body = document.body;
	const bodyText = (body && body.innerText || '').trim();
	const hasContent = bodyText.length > 50;

	// SSG: Gatsby
	const gatsby = document.querySelector('#___gatsby');
	if (gatsby) {
		r.mode = 'SSG';
		r.details = 'Gatsby (#___gatsby)';
		if (window.__REACT_DEVTOOLS_GLOBAL_HOOK__ && window.__REACT_DEVTOOLS_GLOBAL_HOOK__.renderers && window.__REACT_DEVTOOLS_GLOBAL_HOOK__.renderers.size > 0) r.hydrated = true;
		return JSON.stringify(r);
	}

	// SSG: Astro
	if (document.querySelector('astro-island') || document.querySelector('[data-astro-cid]')) {
		r.mode = 'SSG';
		r.details = 'Astro (astro-island)';
		return JSON.stringify(r);
	}

	// SSG: generator meta (Hugo, Jekyll, Eleventy, Hexo)
	const generator = document.querySelector('meta[name="generator"]');
	if (generator) {
		const gen = (generator.getAttribute('content') || '').toLowerCase();
		if (gen.includes('hugo') || gen.includes('jekyll') || gen.includes('eleventy') || gen.includes('hexo')) {
			r.mode = 'SSG';
			r.details = 'Generator: ' + generator.getAttribute('content');
			return JSON.stringify(r);
		}
	}

	// Next.js
	if (window.__NEXT_DATA__) {
		const nd = window.__NEXT_DATA__;
		if (nd.isFallback === true) {
			r.mode = 'ISR'; r.details = 'Next.js (__NEXT_DATA__.isFallback)'; r.hydrated = true;
			return JSON.stringify(r);
		}
		if (nd.props && nd.props.pageProps && nd.props.pageProps.__N_SSP === true) {
			r.mode = 'SSR'; r.details = 'Next.js (__N_SSP)'; r.hydrated = true;
			return JSON.stringify(r);
		}
		r.mode = 'SSG'; r.details = 'Next.js (static props)'; r.hydrated = true;
		return JSON.stringify(r);
	}

	// Nuxt SSR
	if (window.__NUXT__ && window.__NUXT__.serverRendered === true) {
		r.mode = 'SSR'; r.details = 'Nuxt (serverRendered)'; r.hydrated = true;
		return JSON.stringify(r);
	}

	// Vue SSR
	if (document.querySelector('[data-server-rendered="true"]')) {
		r.mode = 'SSR'; r.details = 'Vue (data-server-rendered)';
		const appEl = document.querySelector('[data-server-rendered]');
		if (appEl && (appEl.__vue__ || appEl.__vue_app__)) r.hydrated = true;
		return JSON.stringify(r);
	}

	// Angular Universal SSR
	if (document.querySelector('[ng-server-context]')) {
		r.mode = 'SSR'; r.details = 'Angular Universal (ng-server-context)';
		r.hydrated = !!document.querySelector('[ng-version]');
		return JSON.stringify(r);
	}

	// CSR: empty root with framework globals
	const root = document.querySelector('#root') || document.querySelector('#app');
	if (root) {
		const rootText = (root.innerText || '').trim();
		const hasFramework = !!(
			window.__REACT_DEVTOOLS_GLOBAL_HOOK__ ||
			window.__VUE__ || window.Vue ||
			window.angular || document.querySelector('[ng-version]')
		);
		if (hasFramework && rootText.length === 0) {
			r.mode = 'CSR'; r.details = 'Empty root with framework globals';
			return JSON.stringify(r);
		}
		if (hasFramework) {
			r.mode = 'CSR'; r.hydrated = true; r.details = 'Framework with content (no SSR markers)';
			return JSON.stringify(r);
		}
	}

	// Fallback: substantial content with no framework → static
	if (hasContent) {
		r.mode = 'SSG'; r.details = 'Static HTML (no framework detected)';
		return JSON.stringify(r);
	}

	return JSON.stringify(r);
}`

// DetectRenderMode classifies the current page's rendering mode (CSR, SSR, SSG, ISR).
// The page should be loaded (call WaitLoad first).
func (p *Page) DetectRenderMode() (*RenderInfo, error) {
	if p == nil || p.page == nil {
		return nil, fmt.Errorf("scout: detect render mode: nil page")
	}

	result, err := p.Eval(detectRenderModeJS)
	if err != nil {
		return nil, fmt.Errorf("scout: detect render mode: %w", err)
	}

	var info RenderInfo
	if err := json.Unmarshal([]byte(result.String()), &info); err != nil {
		return nil, fmt.Errorf("scout: detect render mode: parse: %w", err)
	}

	return &info, nil
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
