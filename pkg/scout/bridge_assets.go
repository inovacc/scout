package scout

// Bridge extension assets embedded as Go string constants.
// This follows the same pattern as pkg/stealth/assets.go.

const bridgeManifestJSON = `{
  "manifest_version": 3,
  "name": "Scout Bridge",
  "version": "0.1.0",
  "description": "Bidirectional communication bridge between Scout and the browser runtime",
  "permissions": ["activeTab", "scripting"],
  "content_scripts": [
    {
      "matches": ["<all_urls>"],
      "js": ["content.js"],
      "run_at": "document_start",
      "all_frames": true
    }
  ],
  "background": {
    "service_worker": "background.js"
  }
}`

const bridgeContentJS = `(function () {
  "use strict";

  if (window.__scout) return;

  var handlers = {};
  var mutationObserver = null;

  var scout = {
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

    on: function (type, handler) {
      if (!handlers[type]) handlers[type] = [];
      handlers[type].push(handler);
    },

    off: function (type) {
      delete handlers[type];
    },

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

    stopMutations: function () {
      if (mutationObserver) {
        mutationObserver.disconnect();
        mutationObserver = null;
      }
    },

    available: function () {
      return typeof window.__scoutSend === "function";
    },
  };

  window.addEventListener("__scoutCommand", function (e) {
    var detail = e.detail;
    if (!detail || !detail.type) return;

    var fns = handlers[detail.type];
    if (fns) {
      for (var i = 0; i < fns.length; i++) {
        try {
          var result = fns[i](detail.data);
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

  scout.send("__bridge_ready", { url: window.location.href });
})();`

const bridgeBackgroundJS = `chrome.runtime.onMessage.addListener(function (message, sender, sendResponse) {
  if (message.target === "background") {
    if (message.broadcast) {
      chrome.tabs.query({}, function (tabs) {
        for (var i = 0; i < tabs.length; i++) {
          chrome.tabs.sendMessage(tabs[i].id, message).catch(function () {});
        }
      });
    }
    sendResponse({ ok: true });
  }
  return false;
});`
