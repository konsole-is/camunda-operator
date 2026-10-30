import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";
import {
  SEED_VARIABLES,
  SETTING_KEYS,
  appNamespace,
  appServiceName,
  checkPatch,
  seedFromEnv,
  settingsFromData,
} from "./keys";

const empty = Object.fromEntries(SETTING_KEYS.map((k) => [k, ""]));

describe("seed variables", () => {
  it("the seed variables are the ones the chart sets", () => {
    const deployment = readFileSync(
      fileURLToPath(
        new URL("../../../deploy/chart/deployment.yaml", import.meta.url),
      ),
      "utf8",
    );
    for (const name of Object.values(SEED_VARIABLES)) {
      expect(deployment).toContain(`- name: ${name}\n`);
    }
    expect(Object.keys(SEED_VARIABLES).sort()).toEqual(
      [...SETTING_KEYS].sort(),
    );
  });

  it("the chart sets FLEET_SERVICE_NAME to the name of the Service", () => {
    const chart = (file: string) =>
      readFileSync(
        fileURLToPath(new URL(`../../../deploy/chart/${file}`, import.meta.url)),
        "utf8",
      );
    expect(chart("deployment.yaml")).toContain(
      '- name: FLEET_SERVICE_NAME\n              value: {{ include "fleet-manager.name" . }}\n',
    );
    expect(chart("service.yaml")).toContain(
      '  name: {{ include "fleet-manager.name" . }}\n',
    );
  });

  it("a missing seed variable gives an empty setting", () => {
    expect(seedFromEnv({ FLEET_SETTINGS_GIT_BRANCH: "main" })).toEqual({
      ...empty,
      "git.branch": "main",
    });
  });
});

describe("settingsFromData", () => {
  it("fills missing keys and ignores unknown ones", () => {
    const s = settingsFromData({
      "prometheus.url": "http://prom:9090",
      extra: "x",
    });
    expect(s["prometheus.url"]).toBe("http://prom:9090");
    expect(s["git.path"]).toBe("");
    expect(s).not.toHaveProperty("extra");
  });

  it("gives empty settings for a ConfigMap without data", () => {
    expect(settingsFromData(undefined)).toEqual(empty);
  });
});

describe("credentials in a URL setting", () => {
  const withCredentials = "https://bot:t0ken@git.example.com/fleet.git";

  it("the seed drops the user name and the password of an address", () => {
    const seed = seedFromEnv({
      FLEET_SETTINGS_GIT_REPOSITORY: withCredentials,
      FLEET_SETTINGS_PROMETHEUS_URL: "http://admin:pw@prom:9090/",
    });

    expect(seed["git.repository"]).toBe("https://git.example.com/fleet.git");
    expect(seed["prometheus.url"]).toBe("http://prom:9090/");
  });

  it("settings read from the ConfigMap drop them too", () => {
    const s = settingsFromData({ "git.repository": withCredentials });

    expect(s["git.repository"]).toBe("https://git.example.com/fleet.git");
    expect(JSON.stringify(s)).not.toContain("t0ken");
  });

  it("keeps an http or https address without credentials", () => {
    expect(
      settingsFromData({
        "git.repository": "https://git.example.com/fleet.git",
        "prometheus.url": "http://prom:9090",
      }),
    ).toMatchObject({
      "git.repository": "https://git.example.com/fleet.git",
      "prometheus.url": "http://prom:9090",
    });
  });

  it.each([
    "https://bot:t0ken@git.example.com:bad/fleet.git",
    "bot:t0ken@git.example.com/fleet.git",
    "ssh://bot:t0ken@git.example.com/fleet.git",
  ])("an address that is not http or https is not set (%s)", (value) => {
    expect(
      settingsFromData({ "git.repository": value })["git.repository"],
    ).toBe("");
    expect(
      seedFromEnv({ FLEET_SETTINGS_GIT_REPOSITORY: value })["git.repository"],
    ).toBe("");
  });
});

describe("checkPatch", () => {
  it("refuses an unknown key", () => {
    expect(checkPatch({ "git.token": "x" })).toEqual([
      expect.objectContaining({ code: "Invalid", path: "git.token" }),
    ]);
  });

  it("refuses a value that is not a string", () => {
    expect(checkPatch({ "git.branch": 3 })).toEqual([
      expect.objectContaining({ code: "Invalid", path: "git.branch" }),
    ]);
  });

  it("refuses a store kind that ExternalSecret does not know", () => {
    expect(checkPatch({ "secretStore.kind": "Vault" })).toEqual([
      expect.objectContaining({ code: "Invalid", path: "secretStore.kind" }),
    ]);
  });

  it("accepts both store kinds", () => {
    expect(checkPatch({ "secretStore.kind": "SecretStore" })).toEqual([]);
    expect(checkPatch({ "secretStore.kind": "ClusterSecretStore" })).toEqual(
      [],
    );
  });

  it.each(["git.repository", "prometheus.url"])(
    "refuses a %s that holds credentials",
    (key) => {
      const errors = checkPatch({
        [key]: "https://me:t0ken@git.example.com/fleet.git",
      });
      expect(errors).toEqual([
        expect.objectContaining({ code: "Invalid", path: key }),
      ]);
      expect(JSON.stringify(errors)).not.toContain("t0ken");
    },
  );

  it("refuses an address that is not http or https", () => {
    expect(
      checkPatch({ "git.repository": "ssh://git.example.com/fleet.git" }),
    ).toEqual([
      expect.objectContaining({ code: "Invalid", path: "git.repository" }),
    ]);
  });

  it("refuses a URL that does not parse", () => {
    expect(checkPatch({ "prometheus.url": "not a url" })).toEqual([
      expect.objectContaining({ code: "Invalid", path: "prometheus.url" }),
    ]);
  });

  it("accepts empty values, which unset a setting", () => {
    expect(
      checkPatch({ "git.repository": "", "secretStore.kind": "" }),
    ).toEqual([]);
  });
});

describe("appNamespace", () => {
  const saved = process.env.FLEET_NAMESPACE;
  afterEach(() => {
    if (saved === undefined) delete process.env.FLEET_NAMESPACE;
    else process.env.FLEET_NAMESPACE = saved;
  });

  it("returns the namespace of the pod", () => {
    process.env.FLEET_NAMESPACE = "camunda-system";
    expect(appNamespace()).toBe("camunda-system");
  });

  it("names the variable when it is missing", () => {
    delete process.env.FLEET_NAMESPACE;
    expect(() => appNamespace()).toThrow(/FLEET_NAMESPACE/);
  });
});

describe("appServiceName", () => {
  const saved = process.env.FLEET_SERVICE_NAME;
  afterEach(() => {
    if (saved === undefined) delete process.env.FLEET_SERVICE_NAME;
    else process.env.FLEET_SERVICE_NAME = saved;
  });

  it("returns FLEET_SERVICE_NAME", () => {
    process.env.FLEET_SERVICE_NAME = "camunda-operator-fleet-manager";
    expect(appServiceName()).toBe("camunda-operator-fleet-manager");
  });

  it.each([undefined, ""])(
    "is undefined when FLEET_SERVICE_NAME is %j",
    (value) => {
      if (value === undefined) delete process.env.FLEET_SERVICE_NAME;
      else process.env.FLEET_SERVICE_NAME = value;
      expect(appServiceName()).toBeUndefined();
    },
  );
});
