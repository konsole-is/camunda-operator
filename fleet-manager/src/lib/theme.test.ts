// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { THEME_COOKIE, applyTheme, themeCookie, themeOf } from "./theme";

function cookieFields(text: string) {
  const [pair, ...attributes] = text.split("; ");
  const [name, value] = pair.split("=");
  return {
    name,
    value,
    attributes: Object.fromEntries(attributes.map((a) => a.split("="))),
  };
}

describe("themeOf", () => {
  it.each(["light", "dark"] as const)("reads %s", (value) => {
    expect(themeOf(value)).toBe(value);
  });

  it.each([undefined, "", "system", "Dark", "blue"])(
    "reads %j as system",
    (value) => {
      expect(themeOf(value)).toBe("system");
    },
  );
});

describe("applyTheme", () => {
  it.each(["light", "dark"] as const)(
    "gives the root the class %s and drops the other",
    (choice) => {
      const root = document.createElement("html");
      root.className = "light dark keep";
      applyTheme(choice, root);
      expect([...root.classList].sort()).toEqual([choice, "keep"].sort());
    },
  );

  it("leaves no theme class for system", () => {
    const root = document.createElement("html");
    root.className = "dark keep";
    applyTheme("system", root);
    expect([...root.classList]).toEqual(["keep"]);
  });

  it("gives the same result when called twice", () => {
    const root = document.createElement("html");
    applyTheme("light", root);
    applyTheme("light", root);
    expect([...root.classList]).toEqual(["light"]);
  });
});

describe("themeCookie", () => {
  it.each(["light", "dark"] as const)(
    "keeps %s for a year on every path",
    (choice) => {
      const cookie = cookieFields(themeCookie(choice));
      expect(cookie.name).toBe(THEME_COOKIE);
      expect(themeOf(cookie.value)).toBe(choice);
      expect(cookie.attributes).toMatchObject({
        Path: "/",
        "Max-Age": String(365 * 24 * 60 * 60),
      });
    },
  );

  it("removes the cookie for system", () => {
    const cookie = cookieFields(themeCookie("system"));
    expect(cookie.name).toBe(THEME_COOKIE);
    expect(cookie.attributes).toMatchObject({ Path: "/", "Max-Age": "0" });
  });

  it("is a cookie that document.cookie stores and removes", () => {
    document.cookie = themeCookie("dark");
    expect(document.cookie).toContain(`${THEME_COOKIE}=dark`);
    document.cookie = themeCookie("system");
    expect(document.cookie).not.toContain(THEME_COOKIE);
  });
});
