import { describe, expect, it } from "vitest";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import { parseImagesTable } from "@/startup/bundled-data";
import {
  KEYCLOAK_REALM_WARNING,
  KEYCLOAK_RESTART_WARNING,
  type Running,
  defaultRelease,
  prefillNote,
  versionChangeWarnings,
  versionError,
  versionErrors,
  versionMatrixWarning,
  versionPrefill,
  versionWarnings,
  versionsOf,
} from "./versions";

const table = parseImagesTable({
  schemaVersion: 1,
  minors: {
    "8.9": {
      camunda: { repository: "camunda/camunda", version: "8.9.23" },
      identity: { repository: "camunda/identity", version: "8.9.10" },
      console: { repository: "camunda/console", version: "8.9.122" },
      "web-modeler-restapi": {
        repository: "camunda/web-modeler-restapi",
        version: "8.9.11",
      },
      "web-modeler-websockets": {
        repository: "camunda/web-modeler-websockets",
        version: "8.9.9",
      },
      keycloak: {
        repository: "camunda/keycloak",
        tagPrefix: "quay-optimized-",
        version: "26.6.4",
      },
    },
  },
});
const release = (name: string, version: string) =>
  ({ metadata: { name }, spec: { version } }) as CamundaRelease;
const environment = (name: string, defaultRelease?: string) =>
  ({
    metadata: { name },
    spec: { ...(defaultRelease && { releases: { default: defaultRelease } }) },
  }) as Environment;
const all: Running = { keycloak: true, console: true, webModeler: true };
const none: Running = { keycloak: false, console: false, webModeler: false };

describe("versionPrefill", () => {
  it("takes the image versions of the minor of the default release, never the release version", () => {
    const prefill = versionPrefill(table, release("camunda-8-9-23", "8.9.23"));
    expect(prefill).toEqual({
      release: "camunda-8-9-23",
      minor: "8.9",
      versions: {
        identity: "8.9.10",
        console: "8.9.122",
        webModeler: "8.9.11",
        keycloak: "26.6.4",
      },
    });
    expect(Object.values(prefill!.versions)).not.toContain("8.9.23");
  });

  it("gives Web Modeler the version of web-modeler-restapi", () => {
    expect(
      versionPrefill(table, release("r", "8.9.1"))?.versions.webModeler,
    ).toBe("8.9.11");
  });

  it("gives Keycloak the bare version, without the tag prefix", () => {
    expect(
      versionPrefill(table, release("r", "8.9.1"))?.versions.keycloak,
    ).toBe("26.6.4");
  });

  it("is undefined for a minor that the table does not hold, and without a release", () => {
    expect(versionPrefill(table, release("r", "8.10.1"))).toBeUndefined();
    expect(versionPrefill(table, undefined)).toBeUndefined();
  });
});

describe("defaultRelease", () => {
  const releases = [release("a-rel", "8.9.1"), release("b-rel", "8.9.2")];
  const environments = [environment("a", "a-rel"), environment("b", "b-rel")];

  it("is the default release of the first served environment", () => {
    expect(defaultRelease(["b", "a"], environments, releases)).toBe(
      releases[1],
    );
  });

  it("is undefined when that environment has no default or the release does not exist", () => {
    expect(defaultRelease(["c"], [environment("c")], releases)).toBeUndefined();
    expect(
      defaultRelease(["d"], [environment("d", "gone")], releases),
    ).toBeUndefined();
    expect(defaultRelease([], environments, releases)).toBeUndefined();
  });
});

describe("versionsOf", () => {
  const prefill = versionPrefill(table, release("r", "8.9.23"));

  it("keeps a typed version and fills the others from the prefill", () => {
    expect(versionsOf({ console: "8.9.200" }, prefill, all)).toEqual({
      identity: "8.9.10",
      console: "8.9.200",
      webModeler: "8.9.11",
      keycloak: "26.6.4",
    });
  });

  it("keeps a version that was typed empty", () => {
    expect(versionsOf({ identity: "" }, prefill, none)).toEqual({
      identity: "",
    });
  });

  it("holds only the versions of running apps: no keycloak outside Keycloak mode, no console or webModeler when off", () => {
    expect(
      versionsOf({ keycloak: "26.1.0", console: "8.9.1" }, prefill, none),
    ).toEqual({ identity: "8.9.10" });
  });

  it("leaves an untyped version unset without a prefill", () => {
    expect(versionsOf({ console: "8.9.1" }, undefined, all)).toEqual({
      console: "8.9.1",
    });
  });
});

