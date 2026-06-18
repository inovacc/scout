/**
 * Scout Evade — stealth evasions content script (MAIN world, document_start)
 *
 * Ported faithfully from internal/engine/stealth/stealth_extra.go (17 evasions)
 * plus the navigator.webdriver patch from the main stealth layer.
 *
 * Each evasion is wrapped in its own try/catch so a single failure cannot
 * abort the rest. The outer guard prevents double-injection.
 */
(function () {
  'use strict';

  // Idempotency guard — if the extension is somehow injected twice, bail out.
  if (window.__scoutEvadeInjected) return;
  Object.defineProperty(window, '__scoutEvadeInjected', {
    value: true,
    writable: false,
    configurable: false,
    enumerable: false,
  });

  // ─── toString integrity registry (must run first) ─────────────────────────
  // Evasion 17: Protect Function.prototype.toString so overridden functions
  // look like native code to bot-detection probes.
  var nativeToString;
  var overrides;
  try {
    nativeToString = Function.prototype.toString;
    overrides = new WeakMap();

    var patchedToString = function toString() {
      if (overrides.has(this)) return overrides.get(this);
      return nativeToString.call(this);
    };
    overrides.set(patchedToString, 'function toString() { [native code] }');
    Function.prototype.toString = patchedToString;
  } catch (e) {
    // If we can't patch toString, create a no-op markNative so later
    // evasions don't throw.
    overrides = { has: function () { return false; }, set: function () {} };
  }

  function markNative(fn, name) {
    try { overrides.set(fn, 'function ' + name + '() { [native code] }'); } catch (e) {}
  }

  // ─── Evasion 0: navigator.webdriver ───────────────────────────────────────
  // Sourced from Scout's main stealth layer (go-rod/stealth internalized).
  try {
    Object.defineProperty(navigator, 'webdriver', {
      get: function () { return false; },
      configurable: true,
      enumerable: true,
    });
  } catch (e) {}

  // ─── Evasion 1: Canvas fingerprint noise ──────────────────────────────────
  try {
    var originalToDataURL = HTMLCanvasElement.prototype.toDataURL;
    HTMLCanvasElement.prototype.toDataURL = function (type, quality) {
      var ctx = this.getContext('2d');
      if (ctx) {
        var imageData = ctx.getImageData(0, 0, this.width, this.height);
        var data = imageData.data;
        for (var i = 0; i < data.length; i += 4) {
          data[i]     = Math.max(0, Math.min(255, data[i]     + ((Math.random() * 4 - 2) | 0)));
          data[i + 1] = Math.max(0, Math.min(255, data[i + 1] + ((Math.random() * 4 - 2) | 0)));
          data[i + 2] = Math.max(0, Math.min(255, data[i + 2] + ((Math.random() * 4 - 2) | 0)));
        }
        ctx.putImageData(imageData, 0, 0);
      }
      return originalToDataURL.call(this, type, quality);
    };
    markNative(HTMLCanvasElement.prototype.toDataURL, 'toDataURL');
  } catch (e) {}

  try {
    var originalGetImageData = CanvasRenderingContext2D.prototype.getImageData;
    CanvasRenderingContext2D.prototype.getImageData = function () {
      var imageData = originalGetImageData.apply(this, arguments);
      var data = imageData.data;
      for (var i = 0; i < data.length; i += 4) {
        data[i]     = Math.max(0, Math.min(255, data[i]     + ((Math.random() * 2 - 1) | 0)));
        data[i + 1] = Math.max(0, Math.min(255, data[i + 1] + ((Math.random() * 2 - 1) | 0)));
        data[i + 2] = Math.max(0, Math.min(255, data[i + 2] + ((Math.random() * 2 - 1) | 0)));
      }
      return imageData;
    };
    markNative(CanvasRenderingContext2D.prototype.getImageData, 'getImageData');
  } catch (e) {}

  // ─── Evasion 2: AudioContext fingerprint noise ─────────────────────────────
  try {
    if (typeof AudioContext !== 'undefined') {
      var origCreateOscillator = AudioContext.prototype.createOscillator;
      AudioContext.prototype.createOscillator = function () {
        var oscillator = origCreateOscillator.call(this);
        var origConnect = oscillator.connect.bind(oscillator);
        oscillator.connect = function (destination) {
          try {
            var gainNode = oscillator.context.createGain();
            gainNode.gain.value = 1 + (Math.random() * 0.0001 - 0.00005);
            origConnect(gainNode);
            gainNode.connect(destination);
            return destination;
          } catch (e) {
            return origConnect(destination);
          }
        };
        return oscillator;
      };
    }
  } catch (e) {}

  // ─── Evasion 3: WebGL vendor/renderer spoofing ────────────────────────────
  // TODO: parameterize vendor/renderer strings if different profiles are needed.
  try {
    var getParameterProto = WebGLRenderingContext.prototype.getParameter;
    WebGLRenderingContext.prototype.getParameter = function (param) {
      var ext = this.getExtension('WEBGL_debug_renderer_info');
      if (ext) {
        if (param === ext.UNMASKED_VENDOR_WEBGL)   return 'Intel Inc.';
        if (param === ext.UNMASKED_RENDERER_WEBGL) return 'Intel Iris OpenGL Engine';
      }
      return getParameterProto.call(this, param);
    };
  } catch (e) {}

  try {
    if (typeof WebGL2RenderingContext !== 'undefined') {
      var getParameter2Proto = WebGL2RenderingContext.prototype.getParameter;
      WebGL2RenderingContext.prototype.getParameter = function (param) {
        var ext = this.getExtension('WEBGL_debug_renderer_info');
        if (ext) {
          if (param === ext.UNMASKED_VENDOR_WEBGL)   return 'Intel Inc.';
          if (param === ext.UNMASKED_RENDERER_WEBGL) return 'Intel Iris OpenGL Engine';
        }
        return getParameter2Proto.call(this, param);
      };
    }
  } catch (e) {}

  // ─── Evasion 4: navigator.connection spoofing ─────────────────────────────
  try {
    if ('connection' in navigator) {
      Object.defineProperties(navigator.connection, {
        effectiveType: { value: '4g',  writable: false, enumerable: true, configurable: true },
        downlink:      { value: 10,    writable: false, enumerable: true, configurable: true },
        rtt:           { value: 50,    writable: false, enumerable: true, configurable: true },
        saveData:      { value: false, writable: false, enumerable: true, configurable: true },
      });
    } else {
      Object.defineProperty(navigator, 'connection', {
        get: function () {
          return {
            effectiveType: '4g',
            downlink: 10,
            rtt: 50,
            saveData: false,
            onchange: null,
            addEventListener: function () {},
            removeEventListener: function () {},
            dispatchEvent: function () { return true; },
          };
        },
        enumerable: true,
        configurable: true,
      });
    }
  } catch (e) {}

  // ─── Evasion 5: Notification.permission → "default" ───────────────────────
  try {
    if (typeof Notification !== 'undefined') {
      Object.defineProperty(Notification, 'permission', {
        get: function () { return 'default'; },
        configurable: true,
      });
    }
  } catch (e) {}

  // ─── Evasion 6: WebRTC local-IP leak prevention ───────────────────────────
  try {
    if (typeof RTCPeerConnection !== 'undefined') {
      var OrigRTC = RTCPeerConnection;
      var localIPPattern = /(\b(10|172\.(1[6-9]|2\d|3[01])|192\.168)\.\d{1,3}\.\d{1,3}\b)|([0-9a-f]{1,4}(:[0-9a-f]{1,4}){7})/i;

      function PatchedRTC(config, constraints) {
        var pc = new OrigRTC(config, constraints);

        var origCreateOffer = pc.createOffer.bind(pc);
        pc.createOffer = function (options) {
          return origCreateOffer(options).then(function (offer) {
            if (offer && offer.sdp) {
              offer.sdp = offer.sdp.split('\n').filter(function (line) {
                if (line.indexOf('a=candidate') !== 0) return true;
                return !localIPPattern.test(line);
              }).join('\n');
            }
            return offer;
          });
        };

        var origOnIceCandidate = Object.getOwnPropertyDescriptor(
          RTCPeerConnection.prototype, 'onicecandidate'
        );
        if (origOnIceCandidate && origOnIceCandidate.set) {
          var userHandler = null;
          Object.defineProperty(pc, 'onicecandidate', {
            get: function () { return userHandler; },
            set: function (handler) {
              userHandler = function (event) {
                if (event.candidate && event.candidate.candidate &&
                    localIPPattern.test(event.candidate.candidate)) {
                  return;
                }
                if (typeof handler === 'function') handler(event);
              };
              origOnIceCandidate.set.call(pc, userHandler);
            },
            configurable: true,
            enumerable: true,
          });
        }

        return pc;
      }

      PatchedRTC.prototype = OrigRTC.prototype;
      PatchedRTC.generateCertificate = OrigRTC.generateCertificate;

      Object.defineProperty(window, 'RTCPeerConnection', {
        value: PatchedRTC,
        writable: true,
        configurable: true,
      });
      if (typeof webkitRTCPeerConnection !== 'undefined') {
        Object.defineProperty(window, 'webkitRTCPeerConnection', {
          value: PatchedRTC,
          writable: true,
          configurable: true,
        });
      }
    }
  } catch (e) {}

  // ─── Evasion 7: Font fingerprint normalisation ────────────────────────────
  try {
    if (typeof document !== 'undefined' && document.fonts) {
      var commonFonts = [
        'Arial', 'Arial Black', 'Comic Sans MS', 'Courier New', 'Georgia',
        'Impact', 'Lucida Console', 'Lucida Sans Unicode', 'Palatino Linotype',
        'Tahoma', 'Times New Roman', 'Trebuchet MS', 'Verdana',
        'Microsoft Sans Serif', 'Segoe UI',
      ];

      var origCheck = document.fonts.check.bind(document.fonts);
      document.fonts.check = function (font, text) {
        try {
          var fontName = font.replace(/^[\d.]+\w+\s+/, '').replace(/['"]/g, '').trim();
          for (var i = 0; i < commonFonts.length; i++) {
            if (fontName.toLowerCase() === commonFonts[i].toLowerCase()) return true;
          }
        } catch (e) {}
        return origCheck(font, text);
      };
      markNative(document.fonts.check, 'check');

      var origForEach = document.fonts.forEach.bind(document.fonts);
      document.fonts.forEach = function (callback, thisArg) {
        var seen = new Set();
        origForEach(function (entry) {
          var family = entry.family.replace(/['"]/g, '');
          if (!seen.has(family)) {
            seen.add(family);
            callback.call(thisArg, entry);
          }
        });
      };
      markNative(document.fonts.forEach, 'forEach');
    }
  } catch (e) {}

  // ─── Evasion 8: Screen resolution consistency ──────────────────────────────
  try {
    var sw = window.innerWidth  || 1920;
    var sh = window.innerHeight || 1080;
    Object.defineProperty(screen, 'width',       { get: function () { return sw; }, configurable: true });
    Object.defineProperty(screen, 'height',      { get: function () { return sh; }, configurable: true });
    Object.defineProperty(screen, 'availWidth',  { get: function () { return sw; }, configurable: true });
    Object.defineProperty(screen, 'availHeight', { get: function () { return sh; }, configurable: true });
    Object.defineProperty(screen, 'colorDepth',  { get: function () { return 24; }, configurable: true });
    Object.defineProperty(screen, 'pixelDepth',  { get: function () { return 24; }, configurable: true });
  } catch (e) {}

  // ─── Evasion 9: Battery API spoofing ──────────────────────────────────────
  try {
    if (typeof navigator !== 'undefined') {
      var batteryInfo = {
        charging: true,
        chargingTime: 0,
        dischargingTime: Infinity,
        level: 1.0,
        addEventListener: function () {},
        removeEventListener: function () {},
        dispatchEvent: function () { return true; },
        onchargingchange: null,
        onchargingtimechange: null,
        ondischargingtimechange: null,
        onlevelchange: null,
      };
      Object.defineProperty(navigator, 'getBattery', {
        value: function () { return Promise.resolve(batteryInfo); },
        writable: true,
        configurable: true,
        enumerable: true,
      });
    }
  } catch (e) {}

  // ─── Evasion 10: ChromeDriver leak variable removal ───────────────────────
  try { delete window.cdc_adoQpoasnfa76pfcZLmcfl_l8; } catch (e) {}
  try {
    Object.getOwnPropertyNames(document).forEach(function (prop) {
      if (/^\$cdc_/.test(prop)) {
        try { delete document[prop]; } catch (e) {}
      }
    });
  } catch (e) {}
  try { delete window.callPhantom; }           catch (e) {}
  try { delete window._phantom; }              catch (e) {}
  try { delete window.__nightmare; }           catch (e) {}
  try { delete window.domAutomation; }         catch (e) {}
  try { delete window.domAutomationController; } catch (e) {}

  // ─── Evasion 11: navigator hardware/memory/vendor ─────────────────────────
  try {
    if (navigator.hardwareConcurrency === 0 || !navigator.hardwareConcurrency) {
      Object.defineProperty(navigator, 'hardwareConcurrency', {
        get: function () { return 8; },
        configurable: true,
      });
    }
  } catch (e) {}
  try {
    if (!navigator.deviceMemory) {
      Object.defineProperty(navigator, 'deviceMemory', {
        get: function () { return 8; },
        configurable: true,
      });
    }
  } catch (e) {}
  try {
    if (!navigator.vendor || navigator.vendor === '') {
      Object.defineProperty(navigator, 'vendor', {
        get: function () { return 'Google Inc.'; },
        configurable: true,
      });
    }
  } catch (e) {}

  // ─── Evasion 12: document.hasFocus always true ────────────────────────────
  try {
    Document.prototype.hasFocus = function () { return true; };
    markNative(Document.prototype.hasFocus, 'hasFocus');
  } catch (e) {}

  // ─── Evasion 13: window outer dimensions ──────────────────────────────────
  try {
    if (window.outerWidth === 0) {
      Object.defineProperty(window, 'outerWidth', {
        get: function () { return window.innerWidth || 1920; },
        configurable: true,
      });
    }
    if (window.outerHeight === 0) {
      Object.defineProperty(window, 'outerHeight', {
        // 85 px accounts for Chrome's toolbar/UI chrome
        get: function () { return (window.innerHeight || 1080) + 85; },
        configurable: true,
      });
    }
  } catch (e) {}

  // ─── Evasion 14: navigator.languages / navigator.language ─────────────────
  // TODO: parameterize locale if a non-English profile is needed.
  try {
    if (!navigator.languages || navigator.languages.length === 0) {
      Object.defineProperty(navigator, 'languages', {
        get: function () { return ['en-US', 'en']; },
        configurable: true,
      });
    }
    if (!navigator.language) {
      Object.defineProperty(navigator, 'language', {
        get: function () { return 'en-US'; },
        configurable: true,
      });
    }
  } catch (e) {}

  // ─── Evasion 15: navigator.plugins / mimeTypes ────────────────────────────
  try {
    if (navigator.plugins.length === 0) {
      var fakePlugins = [
        { name: 'PDF Viewer',               description: 'Portable Document Format', filename: 'internal-pdf-viewer', length: 1 },
        { name: 'Chrome PDF Viewer',         description: 'Portable Document Format', filename: 'internal-pdf-viewer', length: 1 },
        { name: 'Chromium PDF Viewer',       description: 'Portable Document Format', filename: 'internal-pdf-viewer', length: 1 },
        { name: 'Microsoft Edge PDF Viewer', description: 'Portable Document Format', filename: 'internal-pdf-viewer', length: 1 },
        { name: 'WebKit built-in PDF',       description: 'Portable Document Format', filename: 'internal-pdf-viewer', length: 1 },
      ];
      var pluginArray = Object.create(PluginArray.prototype);
      for (var pi = 0; pi < fakePlugins.length; pi++) {
        var p = Object.create(Plugin.prototype);
        Object.defineProperties(p, {
          name:        { value: fakePlugins[pi].name,        enumerable: true },
          description: { value: fakePlugins[pi].description, enumerable: true },
          filename:    { value: fakePlugins[pi].filename,    enumerable: true },
          length:      { value: fakePlugins[pi].length,      enumerable: true },
        });
        pluginArray[pi] = p;
      }
      Object.defineProperty(pluginArray, 'length', { value: fakePlugins.length, enumerable: true });
      Object.defineProperty(navigator, 'plugins', {
        get: function () { return pluginArray; },
        configurable: true,
        enumerable: true,
      });
    }
  } catch (e) {}

  try {
    if (navigator.mimeTypes.length === 0) {
      var fakeMimeType = Object.create(MimeType.prototype);
      Object.defineProperties(fakeMimeType, {
        type:        { value: 'application/pdf',        enumerable: true },
        suffixes:    { value: 'pdf',                    enumerable: true },
        description: { value: 'Portable Document Format', enumerable: true },
      });
      var mimeArray = Object.create(MimeTypeArray.prototype);
      mimeArray[0] = fakeMimeType;
      Object.defineProperty(mimeArray, 'length', { value: 1, enumerable: true });
      Object.defineProperty(navigator, 'mimeTypes', {
        get: function () { return mimeArray; },
        configurable: true,
        enumerable: true,
      });
    }
  } catch (e) {}

  // ─── Evasion 16: Intl.DateTimeFormat timezone ─────────────────────────────
  // TODO: parameterize fakeTimezone if the user's real locale is needed.
  try {
    var resolved = Intl.DateTimeFormat().resolvedOptions();
    if (resolved.timeZone === 'UTC' || !resolved.timeZone) {
      var origDTF = Intl.DateTimeFormat;
      var fakeTimezone = 'America/New_York';
      Intl.DateTimeFormat = function (locales, options) {
        options = Object.assign({}, options);
        if (!options.timeZone) options.timeZone = fakeTimezone;
        return new origDTF(locales, options);
      };
      Intl.DateTimeFormat.prototype = origDTF.prototype;
      Intl.DateTimeFormat.supportedLocalesOf = origDTF.supportedLocalesOf;
    }
  } catch (e) {}

  // toString integrity marks for any remaining patched functions.
  try {
    if (navigator.permissions && navigator.permissions.query) {
      markNative(navigator.permissions.query, 'query');
    }
  } catch (e) {}

})();
