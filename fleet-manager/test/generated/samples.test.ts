import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { parse } from "yaml";
import { describe, expect, it } from "vitest";
import { typeCheck } from "./typecheck";

const appDir = fileURLToPath(new URL("../..", import.meta.url));
const samplesDir = path.join(appDir, "..", "config", "samples");
const crdsDir = path.join(appDir, "src", "generated", "crds");

interface Sample {
  file: string;
  object: { kind: string; spec?: Record<string, unknown> };
}

const samples: Sample[] = fs
  .readdirSync(samplesDir)
  .filter((name) => name.endsWith(".yaml") && name !== "kustomization.yaml")
  .sort()
  .map((file) => ({
    file,
    object: parse(fs.readFileSync(path.join(samplesDir, file), "utf8")),
  }));

// module returns a module that assigns object to a const of the generated
// type of its kind.
function module(object: Sample["object"]): string {
  const kind = object.kind;
  return `import type { ${kind} } from "../../../src/generated/crds/${kind.toLowerCase()}";\nexport const sample: ${kind} = ${JSON.stringify(object)};\n`;
}

describe("the samples of config/samples", { timeout: 120_000 }, () => {
  it("cover each generated kind, and each sample has a generated kind", () => {
    const kinds = fs
      .readdirSync(crdsDir)
      .map((name) => name.replace(/\.ts$/, ""))
      .sort();
    const sampled = [
      ...new Set(samples.map((s) => s.object.kind.toLowerCase())),
    ].sort();

    expect(sampled).toEqual(kinds);
  });

  it("type-check against the generated type of their kind", () => {
    const sources = Object.fromEntries(
      samples.map((s) => [
        `test/generated/__virtual__/${s.file}.ts`,
        module(s.object),
      ]),
    );

    const failures = Object.entries(typeCheck(sources)).filter(
      ([, messages]) => messages.length > 0,
    );
    expect(failures).toEqual([]);
  });

  it("fail the check with a field that the CRD does not have", () => {
    const cluster = samples.find((s) => s.object.kind === "CamundaCluster");
    expect(cluster).toBeDefined();
    const broken = {
      ...cluster!.object,
      spec: { ...cluster!.object.spec, notAField: 1 },
    };

    const result = typeCheck({
      "test/generated/__virtual__/broken.ts": module(broken),
    });
    expect(result["test/generated/__virtual__/broken.ts"]).not.toEqual([]);
  });
});
