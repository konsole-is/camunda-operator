// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import {
  normalizeMarkup,
  normalizeStylesheet,
  renderStoryMarkup,
  renderStylesheetRules,
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
        "      Two ",
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

  it("leaves out scripts and styles", () => {
    const text = renderStoryMarkup(
      body(
        '<div id="storybook-root"><style>.a{}</style><p>A</p><script>1</script></div><script>2</script>',
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

  it("keeps the space between a text and an inline element", () => {
    const spaced = renderStoryMarkup(body("<p>Hello <b>world</b></p>"));
    const joined = renderStoryMarkup(body("<p>Hello<b>world</b></p>"));

    expect(spaced).not.toBe(joined);
  });

  it("keeps a space between two inline elements", () => {
    const spaced = renderStoryMarkup(body("<p><b>a</b> <b>b</b></p>"));
    const joined = renderStoryMarkup(body("<p><b>a</b><b>b</b></p>"));

    expect(spaced).not.toBe(joined);
  });

  it("keeps each line and its indent in preformatted text", () => {
    const text = renderStoryMarkup(
      body('<pre>{\n  "a": 1\n}</pre><pre><code>x\n    y</code></pre>'),
    );

    expect(text).toContain('<pre>\n  {\n    "a": 1\n  }\n</pre>');
    expect(text).toContain("<code>\n    x\n        y\n  </code>");
    expect(renderStoryMarkup(body("<pre>a\n  b</pre>"))).not.toBe(
      renderStoryMarkup(body("<pre>a\n    b</pre>")),
    );
  });

  it("keeps the case of an SVG tag", () => {
    const text = renderStoryMarkup(
      body("<svg><linearGradient></linearGradient></svg>"),
    );

    expect(text).toContain("<linearGradient></linearGradient>");
  });
});

describe("renderStylesheetRules", () => {
  // jsdom gives a style element no URL, so the test gives it one, as a
  // stylesheet that the page loads from a file has.
  function loadedStylesheet(css: string): string {
    document.head.innerHTML = `<style>${css}</style>`;
    const href = "http://127.0.0.1/assets/iframe.css";
    Object.defineProperty(document.styleSheets[0], "href", { value: href });
    return href;
  }

  it("keeps the spaces inside a quoted value", () => {
    loadedStylesheet('.a::before {  content: "A  B"; }');

    expect(renderStylesheetRules([]).rules).toBe(
      '.a::before { content: "A  B"; }',
    );
  });

  it("skips a stylesheet that it read before", () => {
    const href = loadedStylesheet(".a { color: red; }");

    expect(renderStylesheetRules([href])).toEqual({ rules: "", seen: [href] });
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
