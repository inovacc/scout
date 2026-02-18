// Scout Bridge — background service worker
// Relays messages between content scripts and popup.
chrome.runtime.onMessage.addListener(function (message, sender, sendResponse) {
  if (message.target === "background") {
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
  return false;
});
