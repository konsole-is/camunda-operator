import { describe, expect, it } from "vitest";
import {
  currentReadErrors,
  unknownUsersText,
  unreadChangeText,
  unreadDeleteText,
  unreadKinds,
} from "./unread";

describe("unreadKinds", () => {
  it("names each failed and each stale kind once, failed kinds first", () => {
    expect(
      unreadKinds({
        errors: [{ kind: "PluginRequest", message: "forbidden" }],
        stale: ["PluginSection", "PluginRequest"],
      }),
    ).toEqual(["PluginRequest", "PluginSection"]);
  });

  it("names no kind when every list is read", () => {
    expect(unreadKinds({})).toEqual([]);
    expect(unreadKinds({ errors: [], stale: [] })).toEqual([]);
  });
});

describe("currentReadErrors", () => {
  const forbidden = (kind: string) => ({
    kind,
    message: `${kind} is forbidden`,
  });

  it("keeps a failed server read while the watch of its kind has not answered", () => {
    expect(
      currentReadErrors([], [forbidden("Environment")], new Set()),
    ).toEqual([forbidden("Environment")]);
  });

  it("drops a failed server read once the watch of its kind answered", () => {
    expect(
      currentReadErrors(
        [],
        [forbidden("Environment")],
        new Set(["Environment"]),
      ),
    ).toEqual([]);
  });

  it("shows the live error of a kind in place of its server error", () => {
    const live = { kind: "Environment", message: "The watch failed" };
    expect(
      currentReadErrors([live], [forbidden("Environment")], new Set()),
    ).toEqual([live]);
  });

  it("puts the live errors first, also of kinds that answered", () => {
    const live = { kind: "Namespace", message: "The watch failed" };
    expect(
      currentReadErrors(
        [live],
        [forbidden("Environment")],
        new Set(["Namespace"]),
      ),
    ).toEqual([live, forbidden("Environment")]);
  });

  it("takes no server read as a read without errors", () => {
    expect(currentReadErrors([], undefined, new Set())).toEqual([]);
  });
});

describe("the texts of kinds that are not read", () => {
  const kinds = ["CamundaClusterClaim", "Environment"];

  it("unreadDeleteText names each kind", () => {
    expect(unreadDeleteText(kinds)).toBe(
      "Delete is possible when the fleet manager can read CamundaClusterClaim, Environment.",
    );
  });

  it("unreadChangeText names each kind", () => {
    expect(unreadChangeText(kinds)).toBe(
      "A change is possible when the fleet manager can read CamundaClusterClaim, Environment.",
    );
  });

  it("unknownUsersText names each kind", () => {
    expect(unknownUsersText(kinds)).toBe(
      "The users are not known while the fleet manager cannot read CamundaClusterClaim, Environment.",
    );
  });
});
