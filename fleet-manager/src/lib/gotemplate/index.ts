// This module runs in the browser and on the server, so it imports nothing.

/**
 * TemplateKind is the place of a template in a registration: a metric
 * query, a grant template, or a template parameter ("x-fleet-template":
 * true). Only a metric query can use {{ .pods }}.
 */
export type TemplateKind = "metric" | "grant" | "parameter";

/**
 * TemplateData holds the values of the placeholders for one request, as
 * TemplateData of pluginkit. pods is the pod regular expression of a metric
 * query.
 */
export type TemplateData = {
  namespace: string;
  claim: string;
  request: string;
  serviceAccount: string;
  parameters: Record<string, unknown>;
  outputs: Record<string, string>;
  pods?: string;
};

/** ParsedTemplate names the parameters and outputs that a template references, each once. */
export type ParsedTemplate = {
  parameters: string[];
  outputs: string[];
  usesPods: boolean;
};

/**
 * TemplateError is a template that cannot render. field is the placeholder
 * path without its dot, such as "parameters.region", when the error is about
 * one placeholder.
 */
export type TemplateError = { error: string; field?: string };

type Part = string | readonly string[];
type Parsed = ParsedTemplate & { parts: Part[] };

const KEYWORDS = new Set([
  "if",
  "else",
  "end",
  "range",
  "with",
  "template",
  "define",
  "block",
  "break",
  "continue",
]);
const LITERALS = new Set(["true", "false", "nil"]);
// A name of Go text/template is a letter or _ and then letters, digits,
// or _, all in Unicode. A field can also start with a digit outside ASCII.
const IDENTIFIER = /^[\p{L}_][\p{L}\p{Nd}_]*/u;
const SEGMENT = String.raw`\.(?:[\p{L}_]|(?![0-9])\p{Nd})[\p{L}\p{Nd}_]*`;
const FIELD = new RegExp(`^(?:${SEGMENT})+$`, "u");
const VARIABLE = /^\$\w*\s*:?=/;
const SPACE = " \t\r\n";
const ONLY_FIELDS = "a template holds field references only";

/**
 * parseTemplate checks a template of the plugin contract (CL41). A template
 * holds plain text and field references only: {{ .namespace }},
 * {{ .claim }}, {{ .request }}, {{ .serviceAccount }},
 * {{ .parameters.<name> }}, {{ .outputs.<key> }}, and in a metric query
 * also {{ .pods }}. {{- and -}} trim the white space around an action, as
 * in Go. Each other construct, an unknown name, and an action that is not
 * closed is an error that names it, with the messages of pluginkit. It does
 * not check the values of a metric query; render does.
 */
export function parseTemplate(
  template: string,
  kind: TemplateKind,
): ParsedTemplate | TemplateError {
  const parsed = parse(template, kind);
  if ("error" in parsed) return parsed;
  const { parameters, outputs, usesPods } = parsed;
  return { parameters, outputs, usesPods };
}

/**
 * render fills the placeholders of a template of kind from data. It fails
 * as parseTemplate does, and also for a parameter, an output, or .pods that
 * data does not hold. A parameter renders when it is a string, a number, or
 * a boolean; another value is an error that names it. A number can print
 * other than in pluginkit, as for 1.0, 1e3, and integers above 2^53. In a
 * metric query, a value in a "..." or '...' string is escaped for it, a
 * value outside a string must be a decimal number or a duration, and a
 * placeholder in a backtick string, in a comment, or right after a
 * backslash is an error.
 */
