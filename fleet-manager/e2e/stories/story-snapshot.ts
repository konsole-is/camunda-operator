import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import type { Page } from "@playwright/test";

/**
 * renderStoryMarkup returns the markup of a story page, one tag or text on
 * each line, indented by depth, so that a line-based diff shows a change.
 * body is the body of the Storybook preview frame. The result holds the
 * story root and each element that the story added to the body, such as a
 * dialog in a portal, and leaves out the elements that Storybook and its
 * addons put in the frame. It holds no script and no style element. Each run
 * of white space in a text becomes one space, and a text with only white
 * space stays as one space. Preformatted text keeps each of its lines and
 * spaces.
 *
 * Playwright runs it in the browser, so it uses nothing from outside its own
 * body.
 */
export function renderStoryMarkup(body: Element): string {
  const skipped = new Set(["script", "style", "link", "meta", "template"]);
  const lines: string[] = [];
  const escapeText = (text: string) =>
    text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  const escapeAttribute = (text: string) =>
    text.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
  const preformatted = (node: Node) => {
    const parent = node.parentElement;
    if (!parent) return false;
    return (
      parent.closest("pre, textarea") !== null ||
      /^(pre|break-spaces)/.test(getComputedStyle(parent).whiteSpace)
    );
  };
  const textLines = (node: Node) => {
    const text = node.textContent ?? "";
    return preformatted(node)
      ? text.split("\n").map(escapeText)
      : [escapeText(text.replace(/\s+/g, " "))];
  };
  const walk = (node: Node, depth: number) => {
    const pad = "  ".repeat(depth);
    if (node.nodeType === 3) {
      for (const line of textLines(node)) lines.push(pad + line);
      return;
    }
    if (node.nodeType !== 1) return;
    const element = node as Element;
    const tag = element.localName;
    if (skipped.has(tag)) return;
    const attributes = Array.from(element.attributes)
      .map((a) => ` ${a.name}="${escapeAttribute(a.value)}"`)
      .join("");
    const children = Array.from(element.childNodes).filter(
      (child) =>
        (child.nodeType === 1 && !skipped.has((child as Element).localName)) ||
        (child.nodeType === 3 && child.textContent !== ""),
    );
    const only =
      children.length === 1 && children[0].nodeType === 3
        ? textLines(children[0])
        : [];
    if (children.length === 0) {
      lines.push(`${pad}<${tag}${attributes}></${tag}>`);
    } else if (only.length === 1) {
      lines.push(`${pad}<${tag}${attributes}>${only[0]}</${tag}>`);
    } else {
      lines.push(`${pad}<${tag}${attributes}>`);
      for (const child of children) walk(child, depth + 1);
      lines.push(`${pad}</${tag}>`);
    }
  };
  for (const child of Array.from(body.children)) {
    // Storybook and its addons give each element of their own an id that
    // starts with storybook-, or a class that starts with sb-.
    if (child.id.startsWith("storybook-") && child.id !== "storybook-root") {
      continue;
    }
    if (Array.from(child.classList).some((c) => c.startsWith("sb-"))) continue;
    walk(child, 0);
  }
  return lines.join("\n");
}

/**
 * renderStylesheetRules returns the rules of each stylesheet that the page
 * loads from a file, one rule on each line, in the order of the page. A
 * nested rule starts with the rules that hold it, such as
 * "@layer utilities > @media (width >= 40rem) > .sm\:flex { display: flex; }".
 * It skips each stylesheet whose URL is in seen, and returns seen with the
 * URLs of the stylesheets that it read.
 *
 * Playwright runs it in the browser, so it uses nothing from outside its own
 * body.
 */