describe("versionErrors", () => {
  const valid = {
    identity: "8.9.10",
    console: "8.9.122",
    webModeler: "8.9.11",
    keycloak: "26.6.4",
  };

  it.each([
    ["identity", "8.8.9", "Management Identity needs version 8.9.0 or later."],
    ["console", "8.8.0", "Console needs version 8.9.0 or later."],
    ["webModeler", "7.0.0", "Web Modeler needs version 8.9.0 or later."],
    [
      "keycloak",
      "25.0.6",
      "Keycloak needs a version from 26.0.0 up to, but not including, 27.0.0.",
    ],
    [
      "keycloak",
      "27.0.0",
      "Keycloak needs a version from 26.0.0 up to, but not including, 27.0.0.",
    ],
    [
      "identity",
      "8.9",
      "Enter the Management Identity version as major.minor.patch, for example 8.9.12.",
    ],
    [
      "keycloak",
      "26.6",
      "Enter the Keycloak version as major.minor.patch, for example 26.6.4.",
    ],
    ["console", "", "Enter the Console version."],
  ])("refuses %s %s", (field, value, message) => {
    expect(versionErrors({ ...valid, [field]: value }, all)).toEqual([message]);
  });

  it("asks for a version that is missing", () => {
    expect(versionErrors({ identity: "8.9.10" }, all)).toEqual([
      "Enter the Console version.",
      "Enter the Web Modeler version.",
      "Enter the Keycloak version.",
    ]);
  });

  it("passes 8.9.0, 8.10.2, 26.0.0, and 26.99.0", () => {
    expect(
      versionErrors(
        {
          identity: "8.9.0",
          console: "8.10.2",
          webModeler: "8.9.0",
          keycloak: "26.0.0",
        },
        all,
      ),
    ).toEqual([]);
    expect(versionErrors({ ...valid, keycloak: "26.99.0" }, all)).toEqual([]);
  });

  it("checks no version of an app that does not run", () => {
    expect(
      versionErrors(
        { identity: "8.9.10", console: "1.0.0", keycloak: "1.0.0" },
        none,
      ),
    ).toEqual([]);
  });
});

describe("versionError", () => {
  it("gives the error of versionErrors for one field", () => {
    expect(versionError("keycloak", "25.0.1")).toBe(
      "Keycloak needs a version from 26.0.0 up to, but not including, 27.0.0.",
    );
    expect(versionError("console", "")).toBe("Enter the Console version.");
    expect(versionError("console", undefined)).toBe(
      "Enter the Console version.",
    );
    expect(versionError("identity", "8.9.14")).toBeUndefined();
  });
});

