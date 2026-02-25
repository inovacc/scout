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

  // Built-in handler: dom.insert — insertAdjacentHTML on element.
  scout.on("dom.insert", function (params) {
    params = params || {};
    var el = document.querySelector(params.selector);
    if (!el) return { error: "element not found: " + params.selector };
    var position = params.position || "beforeend";
    try {
      el.insertAdjacentHTML(position, params.html || "");
    } catch (e) {
      return { error: "insertAdjacentHTML failed: " + e.message };
    }
    return { inserted: true };
  });

  // Built-in handler: dom.remove — remove element from DOM.
  scout.on("dom.remove", function (params) {
    params = params || {};
    var el = document.querySelector(params.selector);
    if (!el) return { error: "element not found: " + params.selector };
    el.remove();
    return { removed: true };
  });

  // Built-in handler: dom.modifyAttr — setAttribute on element.
  scout.on("dom.modifyAttr", function (params) {
    params = params || {};
    var el = document.querySelector(params.selector);
    if (!el) return { error: "element not found: " + params.selector };
    el.setAttribute(params.attribute, params.value || "");
    return { modified: true };
  });

  // Built-in handler: clipboard.read — read clipboard text.
  scout.on("clipboard.read", function (params) {
    // Clipboard API is async; return a promise-like pattern via callback.
    // Since our handler model is synchronous, we attempt sync read first.
    try {
      // This only works with user gesture / permissions policy.
      if (navigator.clipboard && navigator.clipboard.readText) {
        // We cannot do async in this sync handler model, but we try.
        var result = { text: "", pending: true };
        navigator.clipboard.readText().then(function (text) {
          // For async, we send via event.
          scout.send("clipboard.result", { text: text });
        }).catch(function (err) {
          scout.send("clipboard.result", { error: err.message });
        });
        return result;
      }
      return { error: "clipboard API not available" };
    } catch (e) {
      return { error: e.message };
    }
  });

  // Built-in handler: clipboard.write — write text to clipboard.
  scout.on("clipboard.write", function (params) {
    params = params || {};
    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(params.text || "").then(function () {
          scout.send("clipboard.result", { written: true });
        }).catch(function (err) {
          scout.send("clipboard.result", { error: err.message });
        });
        return { writing: true };
      }
      return { error: "clipboard API not available" };
    } catch (e) {
      return { error: e.message };
    }
  });

  // Console capture buffer and interceptors.
  var _consoleBuffer = [];
  var _consoleCapturing = false;

  // Built-in handler: console.capture — install console interceptors.
  scout.on("console.capture", function () {
    if (_consoleCapturing) return { already: true };
    _consoleCapturing = true;
    _consoleBuffer = [];

    var levels = ["log", "warn", "error", "info", "debug"];
    for (var i = 0; i < levels.length; i++) {
      (function (level) {
        var original = console[level];
        console[level] = function () {
          var args = [];
          for (var j = 0; j < arguments.length; j++) {
            try {
              args.push(typeof arguments[j] === "object" ? JSON.stringify(arguments[j]) : String(arguments[j]));
            } catch (e) {
              args.push("[unserializable]");
            }
          }
          _consoleBuffer.push({ level: level, text: args.join(" "), ts: Date.now() });
          // Also forward as event.
          scout.send("console.log", { level: level, text: args.join(" ") });
          // Call original.
          if (original) original.apply(console, arguments);
        };
      })(levels[i]);
    }
    return { capturing: true };
  });

  // Built-in handler: console.get — return buffered console messages.
  scout.on("console.get", function () {
    var msgs = _consoleBuffer.slice();
    return { messages: msgs, count: msgs.length };
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

  // --- Extended __scout API ---

  // Event emitter for Go→browser events.
  var _eventListeners = new Map();

  // Exposed functions callable from Go.
  var _exposedFunctions = {};

  // Promise-based send: sends a message to Go via bridge, returns Promise.
  scout.rpc = function (method, params) {
    return new Promise(function (resolve, reject) {
      var id = "__rpc_" + Date.now() + "_" + Math.random().toString(36).slice(2);
      var timeout = setTimeout(function () {
        delete _rpcCallbacks[id];
        reject(new Error("rpc timeout: " + method));
      }, 10000);
      _rpcCallbacks[id] = function (result, error) {
        clearTimeout(timeout);
        if (error) reject(new Error(error));
        else resolve(result);
      };
      scout.send("__rpc_request", { id: id, method: method, params: params || null });
    });
  };
  var _rpcCallbacks = {};

  // Override on/off for event emitter pattern.
  var _origOn = scout.on;
  scout.on = function (type, handler) {
    // If it's an internal bridge handler, use original registration.
    if (typeof handler === "function") {
      // Also register in event listeners map for Go→browser events.
      if (!_eventListeners.has(type)) _eventListeners.set(type, []);
      _eventListeners.get(type).push(handler);
      // Register in original handlers too for bridge command routing.
      _origOn.call(scout, type, handler);
    }
  };

  var _origOff = scout.off;
  scout.off = function (type, handler) {
    if (handler && _eventListeners.has(type)) {
      var listeners = _eventListeners.get(type).filter(function (fn) { return fn !== handler; });
      if (listeners.length === 0) _eventListeners.delete(type);
      else _eventListeners.set(type, listeners);
    } else if (!handler) {
      _eventListeners.delete(type);
    }
    // Also remove from original handlers.
    if (!handler) _origOff.call(scout, type);
  };

  // Shadow DOM piercing querySelector.
  scout.query = function (selector) {
    return _shadowQuery(document, selector, false);
  };

  // Shadow DOM piercing querySelectorAll.
  scout.queryAll = function (selector) {
    return _shadowQuery(document, selector, true);
  };

  function _shadowQuery(root, selector, all) {
    // Try normal query first.
    if (!all) {
      var el = root.querySelector(selector);
      if (el) return el;
    }
    var results = all ? Array.from(root.querySelectorAll(selector)) : [];

    // Walk into shadow roots.
    var walker = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT);
    while (walker.nextNode()) {
      var node = walker.currentNode;
      if (node.shadowRoot) {
        if (all) {
          results = results.concat(Array.from(node.shadowRoot.querySelectorAll(selector)));
          // Recurse into nested shadows.
          var nested = _shadowQuery(node.shadowRoot, selector, true);
          if (Array.isArray(nested)) results = results.concat(nested);
        } else {
          var found = node.shadowRoot.querySelector(selector);
          if (found) return found;
          var nested2 = _shadowQuery(node.shadowRoot, selector, false);
          if (nested2) return nested2;
        }
      }
    }
    return all ? results : null;
  }

  // Expose a JS function callable from Go via bridge.
  scout.expose = function (name, fn) {
    if (typeof fn !== "function") return;
    _exposedFunctions[name] = fn;
  };

  // List all frames with their URLs.
  scout.frames = function () {
    var result = [];
    try {
      for (var i = 0; i < window.frames.length; i++) {
        try {
          result.push({ index: i, url: window.frames[i].location.href });
        } catch (e) {
          result.push({ index: i, url: "(cross-origin)" });
        }
      }
    } catch (e) { /* ignore */ }
    return result;
  };

  // Send message to another frame via background.js relay.
  scout.sendToFrame = function (frameIndex, method, params) {
    if (typeof chrome !== "undefined" && chrome.runtime && chrome.runtime.sendMessage) {
      return new Promise(function (resolve, reject) {
        chrome.runtime.sendMessage({
          target: "background",
          action: "frame_relay",
          frameIndex: frameIndex,
          method: method,
          params: params || null,
        }, function (response) {
          if (chrome.runtime.lastError) reject(new Error(chrome.runtime.lastError.message));
          else resolve(response);
        });
      });
    }
    return Promise.reject(new Error("chrome.runtime not available"));
  };

  // Built-in handler: __scout_call_exposed — call an exposed function from Go.
  _origOn.call(scout, "__scout_call_exposed", function (params) {
    params = params || {};
    var name = params.name;
    var args = params.args || [];
    var fn = _exposedFunctions[name];
    if (!fn) return { error: "exposed function not found: " + name };
    try {
      var result = fn.apply(null, args);
      return { result: result !== undefined ? result : null };
    } catch (e) {
      return { error: e.message || String(e) };
    }
  });

  // Built-in handler: __scout_emit_event — dispatch event to on() listeners.
  _origOn.call(scout, "__scout_emit_event", function (params) {
    params = params || {};
    var eventName = params.event;
    var data = params.data;
    if (_eventListeners.has(eventName)) {
      var listeners = _eventListeners.get(eventName);
      for (var i = 0; i < listeners.length; i++) {
        try { listeners[i](data); } catch (e) { /* ignore */ }
      }
    }
    return { dispatched: true };
  });

  // Built-in handler: __scout_shadow_query — shadow DOM piercing query from Go.
  _origOn.call(scout, "__scout_shadow_query", function (params) {
    params = params || {};
    var selector = params.selector;
    if (!selector) return { error: "selector required" };
    var all = params.all !== false; // default true for Go-side

    var elements = _shadowQuery(document, selector, true);
    var results = [];
    for (var i = 0; i < elements.length; i++) {
      results.push({
        tag: elements[i].tagName.toLowerCase(),
        text: elements[i].textContent.trim().substring(0, 200),
        html: elements[i].outerHTML.substring(0, 500),
        inShadow: _isInShadow(elements[i]),
      });
    }
    return { count: results.length, elements: results };
  });

  function _isInShadow(el) {
    var node = el;
    while (node) {
      if (node instanceof ShadowRoot) return true;
      node = node.parentNode;
    }
    return false;
  }

  // Built-in handler: __scout_list_frames — list all frames.
  _origOn.call(scout, "__scout_list_frames", function () {
    return { frames: scout.frames() };
  });

  // Built-in handler: __scout_send_to_frame — relay to another frame.
  _origOn.call(scout, "__scout_send_to_frame", function (params) {
    params = params || {};
    // This is handled via background.js relay, so we forward there.
    if (typeof chrome !== "undefined" && chrome.runtime && chrome.runtime.sendMessage) {
      chrome.runtime.sendMessage({
        target: "background",
        action: "frame_relay",
        frameIndex: params.frameIndex,
        method: params.method,
        params: params.params || null,
      });
      return { relayed: true };
    }
    return { error: "chrome.runtime not available for frame relay" };
  });

  // Built-in handler: form.autofill — find form by selector, fill fields by name/id.
  _origOn.call(scout, "form.autofill", function (params) {
    params = params || {};
    var formSelector = params.selector;
    var data = params.data || {};
    if (!formSelector) return { error: "selector required" };

    var form = document.querySelector(formSelector);
    if (!form) return { error: "form not found: " + formSelector };

    var filled = 0;
    var keys = Object.keys(data);
    for (var i = 0; i < keys.length; i++) {
      var key = keys[i];
      var value = data[key];
      // Try by name, then by id within the form.
      var field = form.querySelector('[name="' + key + '"]') ||
                  form.querySelector('#' + key);
      if (!field) continue;

      if (field.tagName === "SELECT") {
        field.value = value;
      } else if (field.type === "checkbox" || field.type === "radio") {
        field.checked = (value === "true" || value === "1" || value === "on");
      } else {
        field.value = value;
      }
      field.dispatchEvent(new Event("input", { bubbles: true }));
      field.dispatchEvent(new Event("change", { bubbles: true }));
      filled++;
    }
    return { filled: filled, total: keys.length };
  });

  // Built-in handler: fetch.download — fetch URL via page fetch API, return base64 body.
  _origOn.call(scout, "fetch.download", function (params) {
    params = params || {};
    var url = params.url;
    if (!url) return { error: "url required" };

    // fetch is async; we return a marker and send result via event.
    // However, bridge command handlers are synchronous. Use XMLHttpRequest sync mode
    // as a pragmatic fallback that inherits cookies.
    try {
      var xhr = new XMLHttpRequest();
      xhr.open("GET", url, false); // synchronous
      xhr.responseType = "arraybuffer";
      // Note: sync XHR does not support arraybuffer responseType in all browsers.
      // Fall back to overrideMimeType for binary safety.
      xhr.overrideMimeType("text/plain; charset=x-user-defined");
      xhr.send(null);

      if (xhr.status < 200 || xhr.status >= 300) {
        return { error: "fetch failed: HTTP " + xhr.status };
      }

      // Convert binary string to base64.
      var raw = xhr.responseText;
      var bytes = new Uint8Array(raw.length);
      for (var i = 0; i < raw.length; i++) {
        bytes[i] = raw.charCodeAt(i) & 0xff;
      }
      // Use btoa on chunks to avoid call stack overflow.
      var binary = "";
      var chunkSize = 8192;
      for (var j = 0; j < bytes.length; j += chunkSize) {
        var slice = bytes.subarray(j, j + chunkSize);
        for (var k = 0; k < slice.length; k++) {
          binary += String.fromCharCode(slice[k]);
        }
      }
      var b64 = btoa(binary);
      return { data: b64, status: xhr.status, size: bytes.length };
    } catch (e) {
      return { error: "fetch failed: " + e.message };
    }
  });

  // Make __scout non-enumerable to avoid detection.
  Object.defineProperty(window, "__scout", {
    value: scout,
    writable: false,
    enumerable: false,
    configurable: false,
  });

  // Define read-only state property.
  Object.defineProperty(scout, "state", {
    get: function () {
      return {
        connected: typeof window.__scoutSend === "function",
        pageID: (typeof chrome !== "undefined" && chrome.runtime) ? chrome.runtime.id : "",
      };
    },
    enumerable: false,
    configurable: false,
  });

  // Notify Go that the bridge content script is loaded.
  scout.send("__bridge_ready", { url: window.location.href });
})();
