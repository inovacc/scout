package recipe

import (
	"context"
	"fmt"

	"github.com/go-rod/rod/lib/input"
	"github.com/inovacc/scout/pkg/scout"
)

// runAutomate executes an automation recipe step by step.
func runAutomate(ctx context.Context, browser *scout.Browser, r *Recipe) (*Result, error) {
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
				page, err = browser.NewPage(step.URL)
				if err != nil {
					return result, fmt.Errorf("recipe: step %d navigate: %w", i, err)
				}
				err = page.WaitLoad()
			} else {
				err = page.Navigate(step.URL)
				if err == nil {
					err = page.WaitLoad()
				}
			}

		case "click":
			if page == nil {
				return result, fmt.Errorf("recipe: step %d click: no page open", i)
			}
			el, findErr := page.Element(step.Selector)
			if findErr != nil {
				err = findErr
			} else {
				err = el.Click()
			}

		case "type":
			if page == nil {
				return result, fmt.Errorf("recipe: step %d type: no page open", i)
			}
			el, findErr := page.Element(step.Selector)
			if findErr != nil {
				err = findErr
			} else {
				err = el.Input(step.Text)
			}

		case "wait":
			if page == nil {
				return result, fmt.Errorf("recipe: step %d wait: no page open", i)
			}
			_, err = page.Element(step.Selector)

		case "screenshot":
			if page == nil {
				return result, fmt.Errorf("recipe: step %d screenshot: no page open", i)
			}
			var data []byte
			if step.FullPage {
				data, err = page.FullScreenshot()
			} else {
				data, err = page.Screenshot()
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
				return result, fmt.Errorf("recipe: step %d extract: no page open", i)
			}
			elements, findErr := page.Elements(step.Selector)
			if findErr != nil {
				err = findErr
			} else {
				var texts []string
				for _, el := range elements {
					text, textErr := el.Text()
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
				return result, fmt.Errorf("recipe: step %d eval: no page open", i)
			}
			evalResult, evalErr := page.Eval(step.Script)
			if evalErr != nil {
				err = evalErr
			} else if step.As != "" {
				result.Variables[step.As] = evalResult.Value
			}

		case "key":
			if page == nil {
				return result, fmt.Errorf("recipe: step %d key: no page open", i)
			}
			key := mapKeyName(step.Text)
			err = page.KeyPress(key)

		default:
			return result, fmt.Errorf("recipe: step %d unknown action %q", i, step.Action)
		}

		if err != nil {
			return result, fmt.Errorf("recipe: step %d %s: %w", i, step.Action, err)
		}
	}

	return result, nil
}

func mapKeyName(name string) input.Key {
	switch name {
	case "Enter":
		return input.Enter
	case "Tab":
		return input.Tab
	case "Escape":
		return input.Escape
	case "Space":
		return input.Space
	case "Backspace":
		return input.Backspace
	default:
		if len(name) == 1 {
			return input.Key(name[0])
		}
		return 0
	}
}
