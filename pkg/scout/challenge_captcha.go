package scout

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"
)

// solveCaptchaWithLLM takes a screenshot of the CAPTCHA area and sends it to
// an LLM provider for solving. Works for simple text/image CAPTCHAs.
func solveCaptchaWithLLM(page *Page, provider LLMProvider, challenge ChallengeInfo) error {
	if page == nil || page.page == nil {
		return fmt.Errorf("scout: challenge: captcha-llm: nil page")
	}
	if provider == nil {
		return fmt.Errorf("scout: challenge: captcha-llm: no LLM provider")
	}

	// Take a screenshot of the page for the LLM to analyze.
	screenshot, err := page.Screenshot()
	if err != nil {
		return fmt.Errorf("scout: challenge: captcha-llm: screenshot: %w", err)
	}

	imgB64 := base64.StdEncoding.EncodeToString(screenshot)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	systemPrompt := "You are a CAPTCHA analysis assistant. Analyze the screenshot and identify any CAPTCHA challenge. " +
		"If it's a text CAPTCHA, return ONLY the text you see. " +
		"If it's an image selection CAPTCHA, describe the grid positions (1-9, left-to-right, top-to-bottom) that match the prompt. " +
		"Return only the answer, no explanation."

	userPrompt := fmt.Sprintf("Challenge type: %s. Details: %s. Screenshot (base64): %s",
		challenge.Type, challenge.Details, imgB64)

	result, err := provider.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("scout: challenge: captcha-llm: %s: %w", provider.Name(), err)
	}

	// Try to find an input field near the CAPTCHA and type the result.
	if challenge.Selector != "" {
		input, inputErr := page.Element(challenge.Selector + " input[type='text']")
		if inputErr == nil && input != nil {
			if err := input.Input(result); err != nil {
				return fmt.Errorf("scout: challenge: captcha-llm: type answer: %w", err)
			}
			// Try to submit.
			submit, subErr := page.Element(challenge.Selector + " button[type='submit']")
			if subErr == nil && submit != nil {
				_ = submit.Click()
			}
			return nil
		}
	}

	return fmt.Errorf("scout: challenge: captcha-llm: could not find input to enter solution")
}

// solveRecaptchaV2 attempts to solve a reCAPTCHA v2 challenge by clicking the
// "I'm not a robot" checkbox. If an image challenge appears, it returns an error
// indicating that an LLM or external service is needed.
func solveRecaptchaV2(page *Page, _ ChallengeInfo) error {
	if page == nil || page.page == nil {
		return fmt.Errorf("scout: challenge: recaptcha-v2: nil page")
	}

	// Try clicking the reCAPTCHA checkbox via the iframe.
	result, err := page.Eval(`(function() {
		var frames = document.querySelectorAll('iframe[src*="google.com/recaptcha"]');
		for (var i = 0; i < frames.length; i++) {
			if (frames[i].src.indexOf('anchor') !== -1) {
				return frames[i].getBoundingClientRect().left + 28 + ',' + (frames[i].getBoundingClientRect().top + 28);
			}
		}
		return "";
	})()`)
	if err == nil && result.String() != "" {
		// Click at the checkbox coordinates.
		// The checkbox is typically at the top-left of the recaptcha iframe.
		checkbox, cbErr := page.Element(".recaptcha-checkbox")
		if cbErr == nil && checkbox != nil {
			_ = checkbox.Click()
		} else {
			// Fallback: try the g-recaptcha container.
			container, cErr := page.Element(".g-recaptcha")
			if cErr == nil && container != nil {
				_ = container.Click()
			}
		}
	}

	// Wait briefly for the checkbox to register.
	time.Sleep(2 * time.Second)

	// Check if an image challenge appeared.
	hasImageChallenge, _ := page.Has("iframe[src*='google.com/recaptcha'][src*='bframe']")
	if hasImageChallenge {
		// Image challenge requires LLM vision or external service.
		return fmt.Errorf("scout: challenge: recaptcha-v2: image challenge appeared, requires LLM or external service")
	}

	// Check if solved by looking for a response token.
	tokenResult, evalErr := page.Eval(`(function() {
		var textarea = document.getElementById('g-recaptcha-response');
		if (textarea && textarea.value) return textarea.value;
		return "";
	})()`)
	if evalErr == nil && tokenResult.String() != "" {
		return nil
	}

	return fmt.Errorf("scout: challenge: recaptcha-v2: could not solve automatically")
}

// solveHCaptcha attempts to solve an hCaptcha challenge by clicking the checkbox.
// If an image challenge appears, it returns an error indicating external help is needed.
func solveHCaptcha(page *Page, _ ChallengeInfo) error {
	if page == nil || page.page == nil {
		return fmt.Errorf("scout: challenge: hcaptcha: nil page")
	}

	// Try clicking the hCaptcha checkbox.
	container, err := page.Element(".h-captcha")
	if err == nil && container != nil {
		_ = container.Click()
	}

	// Wait for the checkbox interaction.
	time.Sleep(2 * time.Second)

	// Check if solved.
	tokenResult, evalErr := page.Eval(`(function() {
		var textarea = document.querySelector('[name="h-captcha-response"]');
		if (textarea && textarea.value) return textarea.value;
		return "";
	})()`)
	if evalErr == nil && tokenResult.String() != "" {
		return nil
	}

	return fmt.Errorf("scout: challenge: hcaptcha: could not solve automatically, requires LLM or external service")
}

// solveRecaptchaAudio is a stub for audio-based reCAPTCHA solving.
// Audio solving requires external speech-to-text services and is not implemented.
func solveRecaptchaAudio(_ *Page) error {
	return fmt.Errorf("scout: challenge: recaptcha-audio: not implemented, requires external speech-to-text service")
}
