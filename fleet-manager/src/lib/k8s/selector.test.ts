import { describe, expect, it } from "vitest";
import { parseSelector } from "./selector";

describe("parseSelector", () => {
  it.each([
    ["", {}, true],
    ["app=web", { app: "web" }, true],
    ["app==web", { app: "db" }, false],
    ["app!=web", {}, true],
    ["app!=web", { app: "web" }, false],
    ["env in (prod, dev)", { env: "dev" }, true],
    ["env in (prod)", {}, false],
    ["env notin (prod)", {}, true],
    ["env notin (prod)", { env: "prod" }, false],
    ["camunda.io/cluster", { "camunda.io/cluster": "a" }, true],
    ["!legacy", { legacy: "yes" }, false],
    ["app=web,env in (prod,dev),!legacy", { app: "web", env: "prod" }, true],
    ["app=web,tier=db", { app: "web" }, false],
    ["constructor", {}, false],
    ["!toString", {}, true],
    ["constructor in (x)", {}, false],
  ])("%j against %j matches: %s", (selector, labels, expected) => {
    expect(parseSelector(selector)(labels)).toBe(expected);
  });

  it.each(["=web", "app in prod", "app=web,,env=prod", "a b"])(
    "refuses %j and quotes it",
    (selector) => {
      expect(() => parseSelector(selector)).toThrow(
        `The label selector "${selector}" is not valid`,
      );
    },
  );

  it("matches an empty value with key=", () => {
    expect(parseSelector("app=")({ app: "" })).toBe(true);
  });
});
