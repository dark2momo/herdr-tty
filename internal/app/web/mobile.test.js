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
}) {
  const styles = new Map();
  const documentListeners = new Map();
  const viewportListeners = new Map();
  const noop = () => {};
  let nextTimer = 1;

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
    setTimeout(callback) {
      callback();
      return nextTimer++;
    },
  };
  const terminalInputs = [];
  window.term = {
    input(data, wasUserInput) {
      terminalInputs.push({ data, wasUserInput });
    },
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
    dispatchBeforeInput(overrides = {}) {
      let prevented = false;
      let stopped = false;
      const target = overrides.target || {
        classList: { contains: (name) => name === "xterm-helper-textarea" },
        selectionEnd: 2,
        selectionStart: 2,
        value: "中文",
        setSelectionRange(start, end) {
          this.selectionStart = start;
          this.selectionEnd = end;
        },
      };
      documentListeners.get("beforeinput")({
        cancelable: true,
        data: "，",
        defaultPrevented: false,
        inputType: "insertText",
        isComposing: false,
        target,
        preventDefault() {
          prevented = true;
        },
        stopImmediatePropagation() {
          stopped = true;
        },
        ...overrides,
      });
      return { prevented, stopped, target };
    },
    dispatchInput(overrides = {}) {
      let stopped = false;
      documentListeners.get("input")({
        data: "，",
        inputType: "insertText",
        stopImmediatePropagation() {
          stopped = true;
        },
        ...overrides,
      });
      return { stopped };
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

test("iOS virtual Chinese punctuation is forwarded as non-composition input", () => {
  const runtime = loadMobile({
    userAgent: "Mozilla/5.0 (iPhone) CriOS/140.0 Mobile/15E148 Safari/604.1",
    maxTouchPoints: 5,
  });

  const result = runtime.dispatchBeforeInput({ data: "，。！？" });

  assert.equal(result.prevented, true);
  assert.equal(result.stopped, true);
  assert.deepEqual(runtime.terminalInputs, [{ data: "，。！？", wasUserInput: true }]);
});

test("non-cancelable iOS punctuation is restored and forwarded once on input", () => {
  const runtime = loadMobile({
    userAgent: "Mozilla/5.0 (iPhone) Version/26.0 Mobile/15E148 Safari/604.1",
    maxTouchPoints: 5,
  });

  const before = runtime.dispatchBeforeInput({ cancelable: false, data: "、" });
  assert.equal(before.prevented, false);
  assert.equal(before.stopped, true);
  assert.deepEqual(runtime.terminalInputs, []);

  before.target.value = "中文、";
  before.target.selectionStart = before.target.selectionEnd = 3;
  const input = runtime.dispatchInput({ data: "、", target: before.target });

  assert.equal(input.stopped, true);
  assert.equal(before.target.value, "中文");
  assert.equal(before.target.selectionStart, 2);
  assert.equal(before.target.selectionEnd, 2);
  assert.deepEqual(runtime.terminalInputs, [{ data: "、", wasUserInput: true }]);
});

for (const input of [
  { name: "ordinary text", overrides: { data: "中" } },
  { name: "active composition", overrides: { isComposing: true } },
  { name: "deletion", overrides: { data: null, inputType: "deleteContentBackward" } },
  {
    name: "non-terminal input",
    overrides: { target: { classList: { contains: () => false }, dispatchEvent() {} } },
  },
]) {
  test(`iOS punctuation fallback ignores ${input.name}`, () => {
    const runtime = loadMobile({
      userAgent: "Mozilla/5.0 (iPhone) Version/26.0 Mobile/15E148 Safari/604.1",
      maxTouchPoints: 5,
    });

    const result = runtime.dispatchBeforeInput(input.overrides);

    assert.equal(result.prevented, false);
    assert.equal(result.stopped, false);
    assert.deepEqual(runtime.terminalInputs, []);
  });
}

test("non-iOS virtual keyboards keep native input handling", () => {
  const runtime = loadMobile({
    userAgent: "Mozilla/5.0 (Linux; Android 16) Chrome/140.0 Mobile Safari/537.36",
    maxTouchPoints: 5,
  });

  const result = runtime.dispatchBeforeInput({ data: "，" });

  assert.equal(result.prevented, false);
  assert.equal(result.stopped, false);
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
