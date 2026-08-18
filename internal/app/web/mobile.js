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

  const holdDelay = 450;
  const dragThreshold = 6;
  const twoFingerTapDelay = 400;
  const twoFingerTapDistance = 12;

  function attachTouchControls(terminal) {
    if (terminal.dataset.herdrWebTouch === "ready") return;
    terminal.dataset.herdrWebTouch = "ready";

    const copyButton = document.createElement("button");
    copyButton.type = "button";
    copyButton.className = "herdr-web-copy-button";
    copyButton.textContent = "Copy";
    copyButton.hidden = true;
    copyButton.setAttribute("aria-label", "Copy terminal selection");
    terminal.appendChild(copyButton);

    for (const eventName of ["touchstart", "touchmove", "touchend", "touchcancel", "pointerdown", "mousedown"]) {
      copyButton.addEventListener(eventName, (event) => event.stopPropagation());
    }
    copyButton.addEventListener("click", (event) => {
      event.preventDefault();
      event.stopPropagation();
      copyButton.focus({ preventScroll: true });
      document.execCommand("copy");
      copyButton.hidden = true;
    });

    let startX = 0;
    let startY = 0;
    let lastY = 0;
    let lastX = 0;
    let lastTime = 0;
    let velocity = 0;
    let dragging = false;
    let animation = 0;
    let holdTimer = 0;
    let activeTouches = 0;
    let selecting = false;
    let selectionMoved = false;
    let twoFinger = false;
    let twoFingerEligible = false;
    let twoFingerSent = false;
    let twoFingerStartTime = 0;
    let twoFingerStartX = 0;
    let twoFingerStartY = 0;
    let twoFingerX = 0;
    let twoFingerY = 0;
    let twoFingerMovement = 0;

    function stopInertia() {
      if (animation) cancelAnimationFrame(animation);
      animation = 0;
      velocity = 0;
    }

    function cancelHold() {
      if (holdTimer) clearTimeout(holdTimer);
      holdTimer = 0;
    }

    function mouseTarget(clientX, clientY) {
      const target = document.elementFromPoint(clientX, clientY);
      return target && terminal.contains(target) ? target : terminal;
    }

    function sendMouse(type, clientX, clientY, button, buttons, forceSelection = false) {
      mouseTarget(clientX, clientY).dispatchEvent(
        new MouseEvent(type, {
          bubbles: true,
          cancelable: true,
          clientX,
          clientY,
          button,
          buttons,
          detail: type === "mousedown" ? 1 : 0,
          shiftKey: forceSelection,
          view: window,
        }),
      );
    }

    function beginSelection() {
      holdTimer = 0;
      if (activeTouches !== 1 || dragging || twoFinger) return;
      selecting = true;
      selectionMoved = false;
      sendMouse("mousedown", startX, startY, 0, 1, true);
    }

    function finishSelection(clientX, clientY) {
      sendMouse("mouseup", clientX, clientY, 0, 0, true);
      selecting = false;
      if (selectionMoved) copyButton.hidden = false;
    }

    function touchCenter(touches) {
      return {
        x: (touches[0].clientX + touches[1].clientX) / 2,
        y: (touches[0].clientY + touches[1].clientY) / 2,
      };
    }

    function startTwoFingerTap(event) {
      cancelHold();
      stopInertia();
      if (selecting) finishSelection(lastX, lastY);
      dragging = false;
      copyButton.hidden = true;
      const center = touchCenter(event.touches);
      twoFinger = true;
      twoFingerEligible = true;
      twoFingerSent = false;
      twoFingerStartTime = performance.now();
      twoFingerStartX = twoFingerX = center.x;
      twoFingerStartY = twoFingerY = center.y;
      twoFingerMovement = 0;
    }

    function sendRightClick() {
      sendMouse("mousedown", twoFingerX, twoFingerY, 2, 2);
      sendMouse("mouseup", twoFingerX, twoFingerY, 2, 0);
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
        if (event.target === copyButton) return;
        activeTouches = event.touches.length;
        copyButton.hidden = true;
        if (event.touches.length === 2) {
          startTwoFingerTap(event);
          event.preventDefault();
          event.stopImmediatePropagation();
          return;
        }
        if (event.touches.length !== 1) {
          cancelHold();
          twoFingerEligible = false;
          return;
        }
        stopInertia();
        const touch = event.touches[0];
        startX = lastX = touch.clientX;
        startY = lastY = touch.clientY;
        lastTime = performance.now();
        dragging = false;
        selectionMoved = false;
        cancelHold();
        holdTimer = window.setTimeout(beginSelection, holdDelay);
      },
      { capture: true, passive: false },
    );

    terminal.addEventListener(
      "touchmove",
      (event) => {
        if (event.target === copyButton) return;
        activeTouches = event.touches.length;
        if (twoFinger) {
          event.preventDefault();
          event.stopImmediatePropagation();
          if (event.touches.length !== 2) {
            twoFingerEligible = false;
            return;
          }
          const center = touchCenter(event.touches);
          twoFingerX = center.x;
          twoFingerY = center.y;
          twoFingerMovement = Math.max(
            twoFingerMovement,
            Math.hypot(center.x - twoFingerStartX, center.y - twoFingerStartY),
          );
          return;
        }
        if (event.touches.length !== 1) return;
        const touch = event.touches[0];
        if (selecting) {
          event.preventDefault();
          event.stopImmediatePropagation();
          if (Math.hypot(touch.clientX - startX, touch.clientY - startY) >= dragThreshold) {
            selectionMoved = true;
          }
          lastX = touch.clientX;
          lastY = touch.clientY;
          sendMouse("mousemove", lastX, lastY, 0, 1, true);
          return;
        }
        const now = performance.now();
        const deltaY = lastY - touch.clientY;
        if (!dragging && Math.hypot(touch.clientX - startX, touch.clientY - startY) < dragThreshold) {
          return;
        }

        dragging = true;
        cancelHold();
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
      (event) => {
        if (event.target === copyButton) return;
        activeTouches = event.touches.length;
        if (twoFinger) {
          event.preventDefault();
          event.stopImmediatePropagation();
          if (!twoFingerSent && event.touches.length < 2) {
            const elapsed = performance.now() - twoFingerStartTime;
            if (twoFingerEligible && elapsed <= twoFingerTapDelay && twoFingerMovement <= twoFingerTapDistance) {
              sendRightClick();
            }
            twoFingerSent = true;
          }
          if (event.touches.length === 0) {
            twoFinger = false;
            twoFingerEligible = false;
          }
          return;
        }
        cancelHold();
        if (selecting) {
          event.preventDefault();
          event.stopImmediatePropagation();
          const touch = event.changedTouches[0];
          finishSelection(touch ? touch.clientX : lastX, touch ? touch.clientY : lastY);
          return;
        }
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
      { capture: true, passive: false },
    );

    terminal.addEventListener(
      "touchcancel",
      (event) => {
        if (event.target === copyButton) return;
        cancelHold();
        stopInertia();
        activeTouches = 0;
        twoFinger = false;
        twoFingerEligible = false;
        if (selecting) {
          const touch = event.changedTouches[0];
          finishSelection(touch ? touch.clientX : lastX, touch ? touch.clientY : lastY);
        }
      },
      { capture: true, passive: true },
    );
  }

  function findTerminal() {
    const terminal = document.querySelector(".xterm");
    if (!terminal) return false;
    attachTouchControls(terminal);
    return true;
  }

  if (!findTerminal()) {
    const observer = new MutationObserver(() => {
      if (findTerminal()) observer.disconnect();
    });
    observer.observe(document.body, { childList: true, subtree: true });
  }
})();
