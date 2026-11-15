// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import {
  normalizeMarkup,
  normalizeStylesheet,
  renderStoryMarkup,
} from "./story-snapshot";

function body(html: string): Element {
  document.body.innerHTML = html;
  return document.body;
}

describe("renderStoryMarkup", () => {
  it("puts each tag and text on its own line, indented by depth", () => {
    const text = renderStoryMarkup(
      body(
        '<div id="storybook-root"><ul class="list"><li>One</li><li>Two <b>2</b></li></ul></div>',
      ),
    );

    expect(text).toBe(
      [
        '<div id="storybook-root">',
        '  <ul class="list">',
        "    <li>One</li>",
        "    <li>",
        "      Two",
        "      <b>2</b>",
        "    </li>",
        "  </ul>",
        "</div>",
      ].join("\n"),
    );
  });

  it("keeps an element that the story added to the body, such as a portal", () => {
    const text = renderStoryMarkup(
      body(
        '<div id="storybook-root"><button>Open</button></div><div role="dialog">Hello</div>',
      ),
    );

    expect(text).toContain('<div role="dialog">Hello</div>');
  });

  it("leaves out the elements of Storybook and its addons", () => {
    const text = renderStoryMarkup(
      body(
        [
          '<div class="sb-preparing-story sb-wrapper">Loading</div>',
          '<div id="storybook-root"><p>Story</p></div>',
          '<div id="storybook-docs"><p>Docs</p></div>',
          '<div id="storybook-highlights-root"></div>',
          '<svg id="storybook-a11y-vision-filters"></svg>',
        ].join(""),
      ),
    );

    expect(text).toBe('<div id="storybook-root">\n  <p>Story</p>\n</div>');
  });

  it("leaves out scripts, styles, and text with only white space", () => {
    const text = renderStoryMarkup(
      body(
        '<div id="storybook-root">\n  <style>.a{}</style><p>A</p>\n  <script>1</script></div><script>2</script>',
      ),
    );

    expect(text).toBe('<div id="storybook-root">\n  <p>A</p>\n</div>');
  });

  it("joins the white space of a text and escapes text and attributes", () => {
    const text = renderStoryMarkup(
      body('<p title="a &quot;b&quot; &amp; c">x  &lt;y&gt;\n z</p>'),
    );

    expect(text).toBe('<p title="a &quot;b&quot; &amp; c">x &lt;y&gt; z</p>');
  });

  it("keeps the case of an SVG tag", () => {
    const text = renderStoryMarkup(
      body("<svg><linearGradient></linearGradient></svg>"),
    );

    expect(text).toContain("<linearGradient></linearGradient>");
  });
});

describe("normalizeMarkup", () => {
  it("numbers the generated ids in the order the markup shows them", () => {
    const markup = [
      '<label for="_r_5_">A</label><input id="_r_5_">',
      '<div aria-labelledby="_r_a_ _r_5_" id="_r_a_-search"></div>',
    ].join("\n");

    expect(normalizeMarkup(markup)).toBe(
      [
        '<label for="generated-id-1">A</label><input id="generated-id-1">',
        '<div aria-labelledby="generated-id-2 generated-id-1" id="generated-id-2-search"></div>',
      ].join("\n"),
    );
  });

  it("gives the same markup when the counters start at another value", () => {
    const a =
      '<clipPath id="recharts1-clip"></clipPath><g clip-path="url(#recharts1-clip)"></g><p id="_r_0_"></p>';
    const b =
      '<clipPath id="recharts7-clip"></clipPath><g clip-path="url(#recharts7-clip)"></g><p id="_r_3_"></p>';

    expect(normalizeMarkup(a)).toBe(normalizeMarkup(b));
  });

  it("keeps an id that is not generated", () => {
    const markup =
      '<div id="storybook-root"><p id="story-name">recharts_measurement_span</p></div>';

    expect(normalizeMarkup(markup)).toBe(markup);
  });
});

describe("normalizeStylesheet", () => {
  it("replaces the hash in the file name of an asset", () => {
    expect(
      normalizeStylesheet(
        '@font-face { src: url("./ibm-plex-sans-latin-400-normal-g30qAdWV.woff2") format("woff2"), url("./a-b_c-Dsrv2Tc_.woff") format("woff"); }',
      ),
    ).toBe(
      '@font-face { src: url("./ibm-plex-sans-latin-400-normal-[hash].woff2") format("woff2"), url("./a-b_c-[hash].woff") format("woff"); }',
    );
  });

  it("keeps a rule without an asset", () => {
    const rules = ".rounded-md { border-radius: calc(var(--radius) - 2px); }";

    expect(normalizeStylesheet(rules)).toBe(rules);
  });
});
