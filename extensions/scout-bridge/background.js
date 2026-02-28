// Scout Bridge — background service worker
// Relays messages between content scripts, popup, and WebSocket server.

var _ws = null;
var _wsPort = null;
var _reconnectDelay = 1000;
var _maxReconnectDelay = 30000;
var _pendingRequests = {};
var _requestCounter = 0;

// Relay messages from content scripts.
chrome.runtime.onMessage.addListener(function (message, sender, sendResponse) {
  if (message.target === "background") {
    // Frame relay: route message to a specific frame in the same tab.
    if (message.action === "frame_relay" && sender && sender.tab) {
      var tabId = sender.tab.id;
      var frameIndex = message.frameIndex;
      // Get all frames in the tab, find the one at the given index.
      chrome.webNavigation.getAllFrames({ tabId: tabId }, function (frames) {
        if (!frames || frameIndex >= frames.length) {
          sendResponse({ error: "frame not found: index " + frameIndex });
          return;
        }
        var targetFrame = frames[frameIndex];
        chrome.tabs.sendMessage(tabId, {
          target: "content",
          bridgeRequest: true,
          method: message.method,
          params: message.params,
        }, { frameId: targetFrame.frameId }, function (response) {
          if (chrome.runtime.lastError) {
            sendResponse({ error: chrome.runtime.lastError.message });
          } else {
            sendResponse(response);
          }
        });
      });
      return true; // async response
    }

    // Forward to all content scripts if needed.
    if (message.broadcast) {
      chrome.tabs.query({}, function (tabs) {
        for (var i = 0; i < tabs.length; i++) {
          chrome.tabs.sendMessage(tabs[i].id, message).catch(function () {});
        }
      });
    }
    sendResponse({ ok: true });
  }

  // Forward content script console events to WebSocket server.
  if (message.type === "bridge_event" && message.method === "console.log" && _ws && _ws.readyState === WebSocket.OPEN) {
    var consoleEvt = {
      type: "event",
      method: "console.log",
      params: message.data || {},
    };
    if (sender && sender.tab) {
      consoleEvt.params._tabId = sender.tab.id;
      consoleEvt.params._tabUrl = sender.tab.url || "";
    }
    _ws.send(JSON.stringify(consoleEvt));
    sendResponse({ ok: true });
    return false;
  }

  // Forward content script events to WebSocket server.
  if (message.type === "bridge_event" && _ws && _ws.readyState === WebSocket.OPEN) {
    var evt = {
      type: "event",
      method: message.method || message.eventType,
      params: message.data || {},
    };
    if (sender && sender.tab) {
      evt.params._tabId = sender.tab.id;
      evt.params._tabUrl = sender.tab.url || "";
    }
    _ws.send(JSON.stringify(evt));
    sendResponse({ ok: true });
    return false;
  }

  return false;
});

// WebSocket connection to the Go bridge server.
function connectWS(port) {
  if (!port) return;
  _wsPort = port;
  _reconnectDelay = 1000;
  _doConnect();
}

function _doConnect() {
  if (!_wsPort) return;

  try {
    _ws = new WebSocket("ws://127.0.0.1:" + _wsPort + "/bridge");
  } catch (e) {
    _scheduleReconnect();
    return;
  }

  _ws.onopen = function () {
    _reconnectDelay = 1000;
    // Register with page ID.
    _ws.send(JSON.stringify({
      type: "register",
      method: "extension-" + chrome.runtime.id,
    }));
  };

  _ws.onmessage = function (event) {
    var msg;
    try {
      msg = JSON.parse(event.data);
    } catch (e) {
      return;
    }

    if (msg.type === "request") {
      _handleServerRequest(msg);
    } else if (msg.type === "response") {
      var cb = _pendingRequests[msg.id];
      if (cb) {
        delete _pendingRequests[msg.id];
        cb(msg);
      }
    }
  };

  _ws.onclose = function () {
    _ws = null;
    _scheduleReconnect();
  };

  _ws.onerror = function () {
    if (_ws) {
      _ws.close();
    }
  };
}

function _scheduleReconnect() {
  setTimeout(function () {
    _reconnectDelay = Math.min(_reconnectDelay * 2, _maxReconnectDelay);
    _doConnect();
  }, _reconnectDelay);
}

