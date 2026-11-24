import type { Page } from "@playwright/test";

/** MIN_TARGET is the smallest width and height of a pointer target, in px (WCAG 2.5.8). */
export const MIN_TARGET = 24;

/**
 * smallTargets returns one line for each control on the page that a pointer
 * hits in a box under MIN_TARGET wide or high, "<tag> "<name>" <w>x<h>". The
 * box is what a hit test through the center of the control gives to it or to
 * its labels, so a part that another element covers does not count. A link
 * in a sentence, a disabled control, a control that is not shown, and a
 * control under aria-hidden are not counted. An empty list means that the
 * page passes.
 */
export async function smallTargets(page: Page): Promise<string[]> {
  return page.evaluate((min) => {
    const CONTROLS =
      "a[href], button, input:not([type=hidden]), select, textarea, summary, " +
      "[role=button], [role=link], [role=checkbox], [role=radio], " +
      "[role=switch], [role=tab], [role=menuitem], [role=option]";
    const shown = (el: Element) => {
      const r = el.getBoundingClientRect();
      const style = getComputedStyle(el);
      // checkVisibility is false inside a closed details, which keeps the
      // box of its content.
      return (
        r.width > 1 &&
        r.height > 1 &&
        style.visibility !== "hidden" &&
        el.checkVisibility() &&
        !el.closest("[aria-hidden=true]")
      );
    };
    // WCAG 2.5.8 exempts a target in a sentence or a block of text.
    const inRunningText = (el: Element) => {
      if (el.tagName !== "A" || getComputedStyle(el).display !== "inline")
        return false;
      return [...(el.parentElement?.childNodes ?? [])].some(
        (n) =>
          n.nodeType === Node.TEXT_NODE &&
          /\p{L}{2}/u.test(n.textContent ?? ""),
      );
    };
    // The open popup of a trigger sits beside it and does not take its target.
    const owns = (el: Element, x: number, y: number) => {
      const hit = document.elementFromPoint(x, y);
      if (!hit) return false;
      const labels = [...((el as HTMLInputElement).labels ?? [])];
      const popup =
        el.getAttribute("aria-expanded") === "true"
          ? document.getElementById(el.getAttribute("aria-controls") ?? "")
          : null;
      return [el, ...labels, popup].some((box) => box?.contains(hit));
    };
    const ownedSteps = (
      el: Element,
      x: number,
      y: number,
      dx: number,
      dy: number,
    ) => {
      let n = 0;
      while (n < min && owns(el, x + dx * (n + 1), y + dy * (n + 1))) n++;
      return n;
    };
    const size = (el: Element) => {
      el.scrollIntoView({ block: "center", inline: "center" });
      // A wrapped link has a box for each line: measure the largest.
      const r = [...el.getClientRects()].reduce((a, b) =>
        b.width * b.height > a.width * a.height ? b : a,
      );
      const x = r.left + r.width / 2;
      const y = r.top + r.height / 2;
      if (!owns(el, x, y)) return { w: 0, h: 0 };
      return {
        w: ownedSteps(el, x, y, -1, 0) + ownedSteps(el, x, y, 1, 0) + 1,
        h: ownedSteps(el, x, y, 0, -1) + ownedSteps(el, x, y, 0, 1) + 1,
      };
    };
    const lines: string[] = [];
    for (const el of document.querySelectorAll(CONTROLS)) {
      if (!shown(el) || inRunningText(el)) continue;
      if (el.matches(":disabled, [aria-disabled=true]")) continue;
      const { w, h } = size(el);
      if (w >= min && h >= min) continue;
      const name = (
        el.getAttribute("aria-label") ??
        (el as HTMLElement).innerText ??
        ""
      )
        .trim()
        .slice(0, 40);
      lines.push(`${el.tagName.toLowerCase()} "${name}" ${w}x${h}`);
    }
    return lines;
  }, MIN_TARGET);
}
