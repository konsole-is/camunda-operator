import "server-only";

/** LabelMatcher reports whether the labels of an object match a selector. */
export type LabelMatcher = (
  labels: Record<string, string> | undefined,
) => boolean;

const KEY = String.raw`[A-Za-z0-9][-A-Za-z0-9_./]*`;
const VALUE = String.raw`[-A-Za-z0-9_.]*`;
const SET = new RegExp(String.raw`^(${KEY})\s+(in|notin)\s*\(([^)]*)\)$`);
const EQUALITY = new RegExp(String.raw`^(${KEY})\s*(==|=|!=)\s*(${VALUE})$`);
const EXISTS = new RegExp(String.raw`^(!?)\s*(${KEY})$`);

/**
 * parseSelector parses a Kubernetes label selector, such as
 * "app=web,tier!=db,env in (prod,dev),!legacy". The empty selector matches
 * every object. As in Kubernetes, != and notin match an object that has no
 * such label. It throws an error that quotes the selector when a
 * requirement is not valid.
 */
export function parseSelector(selector: string): LabelMatcher {
  if (selector.trim() === "") return () => true;
  const requirements = split(selector).map((part) =>
    requirement(selector, part),
  );
  return (labels = {}) => requirements.every((matches) => matches(labels));
}

// split splits at the commas outside parentheses.
function split(selector: string): string[] {
  const parts: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < selector.length; i++) {
    if (selector[i] === "(") depth++;
    if (selector[i] === ")") depth--;
    if (selector[i] === "," && depth === 0) {
      parts.push(selector.slice(start, i));
      start = i + 1;
    }
  }
  parts.push(selector.slice(start));
  return parts.map((p) => p.trim());
}

function requirement(
  selector: string,
  part: string,
): (labels: Record<string, string>) => boolean {
  const set = SET.exec(part);
  if (set) {
    const [, key, op, list] = set;
    const values = list.split(",").map((v) => v.trim());
    return op === "in"
      ? (labels) => Object.hasOwn(labels, key) && values.includes(labels[key])
      : (labels) =>
          !Object.hasOwn(labels, key) || !values.includes(labels[key]);
  }
  const equality = EQUALITY.exec(part);
  if (equality) {
    const [, key, op, value] = equality;
    return op === "!="
      ? (labels) => labels[key] !== value
      : (labels) => Object.hasOwn(labels, key) && labels[key] === value;
  }
  const exists = EXISTS.exec(part);
  if (exists) {
    const [, not, key] = exists;
    return not
      ? (labels) => !Object.hasOwn(labels, key)
      : (labels) => Object.hasOwn(labels, key);
  }
  throw new Error(
    `The label selector "${selector}" is not valid: the fleet manager cannot read "${part}"`,
  );
}