describe("versionChangeWarnings", () => {
  it("warns when a Camunda app moves down", () => {
    expect(
      versionChangeWarnings(
        { identity: "8.9.14" },
        { identity: "8.9.10" },
        all,
      ),
    ).toEqual([
      "Management Identity moves down from 8.9.14 to 8.9.10. Camunda documents no downgrade.",
    ]);
  });

  it("warns when a Camunda app skips a minor", () => {
    expect(
      versionChangeWarnings({ console: "8.9.133" }, { console: "8.11.2" }, all),
    ).toEqual([
      "Console skips 8.10: 8.9.133 to 8.11.2. Move one minor at a time.",
    ]);
  });

  it("counts a move to a higher major as a skipped minor", () => {
    expect(
      versionChangeWarnings(
        { webModeler: "8.9.15" },
        { webModeler: "9.0.0" },
        all,
      ),
    ).toEqual([
      "Web Modeler skips 8.10: 8.9.15 to 9.0.0. Move one minor at a time.",
    ]);
  });

  it("gives no warning for a move of one minor up", () => {
    expect(
      versionChangeWarnings(
        { identity: "8.9.14" },
        { identity: "8.10.0" },
        all,
      ),
    ).toEqual([]);
  });

  it("warns that sign-in stops when the Keycloak version changes", () => {
    expect(
      versionChangeWarnings(
        { keycloak: "26.6.4" },
        { keycloak: "26.6.5" },
        all,
      ),
    ).toEqual([KEYCLOAK_RESTART_WARNING]);
  });

  it("gives no downgrade or skip warning for Keycloak", () => {
    expect(
      versionChangeWarnings(
        { keycloak: "26.6.4" },
        { keycloak: "26.0.1" },
        all,
      ),
    ).toEqual([KEYCLOAK_RESTART_WARNING]);
  });

  it("gives no warning for an app that does not run, or with no old version", () => {
    expect(
      versionChangeWarnings(
        { keycloak: "26.6.4" },
        { keycloak: "26.6.5" },
        none,
      ),
    ).toEqual([]);
    expect(versionChangeWarnings({}, { console: "8.9.133" }, all)).toEqual([]);
  });

  it("gives no warning for a patch upgrade or a malformed version", () => {
    expect(
      versionChangeWarnings(
        { identity: "8.9.10" },
        { identity: "8.9.14" },
        all,
      ),
    ).toEqual([]);
    expect(
      versionChangeWarnings({ identity: "8.9.10" }, { identity: "8.9" }, all),
    ).toEqual([]);
  });
});

describe("versionMatrixWarning", () => {
  const camunda: Running = { keycloak: false, console: true, webModeler: true };

  it("passes the versions of one row of the images table", () => {
    expect(
      versionMatrixWarning(
        { identity: "8.9.14", console: "8.9.133", webModeler: "8.9.15" },
        camunda,
      ),
    ).toBeUndefined();
  });

  it("warns when the running Camunda apps are on different minors", () => {
    expect(
      versionMatrixWarning(
        { identity: "8.10.1", console: "8.9.133", webModeler: "8.9.15" },
        camunda,
      ),
    ).toBe(
      "Management Identity, Console, and Web Modeler run on different minors: 8.10, 8.9. Camunda supports them together on one minor only.",
    );
  });

  it("names two apps with 'and'", () => {
    expect(
      versionMatrixWarning(
        { identity: "8.10.1", console: "8.9.133" },
        { keycloak: true, console: true, webModeler: false },
      ),
    ).toBe(
      "Management Identity and Console run on different minors: 8.10, 8.9. Camunda supports them together on one minor only.",
    );
  });

  it("leaves out an app that does not run", () => {
    expect(
      versionMatrixWarning({ identity: "8.10.1", console: "8.9.133" }, none),
    ).toBeUndefined();
  });

  it("leaves out an app whose version does not parse", () => {
    expect(
      versionMatrixWarning(
        { identity: "8.10.1", console: "8.9", webModeler: "" },
        camunda,
      ),
    ).toBeUndefined();
  });
});

describe("versionWarnings", () => {
  it.each([
    ["8.9.10", "26.7.0", [KEYCLOAK_REALM_WARNING]],
    ["8.9.10", "26.8.1", [KEYCLOAK_REALM_WARNING]],
    ["8.9.10", "26.6.4", []],
    ["8.10.1", "26.7.0", []],
  ])("Identity %s with Keycloak %s", (identity, keycloak, warnings) => {
    expect(versionWarnings({ identity, keycloak }, all)).toEqual(warnings);
  });

  it("warns about nothing outside Keycloak mode", () => {
    expect(
      versionWarnings({ identity: "8.9.10", keycloak: "26.7.0" }, none),
    ).toEqual([]);
  });
});

describe("prefillNote", () => {
  it("reads 'Tested with release <minor>'", () => {
    expect(
      prefillNote(
        versionPrefill(table, release("r", "8.9.23")),
        release("r", "8.9.23"),
      ),
    ).toBe("Tested with release 8.9");
  });

  it("says that the table has no versions for the minor", () => {
    expect(prefillNote(undefined, release("r", "8.10.1"))).toBe(
      "The images table has no versions for Camunda 8.10. Enter each version.",
    );
  });

  it("says that the environment has no default release", () => {
    expect(prefillNote(undefined, undefined)).toBe(
      "The first served environment has no default release. Enter each version.",
    );
  });
});
