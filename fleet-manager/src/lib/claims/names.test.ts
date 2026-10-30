import { describe, expect, it } from "vitest";
import { claimNameErrors, namesIn } from "./names";

describe("claimNameErrors", () => {
  it.each(["orders", "a", "a-1", "a".repeat(40)])("accepts %s", (name) => {
    expect(claimNameErrors(name, [])).toEqual([]);
  });

  it("asks for a name when it is empty", () => {
    expect(claimNameErrors("", [])).toEqual(["Enter a name"]);
  });

  it.each(["1orders", "Orders", "orders-", "or_ders", "-orders"])(
    "refuses %s by the DNS-1035 rule",
    (name) => {
      expect(claimNameErrors(name, [])).toEqual([
        "Start with a lowercase letter. Use only lowercase letters, digits, and '-'. End with a letter or a digit.",
      ]);
    },
  );

  it("refuses 41 characters", () => {
    expect(claimNameErrors("a".repeat(41), [])).toEqual([
      "Use at most 40 characters.",
    ]);
  });

  it("gives every error of a name that breaks more than one rule", () => {
    const name = `${"a".repeat(41)}-`;
    expect(claimNameErrors(name, [name])).toEqual([
      "Start with a lowercase letter. Use only lowercase letters, digits, and '-'. End with a letter or a digit.",
      "Use at most 40 characters.",
      `A cluster named ${name} already exists in this namespace.`,
    ]);
  });

  it("refuses a name that a claim or a cluster has in the namespace", () => {
    expect(claimNameErrors("orders", ["orders"])).toEqual([
      "A cluster named orders already exists in this namespace.",
    ]);
  });
});

describe("namesIn", () => {
  it("lists the names in one namespace across lists, once each", () => {
    const o = (namespace: string, name: string) => ({
      metadata: { namespace, name },
    });
    expect(
      namesIn(
        "team-a",
        [o("team-a", "orders"), o("team-b", "billing")],
        [o("team-a", "orders"), o("team-a", "search")],
      ),
    ).toEqual(["orders", "search"]);
  });
});
