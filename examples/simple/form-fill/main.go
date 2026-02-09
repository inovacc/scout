// Example: form-fill
// Demonstrates form detection, filling, and submission.
// Targets quotes.toscrape.com login page.
package main

import (
	"fmt"
	"log"

	"github.com/inovacc/scout"
)

// LoginData uses form tags to map struct fields to form field names.
type LoginData struct {
	Username string `form:"username"`
	Password string `form:"password"`
}

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

	// Detect the login form.
	form, err := page.DetectForm("form")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Form action:", form.Action)
	fmt.Println("Form method:", form.Method)
	fmt.Println("Fields:")

	for _, f := range form.Fields {
		fmt.Printf("  name=%s type=%s required=%v\n", f.Name, f.Type, f.Required)
	}

	// Check for CSRF token.
	token, err := form.CSRFToken()
	if err == nil {
		fmt.Println("CSRF token found:", token[:20]+"...")
	}

	// Fill using a struct with form tags.
	if err := form.FillStruct(&LoginData{
		Username: "testuser",
		Password: "testpass",
	}); err != nil {
		log.Fatal(err)
	}

	// Submit the form.
	if err := form.Submit(); err != nil {
		log.Fatal(err)
	}

	title, _ := page.Title()
	fmt.Println("After submit, title:", title)
}
