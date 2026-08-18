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