export function renderStylesheetRules(seen: string[]): {
  rules: string;
  seen: string[];
} {
  const lines: string[] = [];
  const oneLine = (text: string) => text.replace(/\s+/g, " ").trim();
  const walk = (rules: CSSRuleList, prefix: string) => {
    for (const rule of Array.from(rules)) {
      if (rule instanceof CSSStyleRule) {
        lines.push(
          `${prefix}${rule.selectorText} { ${oneLine(rule.style.cssText)} }`,
        );
        if (rule.cssRules?.length) {
          walk(rule.cssRules, `${prefix}${rule.selectorText} > `);
        }
      } else if (rule instanceof CSSGroupingRule) {
        const text = rule.cssText;
        walk(
          rule.cssRules,
          `${prefix}${oneLine(text.slice(0, text.indexOf("{")))} > `,
        );
      } else {
        lines.push(prefix + oneLine(rule.cssText));
      }
    }
  };
  const read = [...seen];
  for (const sheet of Array.from(document.styleSheets)) {
    if (!sheet.href || read.includes(sheet.href)) continue;
    read.push(sheet.href);
    walk(sheet.cssRules, "");
  }
  return { rules: lines.join("\n"), seen: read };
}

// React writes an id of useId as _r_<counter>_, and Recharts names a clip
// path recharts<counter>-clip. Each counter counts the calls of the whole
// page, also the calls whose id no element shows.
const GENERATED_IDS = /_r_[0-9a-z]+_|recharts\d+(?=-clip)/g;

/**
 * normalizeMarkup replaces each id that React or Recharts generated with a
 * number in the order in which the markup first shows it, so that each
 * reference to an id still names the same element, and a story gets the same
 * ids when a component calls useId without showing the id.
 */
export function normalizeMarkup(markup: string): string {
  const names = new Map<string, string>();
  return markup.replace(GENERATED_IDS, (id) => {
    let name = names.get(id);
    if (name === undefined) {
      name = `generated-id-${names.size + 1}`;
      names.set(id, name);
    }
    return name;
  });
}

// Vite names each built asset <name>-<8 characters of hash>.<extension>.
const HASHED_URL = /(url\("[^"]*?)-[A-Za-z0-9_-]{8}(\.[a-z0-9]+"\))/g;

/**
 * normalizeStylesheet replaces the hash in the file name of each asset that a
 * url() of the stylesheet names with "[hash]", because each change of the
 * file changes the hash.
 */
export function normalizeStylesheet(rules: string): string {
  return rules.replace(HASHED_URL, "$1-[hash]$2");
}

/** SnapshotStory names one story of a snapshot in stories.json. */
export type SnapshotStory = { id: string; title: string; name: string };

/**
 * storySnapshot writes a story snapshot into dir: stories/<id>.html with the
 * normalized markup of each story, stylesheet.css with the normalized rules
 * of each stylesheet that a story loaded, and stories.json with each story
 * that it holds. add writes the files again after each story. stories.json
 * says that the snapshot is complete only after finish, so a test that stops
 * before the last story leaves a snapshot that is not complete.
 */
export function storySnapshot(dir: string) {
  const stories: SnapshotStory[] = [];
  let seen: string[] = [];
  let stylesheet = "";
  const writeIndex = (complete: boolean) =>
    writeFile(
      path.join(dir, "stories.json"),
      `${JSON.stringify({ complete, stories }, null, 2)}\n`,
    );
  return {
    async add(page: Page, story: SnapshotStory): Promise<void> {
      const markup = normalizeMarkup(
        await page.locator("body").evaluate(renderStoryMarkup),
      );
      const sheets = await page.evaluate(renderStylesheetRules, seen);
      seen = sheets.seen;
      if (sheets.rules) stylesheet += `${normalizeStylesheet(sheets.rules)}\n`;
      stories.push(story);
      await mkdir(path.join(dir, "stories"), { recursive: true });
      await writeFile(
        path.join(dir, "stories", `${story.id}.html`),
        `${markup}\n`,
      );
      await writeFile(path.join(dir, "stylesheet.css"), stylesheet);
      await writeIndex(false);
    },
    async finish(): Promise<void> {
      await mkdir(dir, { recursive: true });
      await writeIndex(true);
    },
  };
}
