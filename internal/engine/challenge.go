package engine

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ChallengeType identifies a bot protection mechanism.
type ChallengeType string

const (
	ChallengeNone        ChallengeType = "none"
	ChallengeCloudflare  ChallengeType = "cloudflare"
	ChallengeTurnstile   ChallengeType = "turnstile"
	ChallengeRecaptchaV2 ChallengeType = "recaptcha_v2"
	ChallengeRecaptchaV3 ChallengeType = "recaptcha_v3"
	ChallengeHCaptcha    ChallengeType = "hcaptcha"
	ChallengeDataDome    ChallengeType = "datadome"
	ChallengePerimeterX  ChallengeType = "perimeterx"
	ChallengeAkamai      ChallengeType = "akamai"
	ChallengeAWSWAF      ChallengeType = "aws_waf"
)

// ChallengeInfo describes a detected bot protection challenge.
type ChallengeInfo struct {
	Type       ChallengeType `json:"type"`
	Detected   bool          `json:"detected"`
	Confidence float64       `json:"confidence"`
	Details    string        `json:"details"`
	Selector   string        `json:"selector,omitempty"`
}

const detectChallengeJS = `(function() {
	var results = [];

	function check(type, confidence, details, selector) {
		results.push({type: type, detected: true, confidence: Math.min(confidence, 1.0), details: details, selector: selector || ""});
	}

	// Cloudflare
	(function() {
		var score = 0;
		if (document.getElementById('cf-browser-verification')) score += 0.4;
		if (document.getElementById('challenge-running') || document.getElementById('cf-challenge-running')) score += 0.3;
		if (document.title && document.title.indexOf('Just a moment') !== -1) score += 0.2;
		if (typeof window.__cf_chl_opt !== 'undefined') score += 0.3;
		var scripts = document.querySelectorAll('script[src*="cdn-cgi/challenge-platform"]');
		if (scripts.length > 0) score += 0.3;
		if (score > 0) check('cloudflare', score, 'Cloudflare challenge detected', '#cf-browser-verification');
	})();

	// Turnstile
	(function() {
		var score = 0;
		if (document.querySelector('.cf-turnstile')) score += 0.5;
		if (document.querySelectorAll('iframe[src*="challenges.cloudflare.com/turnstile"]').length > 0) score += 0.4;
		if (document.querySelectorAll('script[src*="challenges.cloudflare.com/turnstile"]').length > 0) score += 0.4;
		if (score > 0) check('turnstile', score, 'Cloudflare Turnstile detected', '.cf-turnstile');
	})();

	// reCAPTCHA v2
	(function() {
		var score = 0;
		if (document.querySelector('.g-recaptcha')) score += 0.5;
		var scripts = document.querySelectorAll('script[src*="google.com/recaptcha/api.js"]');
		for (var i = 0; i < scripts.length; i++) {
			if (!scripts[i].src.match(/render=/)) score += 0.4;
		}
		if (document.querySelector('.recaptcha-checkbox')) score += 0.3;
		if (score > 0) check('recaptcha_v2', score, 'Google reCAPTCHA v2 detected', '.g-recaptcha');
	})();

	// reCAPTCHA v3
	(function() {
		var score = 0;
		var scripts = document.querySelectorAll('script[src*="google.com/recaptcha/api.js"]');
		for (var i = 0; i < scripts.length; i++) {
			if (scripts[i].src.match(/render=(?!explicit)/)) score += 0.7;
		}
		if (typeof window.grecaptcha !== 'undefined' && !document.querySelector('.g-recaptcha')) score += 0.3;
		if (score > 0) check('recaptcha_v3', score, 'Google reCAPTCHA v3 detected', '');
	})();

	// hCaptcha
	(function() {
		var score = 0;
		if (document.querySelector('.h-captcha')) score += 0.5;
		if (document.querySelectorAll('script[src*="hcaptcha.com"]').length > 0) score += 0.4;
		if (document.querySelectorAll('iframe[src*="hcaptcha.com"]').length > 0) score += 0.3;
		if (score > 0) check('hcaptcha', score, 'hCaptcha detected', '.h-captcha');
	})();

	// DataDome
	(function() {
		var score = 0;
		if (document.querySelectorAll('script[src*="js.datadome.co"]').length > 0) score += 0.5;
		if (document.cookie.indexOf('datadome') !== -1) score += 0.4;
		if (document.querySelectorAll('iframe[src*="geo.captcha-delivery.com"]').length > 0) score += 0.4;
		if (score > 0) check('datadome', score, 'DataDome detected', '');
	})();

	// PerimeterX
	(function() {
		var score = 0;
		if (document.querySelectorAll('script[src*="client.perimeterx.net"]').length > 0) score += 0.5;
		if (document.querySelectorAll('[src*="captcha.px-cdn.net"]').length > 0) score += 0.4;
		if (document.cookie.match(/_px[23]?=/)) score += 0.3;
		if (score > 0) check('perimeterx', score, 'PerimeterX detected', '');
	})();

	// Akamai
	(function() {
		var score = 0;
		if (document.cookie.indexOf('_abck=') !== -1) score += 0.5;
		if (document.querySelectorAll('script[src*="akamai"]').length > 0) score += 0.4;
		if (typeof window._akamai !== 'undefined') score += 0.3;
		if (score > 0) check('akamai', score, 'Akamai Bot Manager detected', '');
	})();

	// AWS WAF
	(function() {
		var score = 0;
		if (document.querySelectorAll('script[src*="awswaf"]').length > 0) score += 0.6;
		if (document.cookie.indexOf('aws-waf-token') !== -1) score += 0.5;
		if (document.querySelector('meta[name="awswaf"]')) score += 0.3;
		if (score > 0) check('aws_waf', score, 'AWS WAF detected', '');
	})();

	return JSON.stringify(results);
})()`

// DetectChallenges returns all bot protection challenges detected on the page.
func (p *Page) DetectChallenges() ([]ChallengeInfo, error) {
	if p == nil || p.page == nil {
		return nil, fmt.Errorf("scout: detect challenges: nil page")
	}

	result, err := p.Eval(detectChallengeJS)
	if err != nil {
		return nil, fmt.Errorf("scout: detect challenges: %w", err)
	}

	var challenges []ChallengeInfo
	if err := json.Unmarshal([]byte(result.String()), &challenges); err != nil {
		return nil, fmt.Errorf("scout: detect challenges: parse result: %w", err)
	}

	return challenges, nil
}

// DetectChallenge returns the highest-confidence challenge, or nil if none detected.
func (p *Page) DetectChallenge() (*ChallengeInfo, error) {
	challenges, err := p.DetectChallenges()
	if err != nil {
		return nil, err
	}

	if len(challenges) == 0 {
		return nil, nil //nolint:nilnil // no challenges detected is not an error
	}

	sort.Slice(challenges, func(i, j int) bool {
		return challenges[i].Confidence > challenges[j].Confidence
	})

	return &challenges[0], nil
}

// HasChallenge returns true if any bot protection challenge is detected.
func (p *Page) HasChallenge() (bool, error) {
	challenges, err := p.DetectChallenges()
	if err != nil {
		return false, err
	}

	return len(challenges) > 0, nil
}