export function render(
  template: string,
  data: TemplateData,
  kind: TemplateKind,
): { text: string } | TemplateError {
  const parsed = parse(template, kind);
  if ("error" in parsed) return parsed;
  for (const name of parsed.parameters) {
    if (!Object.hasOwn(data.parameters, name)) {
      return {
        error: `parameter "${name}" has no value`,
        field: `parameters.${name}`,
      };
    }
    if (
      !["string", "number", "boolean"].includes(typeof data.parameters[name])
    ) {
      return {
        error: `parameter "${name}" is not a string, a number, or a boolean`,
        field: `parameters.${name}`,
      };
    }
  }
  for (const key of parsed.outputs) {
    if (!Object.hasOwn(data.outputs, key)) {
      return { error: `output "${key}" has no value`, field: `outputs.${key}` };
    }
  }
  if (parsed.usesPods && data.pods === undefined) {
    return { error: "the placeholder .pods has no value", field: "pods" };
  }
  if (kind === "metric") return renderPromQL(parsed.parts, data);
  const text = parsed.parts
    .map((part) => (typeof part === "string" ? part : value(part, data)))
    .join("");
  return { text };
}

// A value can change the structure of a PromQL query unless it is a number,
// a duration, or in a quoted string with the quote, the backslash, and the
// newline escaped. A backtick string has no escapes, and a # outside a
// string starts a comment.
const NUMBER = /^-?\d+(\.\d+)?([eE][-+]?\d+)?$/;
const DURATION = /^(\d+(ms|s|m|h|d|w|y))+$/;

function renderPromQL(
  parts: Part[],
  data: TemplateData,
): { text: string } | TemplateError {
  let quote = "";
  let escaped = false;
  let text = "";
  for (const part of parts) {
    if (typeof part !== "string") {
      const field = part.filter(Boolean).join(".");
      if (!quote && !escaped) {
        const bare = value(part, data);
        if (!NUMBER.test(bare) && !DURATION.test(bare)) {
          return {
            error: `.${field} outside a "..." or '...' string in a metric query must be a number or a duration`,
            field,
          };
        }
        text += bare;
        continue;
      }
      if ((quote !== '"' && quote !== "'") || escaped) {
        return {
          error: `.${field} in a metric query must not be in a backtick string, a comment, or an escape`,
          field,
        };
      }
      text += value(part, data)
        .replaceAll("\\", "\\\\")
        .replaceAll(quote, `\\${quote}`)
        .replaceAll("\n", "\\n");
      continue;
    }
    for (const ch of part) {
      if (!quote) {
        if (ch === '"' || ch === "'" || ch === "`" || ch === "#") quote = ch;
      } else if (quote === "#") {
        if (ch === "\n") quote = "";
      } else if (escaped) {
        escaped = false;
      } else if (ch === "\\" && quote !== "`") {
        escaped = true;
      } else if (ch === quote) {
        quote = "";
      }
    }
    text += part;
  }
  return { text };
}

function value([head, name]: readonly string[], data: TemplateData): string {
  switch (head) {
    case "parameters":
      return String(data.parameters[name]);
    case "outputs":
      return data.outputs[name];
    case "pods":
      return data.pods ?? "";
    default:
      return data[head as "namespace" | "claim" | "request" | "serviceAccount"];
  }
}

function parse(template: string, kind: TemplateKind): Parsed | TemplateError {
  const parsed: Parsed = {
    parts: [],
    parameters: [],
    outputs: [],
    usesPods: false,
  };
  let trimNext = false;
  let at = 0;
  for (;;) {
    const open = template.indexOf("{{", at);
    let text = template.slice(at, open < 0 ? undefined : open);
    if (trimNext) text = trimStart(text);
    if (open < 0) {
      parsed.parts.push(text);
      return parsed;
    }
    const trimBefore =
      template[open + 2] === "-" && SPACE.includes(template[open + 3] ?? "x");
    parsed.parts.push(trimBefore ? trimEnd(text) : text);
    const start = open + (trimBefore ? 3 : 2);
    const close = findClose(template, start);
    if (close < 0) {
      return { error: `the action at position ${open} is not closed` };
    }
    trimNext =
      template[close - 1] === "-" &&
      close - 2 >= start &&
      SPACE.includes(template[close - 2]);
    const action = template.slice(start, trimNext ? close - 1 : close).trim();
    const checked = checkAction(action, kind);
    if ("error" in checked) return checked;
    addReference(parsed, checked.path);
    parsed.parts.push(checked.path);
    at = close + 2;
  }
}

