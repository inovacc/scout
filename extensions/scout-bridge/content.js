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

  window.__scout = scout;

  // Notify Go that the bridge content script is loaded.
  scout.send("__bridge_ready", { url: window.location.href });
})();
