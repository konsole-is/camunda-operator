import type { Page } from "@playwright/test";

/** MIN_TARGET is the smallest width and height of a pointer target, in px (WCAG 2.5.8). */
export const MIN_TARGET = 24;

/**
 * smallTargets returns one line for each control on the page that a pointer
 * hits in a box under MIN_TARGET wide or high, "<tag> "<name>" <w>x<h>". The
 * box of a control is the largest of its own box, the box of an absolute
 * ::before or ::after of it, and the box of each of its labels. A link in
 * running text, a disabled control, and a control that is not shown are not
 * counted. An empty list means that the page passes.
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
      return r.width > 1 && r.height > 1 && style.visibility !== "hidden";
    };
    // WCAG 2.5.8 exempts a target in a sentence or a block of text.
    const inRunningText = (el: Element) => {
      if (el.tagName !== "A" || getComputedStyle(el).display !== "inline")
        return false;
      return [...(el.parentElement?.childNodes ?? [])].some(
        (n) => n !== el && (n.textContent ?? "").trim() !== "",
      );
    };
    const size = (el: Element) => {
      const r = el.getBoundingClientRect();
      let w = r.width;
      let h = r.height;
      for (const pseudo of ["::before", "::after"]) {
        const style = getComputedStyle(el, pseudo);
        if (style.content === "none" || style.position !== "absolute") continue;
        w = Math.max(w, parseFloat(style.width) || 0);
        h = Math.max(h, parseFloat(style.height) || 0);
      }
      for (const label of (el as HTMLInputElement).labels ?? []) {
        const l = label.getBoundingClientRect();
        w = Math.max(w, l.width);
        h = Math.max(h, l.height);
      }
      return { w: Math.round(w), h: Math.round(h) };
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
