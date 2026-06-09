// Command loginprobe exercises Scout's automation maturity: a real login flow
// (fill + submit + verify) plus a detailed bot-detector result dump.
// Not part of the build (hacks/). Run: go run ./hacks/loginprobe/
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	b, err := scout.New(scout.WithStealth(), scout.WithHeadless(true), scout.WithNoSandbox())
	if err != nil {
		fmt.Println("LAUNCH ERROR:", err)
		os.Exit(1)
	}
	defer func() { _ = b.Close() }()

	// ---- Automation maturity: real login flow (Scout's native Element API) ----
	fmt.Println("===== AUTOMATION: practicetestautomation.com login =====")

	lp, err := b.NewPage("https://practicetestautomation.com/practice-test-login/")
	if err != nil {
		fmt.Println("  navigate error:", err)
	} else {
		_ = lp.WaitLoad()
		time.Sleep(2 * time.Second)

		step := func(sel, text string) {
			el, e := lp.Element(sel)
			if e != nil {
				fmt.Printf("  element %s NOT FOUND: %v\n", sel, e)
				return
			}

			if text == "" {
				if e := el.Click(); e != nil {
					fmt.Printf("  click %s error: %v\n", sel, e)
				} else {
					fmt.Printf("  clicked %s\n", sel)
				}

				return
			}

			if e := el.Input(text); e != nil {
				fmt.Printf("  input %s error: %v\n", sel, e)
			} else {
				fmt.Printf("  typed into %s\n", sel)
			}
		}

		step("#username", "student")
		step("#password", "Password123")
		step("#submit", "")

		time.Sleep(3 * time.Second)

		if r, e := lp.Eval(`() => location.href + ' :: ' + ((document.body.innerText.includes('successfully logged in') || document.body.innerText.includes('Logged In Successfully')) ? 'LOGIN-SUCCESS' : 'NO-SUCCESS-TEXT')`); e == nil {
			fmt.Println("  RESULT:", r.String())
		}

		_ = lp.Close()
	}

	// ---- Detection detail: bot.incolumitas result tables + behavioral score ----
	fmt.Println("\n===== DETECTION DETAIL: bot.incolumitas =====")

	dp, err := b.NewPage("https://bot.incolumitas.com")
	if err != nil {
		fmt.Println("  navigate error:", err)
	} else {
		_ = dp.WaitLoad()
		time.Sleep(17 * time.Second) // behavioral score updates through 15s of "browsing"

		if r, e := dp.Eval(`() => Array.from(document.querySelectorAll('table')).map(t=>t.innerText.replace(/\t/g,': ')).join('\n----\n').slice(0,2600)`); e == nil {
			fmt.Println(r.String())
		}

		if r, e := dp.Eval(`() => { const m = document.body.innerText.match(/score[^0-9]*([01](?:\.[0-9]+)?)/i); return m ? ('behavioral-ish score match: '+m[0]) : 'behavioral score not in visible text'; }`); e == nil {
			fmt.Println("  ", r.String())
		}

		_ = dp.Close()
	}
}
