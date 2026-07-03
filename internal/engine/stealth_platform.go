package engine

import (
	"runtime"
	"strconv"
)

// defaultStealthUA returns a User-Agent coherent with the real host OS, used under
// WithStealth so the presented platform (UA) agrees with the host's real platform,
// fonts and WebGL — closing the cross-signal "Mac UA on a Windows host" tell that
// sophisticated detectors (CreepJS) flag. darwin keeps Scout's historical profile
// byte-for-byte (the only OS not runtime-validated for this change); windows and
// linux get a host-matching UA. The Chrome major is pinned current — bump on
// Chrome stable releases (ideally derive from the launched browser in a follow-up).
func defaultStealthUA() string {
	switch runtime.GOOS {
	case "windows":
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	case "linux":
		return "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	default: // darwin and unknown: unchanged historical profile
		return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36"
	}
}

// stealthWebGLOverrideJS returns a document-start script that overrides the WebGL
// UNMASKED_VENDOR (37445) / UNMASKED_RENDERER (37446) to values coherent with the
// host OS. Injected after the evasion bundle so it wins. On darwin it returns the
// same Mac values the bundle already sets (a no-op); windows/linux get host-matching
// GPU strings so the GPU agrees with the presented platform.
func stealthWebGLOverrideJS() string {
	vendor, renderer := "Intel Inc.", "Intel Iris OpenGL Engine" // darwin (unchanged)

	switch runtime.GOOS {
	case "windows":
		vendor, renderer = "Google Inc. (Intel)", "ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0, D3D11)"
	case "linux":
		vendor, renderer = "Google Inc. (Intel)", "ANGLE (Intel, Mesa Intel(R) UHD Graphics 630 (CFL GT2), OpenGL 4.6)"
	}

	return `(function(){var V=` + strconv.Quote(vendor) + `,R=` + strconv.Quote(renderer) + `;` +
		`function patch(proto){try{if(!proto)return;var gp=proto.getParameter;proto.getParameter=function(p){if(p===37445)return V;if(p===37446)return R;return gp.call(this,p)}}catch(e){}}` +
		`patch(typeof WebGLRenderingContext!=='undefined'?WebGLRenderingContext.prototype:null);` +
		`patch(typeof WebGL2RenderingContext!=='undefined'?WebGL2RenderingContext.prototype:null);})()`
}
