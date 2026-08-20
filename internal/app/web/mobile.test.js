"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "mobile.js"), "utf8");

function loadMobile({
  userAgent,
  platform = "",
  maxTouchPoints,
  layoutHeight = 1024,
  layoutWidth = 1366,
  deferTimers = false,
}) {
  const styles = new Map();
  const documentListeners = new Map();
  const textareaListeners = new Map();
  const viewportListeners = new Map();
  const noop = () => {};
  const timers = new Map();
  let nextTimer = 1;
  let originalKeydownCalls = 0;

  const visualViewport = {
    height: layoutHeight,
    width: layoutWidth,
    offsetTop: 0,
    offsetLeft: 0,
    pageTop: 0,
    pageLeft: 0,
    addEventListener(name, listener) {
      viewportListeners.set(name, listener);
    },
  };
  const window = {
    visualViewport,
    innerHeight: layoutHeight,
    innerWidth: layoutWidth,
    scrollY: 0,
    scrollX: 0,
    addEventListener: noop,
    dispatchEvent: noop,
    matchMedia: () => ({ matches: false }),
    clearTimeout(timer) {
      timers.delete(timer);
    },
    setTimeout(callback) {
      const timer = nextTimer++;
      if (deferTimers) timers.set(timer, callback);
      else callback();
      return timer;
    },
  };
  const terminalInputs = [];
  const textarea = {
    value: "",
    addEventListener(name, listener) {
      textareaListeners.set(name, listener);
    },
  };
  const compositionHelper = {
    _coreService: {
      triggerDataEvent(data, wasUserInput) {
        terminalInputs.push({ data, wasUserInput });
      },
    },
    _dataAlreadySent: "",
    _isComposing: false,
    _isSendingComposition: false,
    _textarea: textarea,
    keydown() {
      originalKeydownCalls++;
      return true;
    },
  };
  window.term = {
    _core: { _compositionHelper: compositionHelper },
  };
  const document = {
    documentElement: {
      style: {
        setProperty(name, value) {
          styles.set(name, value);
        },
      },
    },
    body: {},
    addEventListener(name, listener) {
      documentListeners.set(name, listener);
    },
    querySelector: () => null,
  };
  const context = {
    window,
    document,
    navigator: { userAgent, platform, maxTouchPoints },
    Event: class {},
    MutationObserver: class {
      observe() {}
      disconnect() {}
    },
    requestAnimationFrame(callback) {
      callback();
      return nextTimer++;
    },
    cancelAnimationFrame: noop,
    clearTimeout: noop,
  };

  vm.runInNewContext(source, context, { filename: "mobile.js" });

  return {
    styles,
    terminalInputs,
    compositionHelper,
    textarea,
    dispatchKeydown(overrides = {}) {
      return compositionHelper.keydown({ key: "，", keyCode: 229, ...overrides });
    },
    dispatchKeyup(overrides = {}) {
      textareaListeners.get("keyup")?.({ key: "，", keyCode: 0, ...overrides });
    },
    originalKeydownCalls() {
      return originalKeydownCalls;
    },
    flushTimers() {
      while (timers.size > 0) {
        const queued = [...timers.entries()];
        timers.clear();
        for (const [, callback] of queued) callback();
      }
    },
    updateViewport({ height, width = layoutWidth, top = 0, left = 0 }) {
      visualViewport.height = height;
      visualViewport.width = width;
      visualViewport.offsetTop = top;
      visualViewport.offsetLeft = left;
      visualViewport.pageTop = top;
      visualViewport.pageLeft = left;
      documentListeners.get("focusin")();
    },
  };
}

test("iOS keyCode 229 punctuation is forwarded on keyup", () => {
  const runtime = loadMobile({
    userAgent: "Mozilla/5.0 (iPhone) CriOS/140.0 Mobile/15E148 Safari/604.1",
    maxTouchPoints: 5,
  });

  runtime.textarea.value = "中文";
  const handled = runtime.dispatchKeydown();
  runtime.textarea.value = "中文，";
  runtime.dispatchKeyup();

  assert.equal(handled, false);
  assert.equal(runtime.originalKeydownCalls(), 0);
  assert.deepEqual(runtime.terminalInputs, [{ data: "，", wasUserInput: true }]);
});

test("iOS enumeration comma follows the same pending 229 path", () => {
  const runtime = loadMobile({
    userAgent: "Mozilla/5.0 (iPhone) Version/26.0 Mobile/15E148 Safari/604.1",
    maxTouchPoints: 5,
  });

  runtime.textarea.value = "中文";
  runtime.dispatchKeydown({ key: "、" });
  runtime.textarea.value = "中文、";
  runtime.dispatchKeyup({ key: "、" });

  assert.deepEqual(runtime.terminalInputs, [{ data: "、", wasUserInput: true }]);
});

