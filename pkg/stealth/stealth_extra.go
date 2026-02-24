package stealth

// ExtraJS provides additional anti-detection evasions beyond extract-stealth-evasions.
// Applied after the main JS injection.
const ExtraJS = `
(function() {
  // 1. Canvas fingerprint noise — add subtle noise to toDataURL and getImageData
  const originalToDataURL = HTMLCanvasElement.prototype.toDataURL;
  HTMLCanvasElement.prototype.toDataURL = function(type, quality) {
    const ctx = this.getContext('2d');
    if (ctx) {
      const imageData = ctx.getImageData(0, 0, this.width, this.height);
      const data = imageData.data;
      for (let i = 0; i < data.length; i += 4) {
        // Add noise to RGB channels only (not alpha), within +-2
        data[i]     = Math.max(0, Math.min(255, data[i]     + (Math.random() * 4 - 2) | 0));
        data[i + 1] = Math.max(0, Math.min(255, data[i + 1] + (Math.random() * 4 - 2) | 0));
        data[i + 2] = Math.max(0, Math.min(255, data[i + 2] + (Math.random() * 4 - 2) | 0));
      }
      ctx.putImageData(imageData, 0, 0);
    }
    return originalToDataURL.call(this, type, quality);
  };

  const originalGetImageData = CanvasRenderingContext2D.prototype.getImageData;
  CanvasRenderingContext2D.prototype.getImageData = function() {
    const imageData = originalGetImageData.apply(this, arguments);
    const data = imageData.data;
    for (let i = 0; i < data.length; i += 4) {
      data[i]     = Math.max(0, Math.min(255, data[i]     + (Math.random() * 2 - 1) | 0));
      data[i + 1] = Math.max(0, Math.min(255, data[i + 1] + (Math.random() * 2 - 1) | 0));
      data[i + 2] = Math.max(0, Math.min(255, data[i + 2] + (Math.random() * 2 - 1) | 0));
    }
    return imageData;
  };

  // 2. AudioContext fingerprint noise — wrap createOscillator to add micro-noise
  if (typeof AudioContext !== 'undefined') {
    const origCreateOscillator = AudioContext.prototype.createOscillator;
    AudioContext.prototype.createOscillator = function() {
      const oscillator = origCreateOscillator.call(this);
      const origConnect = oscillator.connect.bind(oscillator);
      oscillator.connect = function(destination) {
        try {
          const gainNode = oscillator.context.createGain();
          gainNode.gain.value = 1 + (Math.random() * 0.0001 - 0.00005);
          origConnect(gainNode);
          gainNode.connect(destination);
          return destination;
        } catch(e) {
          return origConnect(destination);
        }
      };
      return oscillator;
    };
  }

  // 3. WebGL vendor/renderer spoofing
  const getParameterProto = WebGLRenderingContext.prototype.getParameter;
  WebGLRenderingContext.prototype.getParameter = function(param) {
    const ext = this.getExtension('WEBGL_debug_renderer_info');
    if (ext) {
      if (param === ext.UNMASKED_VENDOR_WEBGL) return 'Intel Inc.';
      if (param === ext.UNMASKED_RENDERER_WEBGL) return 'Intel Iris OpenGL Engine';
    }
    return getParameterProto.call(this, param);
  };
  if (typeof WebGL2RenderingContext !== 'undefined') {
    const getParameter2Proto = WebGL2RenderingContext.prototype.getParameter;
    WebGL2RenderingContext.prototype.getParameter = function(param) {
      const ext = this.getExtension('WEBGL_debug_renderer_info');
      if (ext) {
        if (param === ext.UNMASKED_VENDOR_WEBGL) return 'Intel Inc.';
        if (param === ext.UNMASKED_RENDERER_WEBGL) return 'Intel Iris OpenGL Engine';
      }
      return getParameter2Proto.call(this, param);
    };
  }

  // 4. navigator.connection — spoof NetworkInformation
  if ('connection' in navigator) {
    const connProps = {
      effectiveType: { value: '4g', writable: false, enumerable: true, configurable: true },
      downlink:      { value: 10,   writable: false, enumerable: true, configurable: true },
      rtt:           { value: 50,   writable: false, enumerable: true, configurable: true },
      saveData:      { value: false, writable: false, enumerable: true, configurable: true },
    };
    try {
      Object.defineProperties(navigator.connection, connProps);
    } catch(e) {}
  } else {
    try {
      Object.defineProperty(navigator, 'connection', {
        get: function() {
          return {
            effectiveType: '4g',
            downlink: 10,
            rtt: 50,
            saveData: false,
            onchange: null,
            addEventListener: function() {},
            removeEventListener: function() {},
            dispatchEvent: function() { return true; },
          };
        },
        enumerable: true,
        configurable: true,
      });
    } catch(e) {}
  }

  // 5. Notification permission — override to return "default"
  if (typeof Notification !== 'undefined') {
    try {
      Object.defineProperty(Notification, 'permission', {
        get: function() { return 'default'; },
        configurable: true,
      });
    } catch(e) {}
  }
})();

// 6. WebRTC leak prevention — strip local IP candidates
(function() {
  if (typeof RTCPeerConnection === 'undefined') return;
  const OrigRTC = RTCPeerConnection;
  const localIPPattern = /(\b(10|172\.(1[6-9]|2\d|3[01])|192\.168)\.\d{1,3}\.\d{1,3}\b)|([0-9a-f]{1,4}(:[0-9a-f]{1,4}){7})/i;

  function PatchedRTC(config, constraints) {
    const pc = new OrigRTC(config, constraints);

    const origCreateOffer = pc.createOffer.bind(pc);
    pc.createOffer = function(options) {
      return origCreateOffer(options).then(function(offer) {
        if (offer && offer.sdp) {
          offer.sdp = offer.sdp.split('\n').filter(function(line) {
            if (line.indexOf('a=candidate') !== 0) return true;
            return !localIPPattern.test(line);
          }).join('\n');
        }
        return offer;
      });
    };

    const origOnIceCandidate = Object.getOwnPropertyDescriptor(
      RTCPeerConnection.prototype, 'onicecandidate'
    );
    if (origOnIceCandidate && origOnIceCandidate.set) {
      let userHandler = null;
      Object.defineProperty(pc, 'onicecandidate', {
        get: function() { return userHandler; },
        set: function(handler) {
          userHandler = function(event) {
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
  try {
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
  } catch(e) {}
})();

// 7. Font fingerprint spoofing — normalize font enumeration
(function() {
  if (typeof document === 'undefined' || !document.fonts) return;

  const commonFonts = [
    'Arial', 'Arial Black', 'Comic Sans MS', 'Courier New', 'Georgia',
    'Impact', 'Lucida Console', 'Lucida Sans Unicode', 'Palatino Linotype',
    'Tahoma', 'Times New Roman', 'Trebuchet MS', 'Verdana',
    'Microsoft Sans Serif', 'Segoe UI',
  ];

  const origCheck = document.fonts.check.bind(document.fonts);
  document.fonts.check = function(font, text) {
    try {
      const fontName = font.replace(/^[\d.]+\w+\s+/, '').replace(/['"]/g, '').trim();
      for (let i = 0; i < commonFonts.length; i++) {
        if (fontName.toLowerCase() === commonFonts[i].toLowerCase()) {
          return true;
        }
      }
    } catch(e) {}
    return origCheck(font, text);
  };

  const origForEach = document.fonts.forEach.bind(document.fonts);
  document.fonts.forEach = function(callback, thisArg) {
    const seen = new Set();
    origForEach(function(entry) {
      const family = entry.family.replace(/['"]/g, '');
      if (!seen.has(family)) {
        seen.add(family);
        callback.call(thisArg, entry);
      }
    });
  };
})();

// 8. Screen resolution consistency — match viewport dimensions
(function() {
  try {
    const w = window.innerWidth || 1920;
    const h = window.innerHeight || 1080;
    Object.defineProperty(screen, 'width',       { get: function() { return w; },  configurable: true });
    Object.defineProperty(screen, 'height',      { get: function() { return h; },  configurable: true });
    Object.defineProperty(screen, 'availWidth',  { get: function() { return w; },  configurable: true });
    Object.defineProperty(screen, 'availHeight', { get: function() { return h; },  configurable: true });
    Object.defineProperty(screen, 'colorDepth',  { get: function() { return 24; }, configurable: true });
    Object.defineProperty(screen, 'pixelDepth',  { get: function() { return 24; }, configurable: true });
  } catch(e) {}
})();

// 9. Battery API spoofing — return consistent desktop battery state
(function() {
  if (typeof navigator === 'undefined') return;
  const batteryInfo = {
    charging: true,
    chargingTime: 0,
    dischargingTime: Infinity,
    level: 1.0,
    addEventListener: function() {},
    removeEventListener: function() {},
    dispatchEvent: function() { return true; },
    onchargingchange: null,
    onchargingtimechange: null,
    ondischargingtimechange: null,
    onlevelchange: null,
  };
  Object.defineProperty(navigator, 'getBattery', {
    value: function() { return Promise.resolve(batteryInfo); },
    writable: true,
    configurable: true,
    enumerable: true,
  });
})();
`
