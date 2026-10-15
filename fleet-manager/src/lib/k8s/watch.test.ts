import { describe, expect, it } from "vitest";
import { parseWatchRequest } from "./watch";

const params = (query: string) => new URLSearchParams(query);

describe("parseWatchRequest", () => {
  it("reads the kinds, the namespace, the selector, and the name", () => {
    expect(
      parseWatchRequest(
        params(
          "kind=Pod&kind=Secret&namespace=team&labelSelector=app%3Dweb&name=x",
        ),
      ),
    ).toEqual({
      kinds: ["Pod", "Secret"],
      namespace: "team",
      labelSelector: "app=web",
      name: "x",
    });
  });

  it.each([
    ["", "Name at least one kind with the parameter kind"],
    ["kind=Widget", 'The fleet manager does not read the kind "Widget"'],
    [
      "kind=Environment&namespace=team",
      "Environment is cluster-scoped, so a watch of it takes no namespace",
    ],
    ["kind=Pod&labelSelector=a%20b", 'The label selector "a b" is not valid'],
  ])("refuses %j with a message that names the problem", (query, message) => {
    expect(() => parseWatchRequest(params(query))).toThrow(message);
  });
});
