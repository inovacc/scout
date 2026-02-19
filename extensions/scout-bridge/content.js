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

  window.__scout = scout;

  // Notify Go that the bridge content script is loaded.
  scout.send("__bridge_ready", { url: window.location.href });
})();
