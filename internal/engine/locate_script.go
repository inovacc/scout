package engine

// locateScript is the browser-side resolver injected to back the Locator API.
// It defines window.__scoutLocate(spec) which returns the matching DOM elements
// for a Playwright-style locator spec:
//
//	{ kind: "role"|"text"|"label"|"placeholder"|"alttext"|"title"|"testid"|"css",
//	  value: string, name?: string, exact?: bool, testIdAttr?: string }
//
// Matching is substring + case-insensitive + whitespace-normalized unless
// exact=true. Role resolution uses an implicit-role→selector map plus an
// accessible-name match (aria-label / associated <label> / text / alt / title /
// placeholder). It is intentionally pragmatic (covers the common roles), not a
// full ARIA accessible-name computation.
const locateScript = `
window.__scoutLocate = function (spec) {
  spec = spec || {};
  var exact = !!spec.exact;
  var norm = function (s) { return (s == null ? "" : String(s)).replace(/\s+/g, " ").trim(); };
  var nlc = function (s) { return norm(s).toLowerCase(); };
  var match = function (hay, needle) {
    if (needle == null || needle === "") return true;
    var h = norm(hay), n = norm(needle);
    return exact ? (h === n) : (nlc(h).indexOf(nlc(n)) !== -1);
  };
  var visible = function (el) {
    if (!el || !el.getClientRects) return false;
    var st = el.ownerDocument.defaultView.getComputedStyle(el);
    if (st.visibility === "hidden" || st.display === "none") return false;
    return el.getClientRects().length > 0;
  };
  var roleSelectors = {
    button: "button,[role=button],input[type=button],input[type=submit],input[type=reset],summary",
    link: "a[href],[role=link]",
    textbox: "input:not([type]),input[type=text],input[type=search],input[type=email],input[type=url],input[type=tel],input[type=password],input[type=number],textarea,[role=textbox],[contenteditable=true],[contenteditable='']",
    checkbox: "input[type=checkbox],[role=checkbox]",
    radio: "input[type=radio],[role=radio]",
    combobox: "select,[role=combobox]",
    option: "option,[role=option]",
    heading: "h1,h2,h3,h4,h5,h6,[role=heading]",
    img: "img[alt]:not([alt='']),[role=img]",
    list: "ul,ol,[role=list]",
    listitem: "li,[role=listitem]",
    table: "table,[role=table]",
    row: "tr,[role=row]",
    cell: "td,th,[role=cell],[role=gridcell]",
    dialog: "[role=dialog],dialog",
    navigation: "nav,[role=navigation]",
    banner: "header,[role=banner]",
    main: "main,[role=main]",
    region: "section,[role=region]",
    tab: "[role=tab]",
    menuitem: "[role=menuitem]",
    radiogroup: "[role=radiogroup]",
    switch: "[role=switch]",
    alert: "[role=alert]",
    paragraph: "p,[role=paragraph]"
  };
  var accName = function (el) {
    var al = el.getAttribute && el.getAttribute("aria-label");
    if (al) return al;
    var lb = el.getAttribute && el.getAttribute("aria-labelledby");
    if (lb) {
      var ref = el.ownerDocument.getElementById(lb);
      if (ref) return ref.textContent;
    }
    var tag = (el.tagName || "").toLowerCase();
    if (tag === "img") return el.getAttribute("alt") || "";
    if (tag === "input") {
      var t = (el.getAttribute("type") || "").toLowerCase();
      if (t === "button" || t === "submit" || t === "reset") return el.value || "";
      // associated label
      if (el.labels && el.labels.length) return el.labels[0].textContent;
      return el.getAttribute("placeholder") || el.getAttribute("title") || "";
    }
    return el.textContent || el.getAttribute("title") || "";
  };
  var all = function (sel, root) {
    try { return Array.prototype.slice.call((root || document).querySelectorAll(sel)); }
    catch (e) { return []; }
  };
  var out = [];
  switch (spec.kind) {
    case "css":
      out = all(spec.value);
      break;
    case "testid":
      var attr = spec.testIdAttr || "data-testid";
      out = all("[" + attr + "]").filter(function (el) { return match(el.getAttribute(attr), spec.value); });
      break;
    case "placeholder":
      out = all("[placeholder]").filter(function (el) { return match(el.getAttribute("placeholder"), spec.value); });
      break;
    case "alttext":
      out = all("[alt]").filter(function (el) { return match(el.getAttribute("alt"), spec.value); });
      break;
    case "title":
      out = all("[title]").filter(function (el) { return match(el.getAttribute("title"), spec.value); });
      break;
    case "label":
      all("label").forEach(function (lab) {
        if (!match(lab.textContent, spec.value)) return;
        var ctl = null;
        var forId = lab.getAttribute("for");
        if (forId) ctl = lab.ownerDocument.getElementById(forId);
        if (!ctl) ctl = lab.querySelector("input,select,textarea,[contenteditable]");
        if (ctl && out.indexOf(ctl) === -1) out.push(ctl);
      });
      break;
    case "text":
      out = all("*").filter(function (el) {
        if (!visible(el)) return false;
        // leaf-ish: only elements whose own text (excluding child element text noise) matches
        return match(el.textContent, spec.value);
      });
      // prefer the deepest/smallest matching node
      out = out.filter(function (el) {
        return !out.some(function (o) { return o !== el && el.contains(o); });
      });
      break;
    case "role":
      var sel = roleSelectors[spec.value] || ("[role=" + spec.value + "]");
      out = all(sel).filter(function (el) {
        if (spec.name == null || spec.name === "") return true;
        return match(accName(el), spec.name);
      });
      break;
    default:
      out = [];
  }
  // de-dup, keep DOM order
  var seen = [];
  return out.filter(function (el) { if (seen.indexOf(el) !== -1) return false; seen.push(el); return true; });
};
`
