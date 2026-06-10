// popup.js — Scout Capture popup controller.
function setStatus(text, ok) {
  const el = document.getElementById("status");
  el.textContent = text;
  el.className = ok ? "ok" : "err";
}

function renderAudit() {
  chrome.storage.local.get(["audit"], (data) => {
    const ul = document.getElementById("audit");
    ul.textContent = "";
    const audit = (data && data.audit) || [];
    for (const e of audit) {
      const li = document.createElement("li");
      li.textContent = `${e.site || "?"} — ${e.at}`; // metadata only, no values
      ul.appendChild(li);
    }
  });
}

document.getElementById("save").addEventListener("click", () => {
  const nonce = document.getElementById("nonce").value.trim();
  if (!nonce) { setStatus("Enter the nonce first.", false); return; }
  chrome.storage.local.set({ pairingNonce: nonce }, () => {
    document.getElementById("nonce").value = "";
    setStatus("Nonce saved.", true);
  });
});

const btn = document.getElementById("capture");
btn.addEventListener("click", () => {
  btn.disabled = true;
  setStatus("Capturing…", true);
  chrome.runtime.sendMessage({ cmd: "capture" }, (resp) => {
    btn.disabled = false;
    if (chrome.runtime.lastError) { setStatus(chrome.runtime.lastError.message, false); return; }
    if (resp && resp.ok) { setStatus("Captured ✓ (spool id " + resp.id + ")", true); renderAudit(); }
    else { setStatus((resp && resp.error) || "capture failed", false); }
  });
});

renderAudit();
