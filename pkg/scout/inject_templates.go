package scout

import (
	"bytes"
	"fmt"
	"text/template"
)

// ScriptTemplate is a parameterized JavaScript template that can be rendered
// with dynamic data before injection.
type ScriptTemplate struct {
	Name        string
	Template    string
	Description string
}

// BuiltinTemplates maps template names to their ScriptTemplate definitions.
var BuiltinTemplates = map[string]ScriptTemplate{
	"extract-list": {
		Name:        "extract-list",
		Description: "Extract items from a list by container and field selectors",
		Template: `(function() {
  var container = document.querySelector('{{.container}}');
  if (!container) return [];
  var items = container.querySelectorAll('{{.item}}');
  var result = [];
  for (var i = 0; i < items.length; i++) {
    var obj = {};
    {{range $key, $sel := .fields}}
    var el = items[i].querySelector('{{$sel}}');
    obj['{{$key}}'] = el ? el.innerText.trim() : '';
    {{end}}
    result.push(obj);
  }
  return JSON.stringify(result);
})()`,
	},
	"fill-form": {
		Name:        "fill-form",
		Description: "Fill a form by field name/value pairs",
		Template: `(function() {
  var filled = 0;
  {{range $name, $value := .fields}}
  (function() {
    var el = document.querySelector('[name="{{$name}}"]') || document.getElementById('{{$name}}');
    if (el) {
      var nativeSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
      nativeSetter.call(el, '{{$value}}');
      el.dispatchEvent(new Event('input', {bubbles: true}));
      el.dispatchEvent(new Event('change', {bubbles: true}));
      filled++;
    }
  })();
  {{end}}
  return filled;
})()`,
	},
	"scroll-and-collect": {
		Name:        "scroll-and-collect",
		Description: "Scroll page and collect items matching a selector as they load",
		Template: `(function() {
  var selector = '{{.selector}}';
  var maxScrolls = {{if .maxScrolls}}{{.maxScrolls}}{{else}}20{{end}};
  var delayMs = {{if .delayMs}}{{.delayMs}}{{else}}500{{end}};
  return new Promise(function(resolve) {
    var seen = new Set();
    var results = [];
    var count = 0;
    function collect() {
      document.querySelectorAll(selector).forEach(function(el) {
        var text = el.innerText.trim();
        if (!seen.has(text) && text) { seen.add(text); results.push(text); }
      });
    }
    function step() {
      collect();
      if (count >= maxScrolls) { resolve(JSON.stringify(results)); return; }
      window.scrollTo(0, document.body.scrollHeight);
      count++;
      setTimeout(function() {
        var prev = results.length;
        collect();
        if (results.length === prev && count > 1) { resolve(JSON.stringify(results)); return; }
        step();
      }, delayMs);
    }
    step();
  });
})()`,
	},
}

// RenderTemplate renders a ScriptTemplate with the given data map using Go's
// text/template engine.
func RenderTemplate(tmpl ScriptTemplate, data map[string]any) (string, error) {
	t, err := template.New(tmpl.Name).Parse(tmpl.Template)
	if err != nil {
		return "", fmt.Errorf("scout: inject: parse template %q: %w", tmpl.Name, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("scout: inject: execute template %q: %w", tmpl.Name, err)
	}

	return buf.String(), nil
}

// InjectTemplate renders the named built-in template with data and evaluates it
// on the page, returning the result.
func InjectTemplate(page *Page, tmplName string, data map[string]any) (*EvalResult, error) {
	tmpl, ok := BuiltinTemplates[tmplName]
	if !ok {
		return nil, fmt.Errorf("scout: inject: unknown template %q", tmplName)
	}

	script, err := RenderTemplate(tmpl, data)
	if err != nil {
		return nil, err
	}

	result, err := page.Eval(script)
	if err != nil {
		return nil, fmt.Errorf("scout: inject: eval template %q: %w", tmplName, err)
	}

	return result, nil
}
