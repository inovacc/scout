package engine

import "fmt"

// HelperTableExtract is a self-executing JS script that extracts all HTML tables
// as JSON arrays of {headers, rows} objects. Results are stored in window.__scout.tables.
const HelperTableExtract = `(function() {
  window.__scout = window.__scout || {};
  window.__scout.extractTables = function() {
    var tables = document.querySelectorAll('table');
    var result = [];
    for (var i = 0; i < tables.length; i++) {
      var table = tables[i];
      var headers = [];
      var headerCells = table.querySelectorAll('thead th, thead td, tr:first-child th');
      for (var h = 0; h < headerCells.length; h++) {
        headers.push(headerCells[h].innerText.trim());
      }
      var rows = [];
      var bodyRows = table.querySelectorAll('tbody tr');
      if (bodyRows.length === 0) {
        var allRows = table.querySelectorAll('tr');
        bodyRows = headers.length > 0 ? Array.prototype.slice.call(allRows, 1) : allRows;
      }
      for (var r = 0; r < bodyRows.length; r++) {
        var cells = bodyRows[r].querySelectorAll('td, th');
        var row = [];
        for (var c = 0; c < cells.length; c++) {
          row.push(cells[c].innerText.trim());
        }
        rows.push(row);
      }
      result.push({headers: headers, rows: rows});
    }
    return result;
  };
})();`

// HelperInfiniteScroll is a self-executing JS script that scrolls to the bottom
// of the page repeatedly until no new content loads. Configurable via
// window.__scout.infiniteScroll(maxScrolls, delayMs).
const HelperInfiniteScroll = `(function() {
  window.__scout = window.__scout || {};
  window.__scout.infiniteScroll = function(maxScrolls, delayMs) {
    maxScrolls = maxScrolls || 50;
    delayMs = delayMs || 1000;
    return new Promise(function(resolve) {
      var count = 0;
      var lastHeight = document.body.scrollHeight;
      function step() {
        if (count >= maxScrolls) { resolve({scrolls: count, stopped: 'max_reached'}); return; }
        window.scrollTo(0, document.body.scrollHeight);
        count++;
        setTimeout(function() {
          var newHeight = document.body.scrollHeight;
          if (newHeight === lastHeight) { resolve({scrolls: count, stopped: 'no_new_content'}); return; }
          lastHeight = newHeight;
          step();
        }, delayMs);
      }
      step();
    });
  };
})();`

// HelperShadowQuery is a self-executing JS script that provides recursive
// shadow DOM querying via window.__scout.shadowQuery(selector) and
// window.__scout.shadowQueryAll(selector).
const HelperShadowQuery = `(function() {
  window.__scout = window.__scout || {};
  function queryShadow(root, selector) {
    var result = root.querySelector(selector);
    if (result) return result;
    var children = root.querySelectorAll('*');
    for (var i = 0; i < children.length; i++) {
      if (children[i].shadowRoot) {
        result = queryShadow(children[i].shadowRoot, selector);
        if (result) return result;
      }
    }
    return null;
  }
  function queryAllShadow(root, selector) {
    var results = Array.from(root.querySelectorAll(selector));
    var children = root.querySelectorAll('*');
    for (var i = 0; i < children.length; i++) {
      if (children[i].shadowRoot) {
        results = results.concat(queryAllShadow(children[i].shadowRoot, selector));
      }
    }
    return results;
  }
  window.__scout.shadowQuery = function(selector) { return queryShadow(document, selector); };
  window.__scout.shadowQueryAll = function(selector) { return queryAllShadow(document, selector); };
})();`

// HelperWaitForSelector is a self-executing JS script that polls for a CSS
// selector to appear in the DOM. window.__scout.waitForSelector(selector, timeoutMs)
// returns a Promise that resolves to the element or rejects on timeout.
const HelperWaitForSelector = `(function() {
  window.__scout = window.__scout || {};
  window.__scout.waitForSelector = function(selector, timeoutMs) {
    timeoutMs = timeoutMs || 10000;
    return new Promise(function(resolve, reject) {
      var el = document.querySelector(selector);
      if (el) { resolve(el); return; }
      var elapsed = 0;
      var interval = 100;
      var timer = setInterval(function() {
        elapsed += interval;
        el = document.querySelector(selector);
        if (el) { clearInterval(timer); resolve(el); return; }
        if (elapsed >= timeoutMs) { clearInterval(timer); reject(new Error('timeout waiting for ' + selector)); }
      }, interval);
    });
  };
})();`

// HelperClickAll is a self-executing JS script that clicks all elements matching
// a CSS selector. window.__scout.clickAll(selector) returns the count of clicked elements.
const HelperClickAll = `(function() {
  window.__scout = window.__scout || {};
  window.__scout.clickAll = function(selector) {
    var elements = document.querySelectorAll(selector);
    var count = 0;
    for (var i = 0; i < elements.length; i++) {
      elements[i].click();
      count++;
    }
    return count;
  };
})();`

// allHelpers is the ordered list of all built-in JS helpers.
var allHelpers = []string{
	HelperTableExtract,
	HelperInfiniteScroll,
	HelperShadowQuery,
	HelperWaitForSelector,
	HelperClickAll,
}

// InjectHelper evaluates a single JS helper string on the given page.
func InjectHelper(page *Page, helper string) error {
	if _, err := page.Eval(helper); err != nil {
		return fmt.Errorf("scout: inject: helper: %w", err)
	}

	return nil
}

// InjectAllHelpers injects all built-in helpers onto the page, exposing them
// under the window.__scout namespace. Safe to call multiple times (idempotent).
func InjectAllHelpers(page *Page) error {
	for _, h := range allHelpers {
		if err := InjectHelper(page, h); err != nil {
			return err
		}
	}

	return nil
}
