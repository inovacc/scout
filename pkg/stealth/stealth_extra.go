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
`
