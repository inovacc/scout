package runbook

import (
	"context"
	"fmt"

	input2 "github.com/inovacc/scout/internal/engine/lib/input"
	"github.com/inovacc/scout/pkg/scout"
)

// runAutomate executes an automation runbook step by step.
func runAutomate(ctx context.Context, browser *scout.Browser, r *Runbook) (*Result, error) {
	result := &Result{
		Variables:   make(map[string]any),
		Screenshots: make(map[string][]byte),
	}

	var page *scout.Page

	for i, step := range r.Steps {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		var err error

		switch step.Action {
		case "navigate":
			if page == nil {
				page, err = browser.NewPage(step.URL) //nolint:contextcheck
				if err != nil {
					return result, fmt.Errorf("runbook: step %d navigate: %w", i, err)
				}

				err = page.WaitLoad() //nolint:contextcheck
			} else {
				err = page.Navigate(step.URL) //nolint:contextcheck
				if err == nil {
					err = page.WaitLoad() //nolint:contextcheck
				}
			}

		case "click":
			if page == nil {
				return result, fmt.Errorf("runbook: step %d click: no page open", i)
			}

			el, findErr := page.Element(step.Selector) //nolint:contextcheck
			if findErr != nil {
				err = findErr
			} else {
				err = el.Click() //nolint:contextcheck
			}

		case "type":
			if page == nil {
				return result, fmt.Errorf("runbook: step %d type: no page open", i)
			}

			el, findErr := page.Element(step.Selector) //nolint:contextcheck
			if findErr != nil {
				err = findErr
			} else {
				err = el.Input(step.Text) //nolint:contextcheck
			}

		case "wait":
			if page == nil {
				return result, fmt.Errorf("runbook: step %d wait: no page open", i)
			}

			_, err = page.Element(step.Selector) //nolint:contextcheck

		case "screenshot":
			if page == nil {
				return result, fmt.Errorf("runbook: step %d screenshot: no page open", i)
			}

			var data []byte
			if step.FullPage {
				data, err = page.FullScreenshot() //nolint:contextcheck
			} else {
				data, err = page.Screenshot() //nolint:contextcheck
			}

			if err == nil {
				name := step.Name
				if name == "" {
					name = fmt.Sprintf("step_%d", i)
				}

				result.Screenshots[name] = data
			}

		case "extract":
			if page == nil {
				return result, fmt.Errorf("runbook: step %d extract: no page open", i)
			}

			elements, findErr := page.Elements(step.Selector) //nolint:contextcheck
			if findErr != nil {
				err = findErr
			} else {
				var texts []string

				for _, el := range elements {
					text, textErr := el.Text() //nolint:contextcheck
					if textErr == nil {
						texts = append(texts, text)
					}
				}

				if step.As != "" {
					result.Variables[step.As] = texts
				}
			}

		case "eval":
			if page == nil {
				return result, fmt.Errorf("runbook: step %d eval: no page open", i)
			}

			evalResult, evalErr := page.Eval(step.Script) //nolint:contextcheck
			if evalErr != nil {
				err = evalErr
			} else if step.As != "" {
				result.Variables[step.As] = evalResult.Value
			}

		case "key":
			if page == nil {
				return result, fmt.Errorf("runbook: step %d key: no page open", i)
			}

			key := mapKeyName(step.Text)
			err = page.KeyPress(key) //nolint:contextcheck

		default:
			return result, fmt.Errorf("runbook: step %d unknown action %q", i, step.Action)
		}

		if err != nil {
			return result, fmt.Errorf("runbook: step %d %s: %w", i, step.Action, err)
		}
	}

	return result, nil
}

func mapKeyName(name string) input2.Key {
	switch name {
	case "Enter":
		return input2.Enter
	case "Tab":
		return input2.Tab
	case "Escape":
		return input2.Escape
	case "Space":
		return input2.Space
	case "Backspace":
		return input2.Backspace
	default:
		if len(name) == 1 {
			return input2.Key(name[0])
		}

		return 0
	}
}
