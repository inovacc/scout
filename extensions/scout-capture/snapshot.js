// snapshot.js — runs IN the page (top frame). Returns this origin's web storage
// as { origin, store: { local: {...}, session: {...} } }. No secrets logged.
function scoutCaptureSnapshot() {
  function dump(s) {
    const out = {};
    try {
      for (let i = 0; i < s.length; i++) {
        const k = s.key(i);
        out[k] = s.getItem(k);
      }
    } catch (e) {
      // storage may be blocked (e.g. about: pages); return what we have.
    }
    return out;
  }
  return {
    origin: location.origin,
    store: { local: dump(window.localStorage), session: dump(window.sessionStorage) },
  };
}

// Final statement: when this file is injected via executeScript({files:[...]}),
// executeScript returns the value of the last evaluated expression, so calling
// the function here makes a single injection return the snapshot directly.
scoutCaptureSnapshot();