// Handle requests from the Go server routed to content scripts.
function _handleServerRequest(msg) {
  var method = msg.method;

  // Tab management commands handled in background.
  if (method === "tab.list") {
    chrome.tabs.query({}, function (tabs) {
      var result = tabs.map(function (t) {
        return { id: t.id, url: t.url, title: t.title, active: t.active };
      });
      _sendResponse(msg.id, { tabs: result });
    });
    return;
  }

  if (method === "tab.close") {
    var params = msg.params ? JSON.parse(msg.params) : {};
    var tabId = params.tabId;
    if (tabId) {
      chrome.tabs.remove(tabId, function () {
        _sendResponse(msg.id, { closed: true });
      });
    } else {
      _sendResponse(msg.id, null, "tabId required");
    }
    return;
  }

  if (method === "hijack.start") {
    var params = msg.params ? (typeof msg.params === "string" ? JSON.parse(msg.params) : msg.params) : {};
    var tabId = params.tabId || params._tabId;
    if (tabId) {
      _startHijack(tabId);
      _sendResponse(msg.id, { started: true });
    } else {
      chrome.tabs.query({ active: true, currentWindow: true }, function (tabs) {
        if (tabs.length > 0) {
          _startHijack(tabs[0].id);
          _sendResponse(msg.id, { started: true, tabId: tabs[0].id });
        } else {
          _sendResponse(msg.id, null, "no active tab");
        }
      });
    }
    return;
  }

  if (method === "hijack.stop") {
    var params = msg.params ? (typeof msg.params === "string" ? JSON.parse(msg.params) : msg.params) : {};
    var tabId = params.tabId || params._tabId;
    if (tabId) {
      _stopHijack(tabId);
    } else {
      // Stop all.
      for (var tid in _hijackTargets) {
        _stopHijack(parseInt(tid));
      }
    }
    _sendResponse(msg.id, { stopped: true });
    return;
  }

  if (method === "clipboard.read" || method === "clipboard.write") {
    // Clipboard operations not available in service worker context.
    _sendResponse(msg.id, null, "clipboard not available in service worker");
    return;
  }

  // Route DOM commands to the appropriate content script.
  var params = {};
  try {
    if (msg.params) params = JSON.parse(msg.params);
  } catch (e) {
    // params might already be an object if parsed by websocket lib.
    params = msg.params || {};
  }

  var tabId = params._tabId;
  if (tabId) {
    _forwardToTab(tabId, msg);
  } else {
    // Send to active tab.
    chrome.tabs.query({ active: true, currentWindow: true }, function (tabs) {
      if (tabs.length > 0) {
        _forwardToTab(tabs[0].id, msg);
      } else {
        _sendResponse(msg.id, null, "no active tab");
      }
    });
  }
}

function _forwardToTab(tabId, msg) {
  var sendOpts = {};
  // Support frame-targeted messages via _frameId param.
  var params = {};
  try {
    if (msg.params) params = typeof msg.params === "string" ? JSON.parse(msg.params) : msg.params;
  } catch (e) { params = {}; }
  if (params._frameId !== undefined) {
    sendOpts.frameId = params._frameId;
  }

  chrome.tabs.sendMessage(tabId, {
    target: "content",
    bridgeRequest: true,
    id: msg.id,
    method: msg.method,
    params: msg.params,
  }, sendOpts, function (response) {
    if (chrome.runtime.lastError) {
      _sendResponse(msg.id, null, chrome.runtime.lastError.message);
      return;
    }
    _sendResponse(msg.id, response);
  });
}

function _sendResponse(id, result, error) {
  if (!_ws || _ws.readyState !== WebSocket.OPEN) return;
  var resp = {
    id: id,
    type: "response",
  };
  if (error) {
    resp.error = error;
  } else {
    resp.result = result;
  }
  _ws.send(JSON.stringify(resp));
}

// ════════════════════════ Hijack (debugger-based network capture) ════════════════════════

var _hijackTargets = {}; // tabId -> true

function _startHijack(tabId) {
  if (_hijackTargets[tabId]) return;
  _hijackTargets[tabId] = true;

  chrome.debugger.attach({ tabId: tabId }, "1.3", function () {
    if (chrome.runtime.lastError) {
      delete _hijackTargets[tabId];
      return;
    }
    chrome.debugger.sendCommand({ tabId: tabId }, "Network.enable", {});
  });
}

function _stopHijack(tabId) {
  if (!_hijackTargets[tabId]) return;
  delete _hijackTargets[tabId];
  chrome.debugger.detach({ tabId: tabId }, function () {});
}

chrome.debugger.onEvent.addListener(function (source, method, params) {
  if (!source.tabId || !_hijackTargets[source.tabId]) return;
  if (!_ws || _ws.readyState !== WebSocket.OPEN) return;

  var eventType = null;
  if (method === "Network.requestWillBeSent") eventType = "network.request";
  else if (method === "Network.responseReceived") eventType = "network.response";
  else if (method === "Network.webSocketCreated") eventType = "network.ws.opened";
  else if (method === "Network.webSocketFrameSent") eventType = "network.ws.sent";
  else if (method === "Network.webSocketFrameReceived") eventType = "network.ws.received";
  else if (method === "Network.webSocketClosed") eventType = "network.ws.closed";

  if (eventType) {
    _ws.send(JSON.stringify({
      type: "event",
      method: eventType,
      params: Object.assign({}, params, { _tabId: source.tabId }),
    }));
  }
});

// Listen for bridge port configuration via storage.
chrome.storage.local.get(["scoutBridgePort"], function (data) {
  if (data.scoutBridgePort) {
    connectWS(data.scoutBridgePort);
  }
});

chrome.storage.onChanged.addListener(function (changes) {
  if (changes.scoutBridgePort && changes.scoutBridgePort.newValue) {
    connectWS(changes.scoutBridgePort.newValue);
  }
});
