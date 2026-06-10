// background.js — Scout Capture service worker.
// Wire contract: messages must contain ONLY fields on capture.Msg, and cookies
// ONLY {Name,Value,Domain,Path,Secure,HTTPOnly} (host uses DisallowUnknownFields).
const NATIVE_HOST = "com.inovacc.scout.capture";

function connectHost() {
  const port = chrome.runtime.connectNative(NATIVE_HOST);
  return port;
}

// One request/response round-trip helper over a fresh port.
function captureSession(tab) {
  return new Promise((resolve) => {
    chrome.storage.local.get(["pairingNonce"], (prefs) => {
      const nonce = (prefs && prefs.pairingNonce) || "";
      if (!nonce) {
        resolve({ ok: false, error: "Enter the pairing nonce first (Scout: run `scout vault capture-key init`)." });
        return;
      }
      let port;
      try {
        port = connectHost();
      } catch (e) {
        resolve({ ok: false, error: "Native host not reachable. Run `scout capture-host install <id>`." });
        return;
      }
      let helloAcked = false;
      const fail = (msg) => { try { port.disconnect(); } catch (e) {} resolve({ ok: false, error: msg }); };

      port.onDisconnect.addListener(() => {
        const le = chrome.runtime.lastError;
        if (!helloAcked) fail(le ? le.message : "host disconnected before pairing");
      });

      port.onMessage.addListener((msg) => {
        if (!msg || msg.v !== 1) { fail("bad host reply"); return; }
        if (msg.type === "error") { fail(msg.message || msg.code || "host error"); return; }
        if (msg.type === "hello_ack") {
          helloAcked = true;
          buildAndSend(tab, port, resolve, fail);
          return;
        }
        if (msg.type === "ack") {
          recordAudit(tab, msg.id);
          try { port.disconnect(); } catch (e) {}
          resolve({ ok: true, id: msg.id });
        }
      });

      // Step 1 of the handshake: hello.
      port.postMessage({ v: 1, type: "hello", ext_id: chrome.runtime.id, nonce: nonce });
    });
  });
}

function buildAndSend(tab, port, resolve, fail) {
  const url = tab.url || "";
  let site = "";
  try { site = new URL(url).hostname; } catch (e) { fail("active tab has no capturable URL"); return; }

  // Cookies for this tab's URL, down-mapped to the six WireCookie keys ONLY.
  chrome.cookies.getAll({ url }, (cookies) => {
    const wireCookies = (cookies || []).map((c) => ({
      Name: c.name,
      Value: c.value,
      Domain: c.domain,
      Path: c.path,
      Secure: !!c.secure,
      HTTPOnly: !!c.httpOnly,
    }));

    // Web storage via an injected snapshot (top frame of the active tab only).
    chrome.scripting.executeScript({ target: { tabId: tab.id }, files: ["snapshot.js"] }, () => {
      chrome.scripting.executeScript({ target: { tabId: tab.id }, func: scoutCaptureSnapshotCaller }, (res) => {
        const snap = (res && res[0] && res[0].result) || { origin: url, store: { local: {}, session: {} } };
        const storage = {};
        storage[snap.origin] = { local: snap.store.local || {}, session: snap.store.session || {} };

        const payload = {
          v: 1,
          type: "capture_session",
          site: site,
          cookies: wireCookies,
          storage: storage,
          at: new Date().toISOString(),
        };
        // Size guard mirrors the host's 1 MiB frame cap (no chunking in v1).
        if (JSON.stringify(payload).length > 1000000) {
          fail("session too large to capture in one message (>1 MiB)");
          return;
        }
        port.postMessage(payload);
      });
    });
  });
}

// scoutCaptureSnapshotCaller is injected to invoke the function defined in
// snapshot.js (already injected via files:[...]) and return its result.
function scoutCaptureSnapshotCaller() {
  return scoutCaptureSnapshot();
}

function recordAudit(tab, id) {
  chrome.storage.local.get(["audit"], (data) => {
    const audit = (data && data.audit) || [];
    let site = "";
    try { site = new URL(tab.url).hostname; } catch (e) {}
    audit.unshift({ site: site, id: id, at: new Date().toISOString() }); // metadata only, never values
    chrome.storage.local.set({ audit: audit.slice(0, 100) });
  });
}

// Popup → background command channel.
chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message && message.cmd === "capture") {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      if (!tabs || !tabs[0]) { sendResponse({ ok: false, error: "no active tab" }); return; }
      captureSession(tabs[0]).then(sendResponse);
    });
    return true; // async response
  }
  return false;
});
