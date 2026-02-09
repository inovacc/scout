// Example: form-wizard
// Demonstrates multi-step form wizard automation.
// Uses quotes.toscrape.com as a target (login form as single step).
package main

import (
	"fmt"
	"log"

	"github.com/inovacc/scout"
)

func main() {
	browser, err := scout.New()
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage("https://quotes.toscrape.com/login")
	if err != nil {
		log.Fatal(err)
	}

	// Define wizard steps. Each step targets a form, fills data,
	// and optionally clicks a "next" button or submits.
	wizard := page.NewFormWizard(
		// Step 1: Fill and submit the login form.
		// WaitFor ensures the form is loaded before filling.
		// NextSelector is empty, so the form is submitted instead.
		scout.WizardStep{
			FormSelector: "form",
			WaitFor:      "form",
			Data: map[string]string{
				"username": "testuser",
				"password": "testpass",
			},
			// Empty NextSelector means submit the form on this step.
		},
	)

	// Run all wizard steps sequentially.
	if err := wizard.Run(); err != nil {
		log.Fatal("Wizard failed:", err)
	}

	// Verify the result.
	title, _ := page.Title()
	url, _ := page.URL()

	fmt.Println("Title:", title)
	fmt.Println("URL:", url)

	// Check if login succeeded by looking for a logout link.
	hasLogout, _ := page.Has("a[href='/logout']")
	fmt.Println("Logged in:", hasLogout)
}
