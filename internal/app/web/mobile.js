(() => {
  "use strict";

  const root = document.documentElement;
  const viewport = window.visualViewport;
  let lastViewportMetrics = "";
  let viewportFrame = 0;
  let viewportTimer = 0;

  function updateViewport() {
    const height = Math.round(viewport ? viewport.height : window.innerHeight);
    const width = Math.round(viewport ? viewport.width : window.innerWidth);
    const top = Math.round(viewport ? viewport.offsetTop : 0);
    const left = Math.round(viewport ? viewport.offsetLeft : 0);
    const metrics = `${width}:${height}:${left}:${top}`;
    if (metrics === lastViewportMetrics) return;
    lastViewportMetrics = metrics;
    root.style.setProperty("--herdr-web-viewport-height", `${height}px`);
    root.style.setProperty("--herdr-web-viewport-width", `${width}px`);
    root.style.setProperty("--herdr-web-viewport-top", `${top}px`);
    root.style.setProperty("--herdr-web-viewport-left", `${left}px`);
    window.dispatchEvent(new Event("resize"));
  }

  function scheduleViewportUpdate() {
    if (viewportFrame) cancelAnimationFrame(viewportFrame);
    viewportFrame = requestAnimationFrame(() => {
      viewportFrame = 0;
      updateViewport();
    });
    clearTimeout(viewportTimer);
    viewportTimer = window.setTimeout(updateViewport, 80);
  }

  if (viewport) {
    viewport.addEventListener("resize", scheduleViewportUpdate, { passive: true });
    viewport.addEventListener("scroll", scheduleViewportUpdate, { passive: true });
  }
  window.addEventListener("resize", scheduleViewportUpdate, { passive: true });
  window.addEventListener("orientationchange", scheduleViewportUpdate, { passive: true });
  updateViewport();

  document.addEventListener("contextmenu", (event) => event.preventDefault());

  if (!(navigator.maxTouchPoints > 0 || window.matchMedia("(pointer: coarse)").matches)) {
    return;
  }

  function attachTouchScrolling(terminal) {
    if (terminal.dataset.herdrWebTouch === "ready") return;
    terminal.dataset.herdrWebTouch = "ready";

    let startY = 0;
    let lastY = 0;
    let lastX = 0;
    let lastTime = 0;
    let velocity = 0;
    let dragging = false;
    let animation = 0;

    function stopInertia() {
      if (animation) cancelAnimationFrame(animation);
      animation = 0;
      velocity = 0;
    }

    function sendWheel(deltaY, clientX, clientY) {
      terminal.dispatchEvent(
        new WheelEvent("wheel", {
          bubbles: true,
          cancelable: true,
          clientX,
          clientY,
          deltaMode: 0,
          deltaY,
          view: window,
        }),
      );
    }

    terminal.addEventListener(
      "touchstart",
      (event) => {
        stopInertia();
        if (event.touches.length !== 1) return;
        const touch = event.touches[0];
        startY = lastY = touch.clientY;
        lastX = touch.clientX;
        lastTime = performance.now();
        dragging = false;
      },
      { capture: true, passive: true },
    );

    terminal.addEventListener(
      "touchmove",
      (event) => {
        if (event.touches.length !== 1) return;
        const touch = event.touches[0];
        const now = performance.now();
        const deltaY = lastY - touch.clientY;
        if (!dragging && Math.abs(touch.clientY - startY) < 6) return;

        dragging = true;
        event.preventDefault();
        event.stopImmediatePropagation();
        sendWheel(deltaY, touch.clientX, touch.clientY);

        const elapsed = Math.max(1, now - lastTime);
        const frameVelocity = (deltaY / elapsed) * 16.67;
        velocity = velocity * 0.65 + frameVelocity * 0.35;
        lastY = touch.clientY;
        lastX = touch.clientX;
        lastTime = now;
      },
      { capture: true, passive: false },
    );

    terminal.addEventListener(
      "touchend",
      () => {
        if (!dragging || Math.abs(velocity) < 0.35) return;
        const glide = () => {
          velocity *= 0.92;
          if (Math.abs(velocity) < 0.35) {
            animation = 0;
            return;
          }
          sendWheel(velocity, lastX, lastY);
          animation = requestAnimationFrame(glide);
        };
        animation = requestAnimationFrame(glide);
      },
      { capture: true, passive: true },
    );

    terminal.addEventListener("touchcancel", stopInertia, { capture: true, passive: true });
  }

  function findTerminal() {
    const terminal = document.querySelector(".xterm");
    if (!terminal) return false;
    attachTouchScrolling(terminal);
    return true;
  }

  if (!findTerminal()) {
    const observer = new MutationObserver(() => {
      if (findTerminal()) observer.disconnect();
    });
    observer.observe(document.body, { childList: true, subtree: true });
  }
})();