test("iOS timer fallback sends once and keyup does not duplicate it", () => {
  const runtime = loadMobile({
    userAgent: "Mozilla/5.0 (iPhone) Version/26.0 Mobile/15E148 Safari/604.1",
    maxTouchPoints: 5,
    deferTimers: true,
  });

  runtime.textarea.value = "中文";
  runtime.dispatchKeydown();
  runtime.textarea.value = "中文，";
  runtime.flushTimers();
  runtime.dispatchKeyup();

  assert.deepEqual(runtime.terminalInputs, [{ data: "，", wasUserInput: true }]);
});

test("iOS double-space replacement sends a precise delete and insert", () => {
  const runtime = loadMobile({
    userAgent: "Mozilla/5.0 (iPhone) Version/26.0 Mobile/15E148 Safari/604.1",
    maxTouchPoints: 5,
  });

  runtime.textarea.value = "中文 ";
  runtime.dispatchKeydown({ key: " " });
  runtime.textarea.value = "中文。";
  runtime.dispatchKeyup({ key: " " });

  assert.deepEqual(runtime.terminalInputs, [{ data: "\x7f。", wasUserInput: true }]);
});

test("ordinary keys and active composition stay on xterm's native path", () => {
  const runtime = loadMobile({
    userAgent: "Mozilla/5.0 (iPhone) Version/26.0 Mobile/15E148 Safari/604.1",
    maxTouchPoints: 5,
  });

  assert.equal(runtime.dispatchKeydown({ key: "a", keyCode: 65 }), true);
  runtime.compositionHelper._isComposing = true;
  assert.equal(runtime.dispatchKeydown(), true);
  runtime.dispatchKeyup();

  assert.equal(runtime.originalKeydownCalls(), 2);
  assert.deepEqual(runtime.terminalInputs, []);
});

test("non-iOS virtual keyboards keep native input handling", () => {
  const runtime = loadMobile({
    userAgent: "Mozilla/5.0 (Linux; Android 16) Chrome/140.0 Mobile Safari/537.36",
    maxTouchPoints: 5,
  });

  const result = runtime.dispatchKeydown();

  assert.equal(result, true);
  assert.equal(runtime.originalKeydownCalls(), 1);
  assert.deepEqual(runtime.terminalInputs, []);
});

function viewportStyles(runtime) {
  return {
    height: runtime.styles.get("--herdr-web-viewport-height"),
    width: runtime.styles.get("--herdr-web-viewport-width"),
    top: runtime.styles.get("--herdr-web-viewport-top"),
    left: runtime.styles.get("--herdr-web-viewport-left"),
  };
}

test("iPad Chrome keeps the layout viewport after input focus", () => {
  const runtime = loadMobile({
    userAgent: "Mozilla/5.0 (iPad) CriOS/140.0 Mobile/15E148 Safari/604.1",
    maxTouchPoints: 5,
  });

  runtime.updateViewport({ height: 930, top: 94 });

  assert.deepEqual(viewportStyles(runtime), {
    height: "1024px",
    width: "1366px",
    top: "0px",
    left: "0px",
  });
});

test("iPad Chrome keeps the keyboard above the terminal", () => {
  const runtime = loadMobile({
    userAgent: "Mozilla/5.0 (iPad) CriOS/140.0 Mobile/15E148 Safari/604.1",
    maxTouchPoints: 5,
  });

  runtime.updateViewport({ height: 650, top: 84 });

  assert.deepEqual(viewportStyles(runtime), {
    height: "650px",
    width: "1366px",
    top: "84px",
    left: "0px",
  });
});

test("iPad Chrome desktop mode also keeps the keyboard above the terminal", () => {
  const runtime = loadMobile({
    userAgent: "Mozilla/5.0 (Macintosh) CriOS/140.0 Safari/604.1",
    platform: "MacIntel",
    maxTouchPoints: 5,
  });

  runtime.updateViewport({ height: 650, top: 84 });

  assert.deepEqual(viewportStyles(runtime), {
    height: "650px",
    width: "1366px",
    top: "84px",
    left: "0px",
  });
});

for (const browser of [
  {
    name: "iPad Safari",
    userAgent: "Mozilla/5.0 Version/26.0 Mobile/15E148 Safari/604.1",
    maxTouchPoints: 5,
  },
  {
    name: "desktop Chrome",
    userAgent: "Mozilla/5.0 Chrome/140.0 Safari/537.36",
    maxTouchPoints: 0,
  },
  {
    name: "iPhone Chrome",
    userAgent: "Mozilla/5.0 (iPhone) CriOS/140.0 Mobile/15E148 Safari/604.1",
    maxTouchPoints: 5,
  },
]) {
  test(`${browser.name} keeps the visual viewport behavior`, () => {
    const runtime = loadMobile(browser);

    runtime.updateViewport({ height: 930, width: 1300, top: 94, left: 12 });

    assert.deepEqual(viewportStyles(runtime), {
      height: "930px",
      width: "1300px",
      top: "94px",
      left: "12px",
    });
  });
}
