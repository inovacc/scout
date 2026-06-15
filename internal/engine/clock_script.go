package engine

// clockScript defines window.__scoutClock, a compact fake-timers shim modeled
// on Playwright's Clock. install(ms) overrides Date, setTimeout/setInterval,
// performance.now and requestAnimationFrame so page time is controlled from Go:
//
//	install(ms)        set the clock and override the timer/date globals (idempotent)
//	setSystemTime(ms)  jump the current time without firing timers
//	setFixedTime(ms)   pin Date to ms (alias of setSystemTime for our purposes)
//	tick(ms)           advance time by ms, firing every timer that comes due in order
//	now()              read the current emulated time (ms since epoch)
const clockScript = `window.__scoutClock = (function () {
  var RealDate = window.Date;
  var now = 0, perfOrigin = 0, installed = false;
  var timers = [], seq = 1;

  function FakeDate() {
    if (arguments.length === 0) { return new RealDate(now); }
    return new (RealDate.bind.apply(RealDate, [null].concat([].slice.call(arguments))))();
  }
  FakeDate.prototype = RealDate.prototype;
  FakeDate.now = function () { return Math.floor(now); };
  FakeDate.parse = RealDate.parse;
  FakeDate.UTC = RealDate.UTC;

  function schedule(cb, delay, interval, args) {
    var id = seq++;
    timers.push({ id: id, cb: cb, due: now + (delay || 0), interval: interval || 0, args: args || [] });
    return id;
  }
  function clear(id) { timers = timers.filter(function (t) { return t.id !== id; }); }

  function tick(ms) {
    var target = now + ms;
    while (true) {
      var due = null;
      for (var i = 0; i < timers.length; i++) {
        var t = timers[i];
        if (t.due <= target && (due === null || t.due < due.due || (t.due === due.due && t.id < due.id))) {
          due = t;
        }
      }
      if (!due) break;
      now = due.due;
      if (due.interval > 0) { due.due = now + due.interval; }
      else { timers = timers.filter(function (x) { return x !== due; }); }
      try { due.cb.apply(null, due.args); } catch (e) {}
    }
    now = target;
  }

  function install(ms) {
    now = ms; perfOrigin = ms;
    if (installed) return;
    installed = true;
    window.Date = FakeDate;
    window.setTimeout = function (cb, delay) { return schedule(cb, delay, 0, [].slice.call(arguments, 2)); };
    window.setInterval = function (cb, delay) { return schedule(cb, delay, delay || 0, [].slice.call(arguments, 2)); };
    window.clearTimeout = clear;
    window.clearInterval = clear;
    window.requestAnimationFrame = function (cb) { return schedule(function () { cb(now); }, 16, 0, []); };
    window.cancelAnimationFrame = clear;
    if (window.performance) { window.performance.now = function () { return now - perfOrigin; }; }
  }

  return {
    install: install,
    setSystemTime: function (ms) { now = ms; },
    setFixedTime: function (ms) { now = ms; },
    tick: tick,
    now: function () { return Math.floor(now); }
  };
})();`
