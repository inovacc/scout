package scout

import (
	"fmt"
	"time"
)

// waitFrameworkReadyJS contains per-framework wait logic executed in the page.
// It takes a framework name and returns a Promise that resolves when the framework
// is ready, with a built-in 5-second timeout to avoid hanging.
const waitFrameworkReadyJS = `(name) => {
	return new Promise((resolve) => {
		const timeout = setTimeout(() => resolve('timeout'), 5000);

		function done(msg) {
			clearTimeout(timeout);
			resolve(msg || 'ready');
		}

		switch (name) {
		case 'React':
			// Wait for React root to have children (hydration complete)
			const checkReact = () => {
				const hook = window.__REACT_DEVTOOLS_GLOBAL_HOOK__;
				if (hook && hook.renderers && hook.renderers.size > 0) {
					return done('react-renderers');
				}
				const root = document.querySelector('[data-reactroot]') ||
					document.querySelector('#root') ||
					document.querySelector('#__next');
				if (root && root.childNodes.length > 0 && root.innerHTML.trim().length > 0) {
					return done('react-root-children');
				}
				return false;
			};
			if (checkReact()) return;
			const reactObs = new MutationObserver(() => { if (checkReact()) reactObs.disconnect(); });
			reactObs.observe(document.body || document.documentElement, {childList: true, subtree: true});
			break;

		case 'Next.js':
			// __NEXT_DATA__ present + document complete + router ready
			const checkNext = () => {
				if (window.__NEXT_DATA__ && document.readyState === 'complete') {
					if (window.next && window.next.router) return done('nextjs-router');
					return done('nextjs-data');
				}
				return false;
			};
			if (checkNext()) return;
			if (document.readyState !== 'complete') {
				window.addEventListener('load', () => { if (!checkNext()) setTimeout(() => done('nextjs-load'), 100); });
			} else {
				setTimeout(() => done('nextjs-fallback'), 200);
			}
			break;

		case 'Angular':
			// getAllAngularTestabilities whenStable
			if (window.getAllAngularTestabilities) {
				try {
					const testabilities = window.getAllAngularTestabilities();
					if (testabilities && testabilities.length > 0) {
						testabilities[0].whenStable(() => done('angular-stable'));
						return;
					}
				} catch(e) {}
			}
			// Fallback: ng-version attribute present means bootstrapped
			if (document.querySelector('[ng-version]')) {
				return done('angular-bootstrapped');
			}
			done('angular-fallback');
			break;

		case 'AngularJS':
			if (window.angular) {
				try {
					const injector = window.angular.element(document.body).injector();
					if (injector) return done('angularjs-ready');
				} catch(e) {}
			}
			done('angularjs-fallback');
			break;

		case 'Vue':
			// Vue.nextTick or check mounted app
			if (window.__VUE__ || window.Vue) {
				const vue = window.Vue || window.__VUE__;
				if (vue && typeof vue.nextTick === 'function') {
					vue.nextTick(() => done('vue-nexttick'));
					return;
				}
			}
			const appEl = document.querySelector('#app');
			if (appEl && (appEl.__vue__ || appEl.__vue_app__)) {
				return done('vue-mounted');
			}
			done('vue-fallback');
			break;

		case 'Nuxt':
			// __NUXT__ present means data hydrated
			if (window.__NUXT__) {
				if (window.$nuxt && window.$nuxt.$loading) {
					// Wait for loading to finish
					const checkLoading = () => {
						if (!window.$nuxt.$loading.show) return done('nuxt-loaded');
						setTimeout(checkLoading, 50);
					};
					checkLoading();
					return;
				}
				return done('nuxt-data');
			}
			done('nuxt-fallback');
			break;

		case 'Svelte':
		case 'SvelteKit':
			// No specific hydration signal - DOM stable is the best we can do
			done('svelte-immediate');
			break;

		case 'Remix':
			if (window.__remixContext) return done('remix-context');
			done('remix-fallback');
			break;

		case 'Gatsby':
			// Gatsby uses React under the hood
			const gatsbyRoot = document.querySelector('#___gatsby');
			if (gatsbyRoot && gatsbyRoot.childNodes.length > 0) {
				return done('gatsby-hydrated');
			}
			const gatsbyObs = new MutationObserver(() => {
				if (gatsbyRoot && gatsbyRoot.childNodes.length > 0) {
					gatsbyObs.disconnect();
					done('gatsby-hydrated');
				}
			});
			if (gatsbyRoot) {
				gatsbyObs.observe(gatsbyRoot, {childList: true});
			} else {
				done('gatsby-fallback');
			}
			break;

		case 'Astro':
			// Astro islands hydrate independently; just ensure document is complete
			if (document.readyState === 'complete') return done('astro-complete');
			window.addEventListener('load', () => done('astro-loaded'));
			break;

		default:
			// Unknown framework - resolve immediately
			done('unknown-framework');
		}
	});
}`

// WaitFrameworkReady detects the page's framework and waits for it to be fully ready.
// Falls back to WaitLoad + a short WaitDOMStable if no framework is detected.
func (p *Page) WaitFrameworkReady() error {
	if p == nil || p.page == nil {
		return fmt.Errorf("scout: wait framework ready: nil page")
	}

	// Detect the primary framework
	fw, err := p.DetectFramework()
	if err != nil || fw == nil {
		// No framework detected — standard wait fallback
		if loadErr := p.WaitLoad(); loadErr != nil {
			return fmt.Errorf("scout: wait framework ready: %w", loadErr)
		}
		if stableErr := p.WaitDOMStable(300*time.Millisecond, 0.1); stableErr != nil {
			// DOM stable can timeout on simple pages; not fatal
			return nil
		}
		return nil
	}

	// Framework-specific wait via JS
	_, err = p.Eval(fmt.Sprintf(`(name) => %s(name)`, waitFrameworkReadyJS), fw.Name)
	if err != nil {
		// JS wait failed — fall back to standard wait
		_ = p.WaitLoad()
		return nil
	}

	return nil
}
