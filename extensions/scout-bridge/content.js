// Scout Bridge — content script
// Provides window.__scout API for Go↔browser communication via CDP binding.
(function () {
  "use strict";

  if (window.__scout) return;

  const handlers = {};
  let mutationObserver = null;

  const scout = {
    // Send an event to Go via CDP binding.
    send: function (type, data) {
      if (typeof window.__scoutSend === "function") {
        window.__scoutSend(
          JSON.stringify({
            type: type,
            data: data !== undefined ? data : null,
            ts: Date.now(),
          })
        );
      }
    },

    // Register a handler for commands from Go.
    on: function (type, handler) {
      if (!handlers[type]) handlers[type] = [];
      handlers[type].push(handler);
    },

    // Unregister all handlers for a given type.
    off: function (type) {
      delete handlers[type];
    },

    // Start observing DOM mutations on elements matching selector.
    observeMutations: function (selector) {
      if (mutationObserver) {
        mutationObserver.disconnect();
      }

      var target = selector
        ? document.querySelector(selector)
        : document.body;
      if (!target) return;

      mutationObserver = new MutationObserver(function (mutations) {
        var summary = mutations.map(function (m) {
          return {
            type: m.type,
            target: m.target.nodeName,
            addedNodes: m.addedNodes.length,
            removedNodes: m.removedNodes.length,
            attributeName: m.attributeName || null,
            oldValue: m.oldValue || null,
          };
        });
        scout.send("mutation", summary);
      });

      mutationObserver.observe(target, {
        childList: true,
        attributes: true,
        characterData: true,
        subtree: true,
        attributeOldValue: true,
      });
    },

    // Stop observing DOM mutations.
    stopMutations: function () {
      if (mutationObserver) {
        mutationObserver.disconnect();
        mutationObserver = null;
      }
    },

    // Check if bridge is available.
    available: function () {
      return typeof window.__scoutSend === "function";
    },
  };

  // Listen for commands dispatched from Go via page.Eval().
  window.addEventListener("__scoutCommand", function (e) {
    var detail = e.detail;
    if (!detail || !detail.type) return;

    var fns = handlers[detail.type];
    if (fns) {
      for (var i = 0; i < fns.length; i++) {
        try {
          var result = fns[i](detail.data);
          // If this is a query (has an id), send back the response.
          if (detail.id) {
            scout.send("__query_response", {
              id: detail.id,
              result: result !== undefined ? result : null,
              error: null,
            });
          }
        } catch (err) {
          if (detail.id) {
            scout.send("__query_response", {
              id: detail.id,
              result: null,
              error: err.message || String(err),
            });
          }
        }
      }
    }
  });

  // Built-in handler: dom_json — convert DOM to JSON tree.
  scout.on("dom_json", function (params) {
    params = params || {};
    var selector = params.selector || null;
    var maxDepth = params.depth || 50;
    var skipTags = { SCRIPT: 1, STYLE: 1, NOSCRIPT: 1 };

    function walk(node, depth) {
      if (depth > maxDepth) return null;
      if (node.nodeType === Node.TEXT_NODE) {
        var text = node.textContent.trim();
        if (!text) return null;
        return { tag: "#text", text: text };
      }
      if (node.nodeType !== Node.ELEMENT_NODE) return null;
      var tag = node.tagName;
      if (skipTags[tag]) return null;

      var obj = { tag: tag.toLowerCase() };
      if (node.attributes && node.attributes.length > 0) {
        obj.attributes = {};
        for (var i = 0; i < node.attributes.length; i++) {
          obj.attributes[node.attributes[i].name] = node.attributes[i].value;
        }
      }
      var children = [];
      for (var c = node.firstChild; c; c = c.nextSibling) {
        var child = walk(c, depth + 1);
        if (child) children.push(child);
      }
      if (children.length > 0) obj.children = children;
      return obj;
    }

    var root = selector ? document.querySelector(selector) : document.documentElement;
    if (!root) return { error: "selector not found: " + selector };
    return walk(root, 0);
  });

  // Built-in handler: dom_markdown — convert DOM to markdown in-browser.
  scout.on("dom_markdown", function (params) {
    params = params || {};
    var selector = params.selector || null;
    var mainOnly = params.mainOnly || false;

    function findMain(doc) {
      var candidates = [
        doc.querySelector("main"),
        doc.querySelector("article"),
        doc.querySelector('[role="main"]'),
        doc.querySelector("#content"),
        doc.querySelector(".content"),
      ];
      for (var i = 0; i < candidates.length; i++) {
        if (candidates[i]) return candidates[i];
      }
      return doc.body || doc.documentElement;
    }

    var root;
    if (selector) {
      root = document.querySelector(selector);
      if (!root) return "<!-- selector not found: " + selector + " -->";
    } else if (mainOnly) {
      root = findMain(document);
    } else {
      root = document.body || document.documentElement;
    }

    var skipTags = { SCRIPT: 1, STYLE: 1, NOSCRIPT: 1, SVG: 1 };
    var blockTags = {
      P: 1, DIV: 1, SECTION: 1, ARTICLE: 1, ASIDE: 1, HEADER: 1, FOOTER: 1,
      NAV: 1, MAIN: 1, BLOCKQUOTE: 1, FIGURE: 1, FIGCAPTION: 1, DETAILS: 1,
      SUMMARY: 1, ADDRESS: 1, HGROUP: 1, SEARCH: 1,
    };

    function convert(node) {
      if (node.nodeType === Node.TEXT_NODE) {
        return node.textContent;
      }
      if (node.nodeType !== Node.ELEMENT_NODE) return "";
      var tag = node.tagName;
      if (skipTags[tag]) return "";

      var inner = "";
      for (var c = node.firstChild; c; c = c.nextSibling) {
        inner += convert(c);
      }
      inner = inner.replace(/\n{3,}/g, "\n\n");

      switch (tag) {
        case "H1": return "\n\n# " + inner.trim() + "\n\n";
        case "H2": return "\n\n## " + inner.trim() + "\n\n";
        case "H3": return "\n\n### " + inner.trim() + "\n\n";
        case "H4": return "\n\n#### " + inner.trim() + "\n\n";
        case "H5": return "\n\n##### " + inner.trim() + "\n\n";
        case "H6": return "\n\n###### " + inner.trim() + "\n\n";
        case "P": return "\n\n" + inner.trim() + "\n\n";
        case "BR": return "\n";
        case "HR": return "\n\n---\n\n";
        case "STRONG": case "B": return "**" + inner.trim() + "**";
        case "EM": case "I": return "*" + inner.trim() + "*";
        case "CODE":
          if (node.parentElement && node.parentElement.tagName === "PRE") return inner;
          return "`" + inner.trim() + "`";
        case "PRE": return "\n\n```\n" + inner.trim() + "\n```\n\n";
        case "A":
          var href = node.getAttribute("href") || "";
          var text = inner.trim();
          if (!text) return "";
          return "[" + text + "](" + href + ")";
        case "IMG":
          var alt = node.getAttribute("alt") || "";
          var src = node.getAttribute("src") || "";
          return "![" + alt + "](" + src + ")";
        case "UL": return "\n\n" + inner + "\n";
        case "OL": return "\n\n" + inner + "\n";
        case "LI":
          var prefix = "- ";
          if (node.parentElement && node.parentElement.tagName === "OL") {
            var idx = 1;
            for (var s = node; s.previousElementSibling; s = s.previousElementSibling) idx++;
            prefix = idx + ". ";
          }
          return prefix + inner.trim() + "\n";
        case "BLOCKQUOTE": return "\n\n> " + inner.trim().replace(/\n/g, "\n> ") + "\n\n";
        case "TABLE": return "\n\n" + convertTable(node) + "\n\n";
        case "THEAD": case "TBODY": case "TFOOT": case "TR":
        case "TH": case "TD": return inner; // handled by convertTable
        default:
          if (blockTags[tag]) return "\n\n" + inner.trim() + "\n\n";
          return inner;
      }
    }

    function convertTable(table) {
      var rows = table.querySelectorAll("tr");
      if (rows.length === 0) return "";
      var lines = [];
      for (var r = 0; r < rows.length; r++) {
        var cells = rows[r].querySelectorAll("th, td");
        var cols = [];
        for (var c = 0; c < cells.length; c++) {
          cols.push(cells[c].textContent.trim().replace(/\|/g, "\\|"));
        }
        lines.push("| " + cols.join(" | ") + " |");
        if (r === 0 && rows[r].querySelector("th")) {
          lines.push("| " + cols.map(function () { return "---"; }).join(" | ") + " |");
        }
      }
      return lines.join("\n");
    }

    var result = convert(root);
    // Clean up excessive newlines.
    result = result.replace(/\n{3,}/g, "\n\n").trim();
    return result;
  });

  // Built-in handler: dom.query — querySelector/querySelectorAll and return results.
  scout.on("dom.query", function (params) {
    params = params || {};
    var selector = params.selector;
    if (!selector) return { error: "selector required" };
    var all = params.all || false;

    if (all) {
      var els = document.querySelectorAll(selector);
      var results = [];
      for (var i = 0; i < els.length; i++) {
        results.push({
          tag: els[i].tagName.toLowerCase(),
          text: els[i].textContent.trim().substring(0, 200),
          html: els[i].outerHTML.substring(0, 500),
        });
      }
      return { count: results.length, elements: results };
    }

    var el = document.querySelector(selector);
    if (!el) return { found: false };
    return {
      found: true,
      tag: el.tagName.toLowerCase(),
      text: el.textContent.trim().substring(0, 200),
      html: el.outerHTML.substring(0, 500),
    };
  });

  // Built-in handler: dom.click — click element by selector.
  scout.on("dom.click", function (params) {
    params = params || {};
    var el = document.querySelector(params.selector);
    if (!el) return { error: "element not found: " + params.selector };
    el.click();
    return { clicked: true };
  });

  // Built-in handler: dom.type — type text into element.
  scout.on("dom.type", function (params) {
    params = params || {};
    var el = document.querySelector(params.selector);
    if (!el) return { error: "element not found: " + params.selector };
    el.focus();
    el.value = params.text || "";
    el.dispatchEvent(new Event("input", { bubbles: true }));
    el.dispatchEvent(new Event("change", { bubbles: true }));
    return { typed: true };
  });

  // Built-in handler: dom.getAttributes — get all attributes of element.
  scout.on("dom.getAttributes", function (params) {
    params = params || {};
    var el = document.querySelector(params.selector);
    if (!el) return { error: "element not found: " + params.selector };
    var attrs = {};
    for (var i = 0; i < el.attributes.length; i++) {
      attrs[el.attributes[i].name] = el.attributes[i].value;
    }
    return { tag: el.tagName.toLowerCase(), attributes: attrs };
  });

  // Built-in handler: dom.observe — start MutationObserver, forward mutations.
  scout.on("dom.observe", function (params) {
    params = params || {};
    scout.observeMutations(params.selector || "");
    return { observing: true };
  });

  // Built-in handler: dom.shadowQuery — query inside shadow DOM roots.
  scout.on("dom.shadowQuery", function (params) {
    params = params || {};
    var hostSelector = params.host;
    var innerSelector = params.selector;
    if (!hostSelector || !innerSelector) return { error: "host and selector required" };

    var host = document.querySelector(hostSelector);
    if (!host) return { error: "host not found: " + hostSelector };
    if (!host.shadowRoot) return { error: "no shadow root on: " + hostSelector };

    var el = host.shadowRoot.querySelector(innerSelector);
    if (!el) return { found: false };
    return {
      found: true,
      tag: el.tagName.toLowerCase(),
      text: el.textContent.trim().substring(0, 200),
      html: el.outerHTML.substring(0, 500),
    };
  });

  // User interaction tracking (active when observation is enabled).
  var _interactionTracking = false;

  scout.on("__enable_interaction_tracking", function () {
    if (_interactionTracking) return { already: true };
    _interactionTracking = true;

    document.addEventListener("click", function (e) {
      if (!_interactionTracking) return;
      scout.send("user.click", {
        selector: _cssPath(e.target),
        x: e.clientX,
        y: e.clientY,
        tag: e.target.tagName.toLowerCase(),
        text: (e.target.textContent || "").trim().substring(0, 100),
      });
    }, true);

    document.addEventListener("input", function (e) {
      if (!_interactionTracking) return;
      scout.send("user.input", {
        selector: _cssPath(e.target),
        tag: e.target.tagName.toLowerCase(),
        value: (e.target.value || "").substring(0, 200),
      });
    }, true);

    return { tracking: true };
  });

  // Generate a simple CSS path for an element.
  function _cssPath(el) {
    if (!el || el === document.body) return "body";
    var parts = [];
    while (el && el !== document.body && el.nodeType === Node.ELEMENT_NODE) {
      var tag = el.tagName.toLowerCase();
      if (el.id) {
        parts.unshift(tag + "#" + el.id);
        break;
      }
      var idx = 1;
      for (var s = el.previousElementSibling; s; s = s.previousElementSibling) {
        if (s.tagName === el.tagName) idx++;
      }
      var siblings = el.parentElement ? el.parentElement.querySelectorAll(":scope > " + tag) : [];
      if (siblings.length > 1) {
        parts.unshift(tag + ":nth-of-type(" + idx + ")");
      } else {
        parts.unshift(tag);
      }
      el = el.parentElement;
    }
    return parts.join(" > ");
  }

  // Handle messages forwarded from background (WebSocket bridge requests).
  if (typeof chrome !== "undefined" && chrome.runtime && chrome.runtime.onMessage) {
    chrome.runtime.onMessage.addListener(function (message, sender, sendResponse) {
      if (message.target !== "content" || !message.bridgeRequest) return false;

      var method = message.method;
      var fns = handlers[method];
      if (fns && fns.length > 0) {
        try {
          var params = message.params;
          if (typeof params === "string") {
            try { params = JSON.parse(params); } catch (e) { /* use as-is */ }
          }
          var result = fns[0](params);
          sendResponse(result !== undefined ? result : null);
        } catch (err) {
          sendResponse({ error: err.message || String(err) });
        }
      } else {
        sendResponse({ error: "no handler for method: " + method });
      }
      return false;
    });
  }

  window.__scout = scout;

  // Notify Go that the bridge content script is loaded.
  scout.send("__bridge_ready", { url: window.location.href });
})();
