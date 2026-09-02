"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const loginGo = fs.readFileSync(path.join(__dirname, "..", "login.go"), "utf8");
const scriptMatch = loginGo.match(/  <script>\n([\s\S]*?)\n  <\/script>/);
assert.ok(scriptMatch, "login page script not found");
const source = scriptMatch[1];

function loadLogin({
  coarse = false,
  maxTouchPoints = 0,
} = {}) {
  const listeners = new Map();
  const timers = [];
  const button = { disabled: false };
  const form = {
    dataset: {},
    addEventListener(name, listener) {
      listeners.set(name, listener);
    },
    querySelector: () => button,
  };
  let blurred = 0;
  let scrolls = 0;
  let submitted = 0;
  const document = {
    activeElement: {
      blur() {
        blurred += 1;
      },
    },
    getElementById: () => form,
  };
  function HTMLFormElement() {}
  HTMLFormElement.prototype.submit = () => {
    submitted += 1;
  };
  const window = {
    matchMedia(query) {
      return {
        matches: query === "(pointer: coarse)" && coarse,
      };
    },
    scrollTo() {
      scrolls += 1;
    },
    setTimeout(callback, delay) {
      timers.push({ callback, delay });
    },
  };
  const navigator = {
    maxTouchPoints,
  };

  vm.runInNewContext(
    source,
    { document, HTMLFormElement, navigator, window },
    { filename: "login-inline.js" },
  );

  return {
    button,
    get blurred() {
      return blurred;
    },
    get scrolls() {
      return scrolls;
    },
    get submitted() {
      return submitted;
    },
    get submitListener() {
      return listeners.get("submit");
    },
    submit() {
      let prevented = false;
      listeners.get("submit")?.({
        preventDefault() {
          prevented = true;
        },
      });
      return prevented;
    },
    runTimers() {
      for (const timer of timers.splice(0)) timer.callback();
    },
    get timerDelays() {
      return timers.map((timer) => timer.delay);
    },
  };
}

test("touch browsers settle login focus before navigating to the terminal", () => {
  const browsers = [
    { maxTouchPoints: 5 },
    { coarse: true, maxTouchPoints: 1 },
  ];

  for (const browser of browsers) {
    const runtime = loadLogin(browser);

    assert.equal(typeof runtime.submitListener, "function");
    assert.equal(runtime.submit(), true);
    assert.equal(runtime.blurred, 1);
    assert.equal(runtime.scrolls, 1);
    assert.equal(runtime.button.disabled, true);
    assert.equal(runtime.submitted, 0);
    assert.deepEqual(runtime.timerDelays, [300]);
    assert.equal(runtime.submit(), true);
    assert.deepEqual(runtime.timerDelays, [300]);

    runtime.runTimers();
    assert.equal(runtime.submitted, 1);
  }
});

test("desktop login keeps native form submission", () => {
  const runtime = loadLogin();

  assert.equal(runtime.submitListener, undefined);
});