// findClose returns the index of the }} that ends the action that starts at
// start, or -1. A comment ends at */ first, and a }} inside a quoted string
// does not end the action, as in the lexer of Go.
function findClose(template: string, start: number): number {
  let i = start;
  while (i < template.length && SPACE.includes(template[i])) i++;
  if (template.startsWith("/*", i)) {
    const end = template.indexOf("*/", i + 2);
    return end < 0 ? -1 : template.indexOf("}}", end + 2);
  }
  for (; i < template.length; i++) {
    const c = template[i];
    if (template.startsWith("}}", i)) return i;
    if (c === '"' || c === "'" || c === "`") {
      i = skipQuoted(template, i);
      if (i < 0) return -1;
    }
  }
  return -1;
}

// skipQuoted returns the index of the quote that closes the literal that
// opens at start, or -1.
function skipQuoted(template: string, start: number): number {
  const quote = template[start];
  for (let i = start + 1; i < template.length; i++) {
    if (quote !== "`" && template[i] === "\\") i++;
    else if (template[i] === quote) return i;
  }
  return -1;
}

function checkAction(
  action: string,
  kind: TemplateKind,
): { path: string[] } | TemplateError {
  const shown = `{{ ${action} }}`;
  if (action.startsWith("/*")) return constructError("comment");
  const word = IDENTIFIER.exec(action)?.[0];
  if (word && KEYWORDS.has(word)) return constructError(word);
  if (VARIABLE.test(action)) {
    return {
      error: `a variable is not allowed in ${shown}: ${ONLY_FIELDS}`,
    };
  }
  if (hasPipe(action)) {
    return {
      error: `a pipeline is not allowed in ${shown}: ${ONLY_FIELDS}`,
    };
  }
  if (word && !LITERALS.has(word)) {
    return {
      error: `the function ${word} is not allowed: ${ONLY_FIELDS}`,
    };
  }
  if (!FIELD.test(action)) {
    return { error: `${shown} is not allowed: ${ONLY_FIELDS}` };
  }
  return checkField(action.slice(1).split("."), kind);
}

function constructError(name: string): TemplateError {
  return { error: `{{ ${name} }} is not allowed: ${ONLY_FIELDS}` };
}

function hasPipe(action: string): boolean {
  for (let i = 0; i < action.length; i++) {
    const c = action[i];
    if (c === "|") return true;
    if (c === '"' || c === "'" || c === "`") {
      i = skipQuoted(action, i);
      if (i < 0) return false;
    }
  }
  return false;
}

function checkField(
  path: string[],
  kind: TemplateKind,
): { path: string[] } | TemplateError {
  const name = "." + path.join(".");
  const field = path.join(".");
  const [head] = path;
  if (path.length === 1) {
    if (head === "parameters" || head === "outputs") {
      return {
        error: `the placeholder ${name} needs a name, as in ${name}.<name>`,
        field,
      };
    }
    if (head === "pods") {
      return kind === "metric"
        ? { path }
        : {
            error: "the placeholder .pods is allowed only in a metric query",
            field,
          };
    }
    if (["namespace", "claim", "request", "serviceAccount"].includes(head)) {
      return { path };
    }
  }
  if (path.length === 2 && (head === "parameters" || head === "outputs")) {
    return { path };
  }
  return { error: `unknown placeholder ${name}`, field };
}

function addReference(parsed: Parsed, [head, name]: readonly string[]): void {
  if (head === "pods") parsed.usesPods = true;
  const names =
    head === "parameters"
      ? parsed.parameters
      : head === "outputs"
        ? parsed.outputs
        : undefined;
  if (names && !names.includes(name)) names.push(name);
}

function trimStart(text: string): string {
  let i = 0;
  while (i < text.length && SPACE.includes(text[i])) i++;
  return text.slice(i);
}

function trimEnd(text: string): string {
  let i = text.length;
  while (i > 0 && SPACE.includes(text[i - 1])) i--;
  return text.slice(0, i);
}
