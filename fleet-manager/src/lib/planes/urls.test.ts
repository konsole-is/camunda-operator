import { describe, expect, it } from "vitest";
import { accessUrls, planeUrlFields, urlErrors } from "./urls";
import type { Running } from "./versions";

const all: Running = { keycloak: true, console: true, webModeler: true };
const none: Running = { keycloak: false, console: false, webModeler: false };
const base = "https://management.example.com";

describe("planeUrlFields", () => {
  it("gives the five paths with every app running, from SLOT_FIELDS", () => {
    expect(
      planeUrlFields(all).map((f) => [f.id, f.names.join("."), f.suffix]),
    ).toEqual([
      ["keycloak", "identityProvider.keycloak.externalUrl", "/auth"],
      ["identity", "identity.externalUrl", "/identity"],
      ["console", "console.externalUrl", "/console"],
      ["webModeler", "webModeler.externalUrl", "/modeler"],
      [
        "webModelerWebsockets",
        "webModeler.websocketsExternalUrl",
        "/modeler-ws",
      ],
    ]);
  });

  it("gives no Keycloak URL outside Keycloak mode and no Console or Web Modeler URLs when off", () => {
    expect(planeUrlFields(none).map((f) => f.id)).toEqual(["identity"]);
  });

  it("labels each field and shows the fixed path in the placeholder", () => {
    expect(planeUrlFields(all).map((f) => [f.label, f.placeholder])).toEqual([
      ["Keycloak", `${base}/auth`],
      ["Management Identity", `${base}/identity`],
      ["Console", `${base}/console`],
      ["Web Modeler", `${base}/modeler`],
      ["Web Modeler websockets", `${base}/modeler-ws`],
    ]);
  });
});

describe("accessUrls", () => {
  it("access URLs are https://<host> plus the fixed path", () => {
    expect(accessUrls("plane-a.example.com", planeUrlFields(all))).toEqual({
      keycloak: "https://plane-a.example.com/auth",
      identity: "https://plane-a.example.com/identity",
      console: "https://plane-a.example.com/console",
      webModeler: "https://plane-a.example.com/modeler",
      webModelerWebsockets: "https://plane-a.example.com/modeler-ws",
    });
  });
});

describe("urlErrors", () => {
  const valid = {
    keycloak: `${base}/auth`,
    identity: `${base}/identity`,
    console: `${base}/console`,
    webModeler: `${base}/modeler`,
    webModelerWebsockets: `${base}/modeler-ws`,
  };

  it("passes a URL for each running app", () => {
    expect(urlErrors(valid, planeUrlFields(all))).toEqual([]);
  });

  it.each([
    ["keycloak", "", "Enter the Keycloak URL."],
    ["console", "", "Enter the Console URL."],
    [
      "identity",
      "ftp://management.example.com/identity",
      "The Management Identity URL must be an http or https URL.",
    ],
    [
      "console",
      "management.example.com/console",
      "The Console URL must be an http or https URL.",
    ],
    [
      "keycloak",
      `${base}/keycloak`,
      "The Keycloak URL must end in /auth, for example https://management.example.com/auth.",
    ],
    [
      "keycloak",
      "https:management.example.com/auth",
      "The Keycloak URL must be an http or https URL.",
    ],
    [
      "console",
      "https:/management.example.com/console",
      "The Console URL must be an http or https URL.",
    ],
    [
      "keycloak",
      `${base}/a/../auth`,
      "The Keycloak URL must end in /auth, for example https://management.example.com/auth.",
    ],
    [
      "keycloak",
      `${base}/auth?x=1`,
      "The Keycloak URL must carry no query and no fragment.",
    ],
    [
      "identity",
      "http://management.example.com/identity",
      "The Management Identity URL needs https when it has a path.",
    ],
    [
      "console",
      `${base}/con%20sole`,
      "The Console URL must carry no percent-encoded character.",
    ],
    [
      "webModeler",
      `${base}\\modeler`,
      "The Web Modeler URL must carry no backslash.",
    ],
    [
      "webModelerWebsockets",
      `${base}/a/../modeler-ws`,
      "The Web Modeler websockets URL must carry no . or .. path segment.",
    ],
  ])("refuses %s %s", (id, value, message) => {
    expect(urlErrors({ ...valid, [id]: value }, planeUrlFields(all))).toEqual([
      message,
    ]);
  });

  it("an Identity URL over http without a path passes", () => {
    expect(
      urlErrors(
        { identity: "http://identity.example.com" },
        planeUrlFields(none),
      ),
    ).toEqual([]);
  });

  it("checks only the fields it gets", () => {
    expect(
      urlErrors({ identity: `${base}/identity` }, planeUrlFields(none)),
    ).toEqual([]);
  });
});
