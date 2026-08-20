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

function loadTouchMobile({
  copyEventSupported = true,
  promptValue = null,
  selection = "selected text",
} = {}) {
  const documentListeners = new Map();
  const rootClasses = new Set();
  const terminalInputs = [];
  const terminalPastes = [];
  const prompts = [];
  let copiedText = "";
  let selectedControl = null;
  let document;

  function createElement(tagName) {
    const listeners = new Map();
    const element = {
      tagName: tagName.toUpperCase(),
      children: [],
      dataset: {},
      style: {},
      className: "",
      hidden: false,
      disabled: false,
      textContent: "",
      value: "",
      parentNode: null,
      classList: {
        contains(name) {
          return element.className.split(/\s+/).includes(name);
        },
      },
      addEventListener(name, listener) {
        const registered = listeners.get(name) || [];
        registered.push(listener);
        listeners.set(name, registered);
      },
      appendChild(child) {
        child.parentNode = element;
        element.children.push(child);
        return child;
      },
      contains(target) {
        return target === element || element.children.some((child) => child.contains?.(target));
      },
      setAttribute() {},
      focus() {
        document.activeElement = element;
      },
      select() {
        selectedControl = element;
      },
      setSelectionRange() {
        selectedControl = element;
      },
      remove() {
        if (!element.parentNode) return;
        element.parentNode.children = element.parentNode.children.filter((child) => child !== element);
        element.parentNode = null;
      },
      listeners,
    };
    return element;
  }

  const rootElement = createElement("html");
  rootElement.style.setProperty = () => {};
  rootElement.classList = {
    contains: (name) => rootClasses.has(name),
    toggle(name, enabled) {
      if (enabled) rootClasses.add(name);
      else rootClasses.delete(name);
    },
  };
  const body = createElement("body");
  const terminal = createElement("div");
  terminal.className = "xterm";
  const helper = createElement("textarea");
  helper.className = "xterm-helper-textarea";
  terminal.appendChild(helper);

  function dispatchDocument(name, event) {
    for (const listener of documentListeners.get(name) || []) listener(event);
  }

  document = {
    documentElement: rootElement,
    body,
    activeElement: body,
    createElement,
    addEventListener(name, listener) {
      const registered = documentListeners.get(name) || [];
      registered.push(listener);
      documentListeners.set(name, registered);
    },
    querySelector(selector) {
      if (selector === ".xterm") return terminal;
      if (selector === ".xterm-helper-textarea") return helper;
      return null;
    },
    elementFromPoint: () => terminal,
    getSelection: () => ({ toString: () => "" }),
    execCommand(command) {
      if (command !== "copy" || !selectedControl) return false;
      if (copyEventSupported) {
        for (const listener of selectedControl.listeners.get("copy") || []) {
          listener({
            clipboardData: {
              setData(type, value) {
                if (type === "text/plain") copiedText = value;
              },
            },
            preventDefault() {},
          });
        }
      }
      // WebKit may report false even when the copy event accepted text.
      return false;
    },
  };

  const visualViewport = {
    height: 1024,
    width: 1366,
    offsetTop: 0,
    offsetLeft: 0,
    pageTop: 0,
    pageLeft: 0,
    addEventListener() {},
  };
  const window = {
    visualViewport,
    innerHeight: 1024,
    innerWidth: 1366,
    scrollY: 0,
    scrollX: 0,
    isSecureContext: false,
    addEventListener() {},
    dispatchEvent() {},
    matchMedia: () => ({ matches: true }),
    setTimeout(callback) {
      callback();
      return 1;
    },
    prompt(...args) {
      prompts.push(args);
      return promptValue;
    },
  };
  window.term = {
    input(data, wasUserInput) {
      terminalInputs.push({ data, wasUserInput });
    },
    paste(data) {
      terminalPastes.push(data);
    },
    getSelection: () => selection,
    focus() {
      helper.focus();
    },
  };

  const context = {
    window,
    document,
    navigator: {
      userAgent: "Mozilla/5.0 (iPad) Version/26.0 Mobile/15E148 Safari/604.1",
      platform: "iPad",
      maxTouchPoints: 5,
    },
    Event: class {},
    MutationObserver: class {
      observe() {}
      disconnect() {}
    },
    requestAnimationFrame(callback) {
      callback();
      return 1;
    },
    cancelAnimationFrame() {},
    clearTimeout() {},
  };
  vm.runInNewContext(source, context, { filename: "mobile.js" });

  const copyButton = terminal.children.find((child) => child.className === "herdr-web-copy-button");
  const toolbar = body.children.find((child) => child.className === "herdr-web-input-toolbar");

  return {
    copyButton,
    toolbar,
    terminalInputs,
    terminalPastes,
    prompts,
    get copiedText() {
      return copiedText;
    },
    focusTerminal() {
      helper.focus();
      dispatchDocument("focusin", { target: helper });
    },
    async click(element) {
      const event = { preventDefault() {}, stopPropagation() {} };
      for (const listener of element.listeners.get("click") || []) listener(event);
      await Promise.resolve();
      await Promise.resolve();
    },
    toolbarButton(label) {
      return toolbar.children.find((button) => button.textContent === label);
    },
    rootHasClass(name) {
      return rootClasses.has(name);
    },
  };
}

test("copy button writes the xterm selection on LAN HTTP", async () => {
  const runtime = loadTouchMobile({ selection: "first line\nsecond line" });

  await runtime.click(runtime.copyButton);

  assert.equal(runtime.copiedText, "first line\nsecond line");
  assert.equal(runtime.copyButton.hidden, true);
  assert.equal(runtime.copyButton.disabled, false);
});

test("copy button offers a native manual field when WebKit rejects programmatic copy", async () => {
  const runtime = loadTouchMobile({ copyEventSupported: false, selection: "manual text" });

  await runtime.click(runtime.copyButton);

  assert.deepEqual(runtime.prompts, [["Copy selected text", "manual text"]]);
  assert.equal(runtime.copyButton.hidden, true);
  assert.equal(runtime.copyButton.disabled, false);
});

test("terminal focus shows input toolbar with key and paste actions", async () => {
  const runtime = loadTouchMobile({ promptValue: "pasted text" });

  runtime.focusTerminal();

  assert.equal(runtime.toolbar.hidden, false);
  assert.equal(runtime.rootHasClass("herdr-web-toolbar-visible"), true);
  await runtime.click(runtime.toolbarButton("Esc"));
  await runtime.click(runtime.toolbarButton("Tab"));
  await runtime.click(runtime.toolbarButton("Paste"));
  await runtime.click(runtime.toolbarButton("Enter"));
  assert.deepEqual(runtime.terminalInputs, [
    { data: "\x1b", wasUserInput: true },
    { data: "\t", wasUserInput: true },
    { data: "\r", wasUserInput: true },
  ]);
  assert.deepEqual(runtime.terminalPastes, ["pasted text"]);
});

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
