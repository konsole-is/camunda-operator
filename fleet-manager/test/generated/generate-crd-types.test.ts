import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import type { JSONSchema } from "json-schema-to-typescript";
import { stringify } from "yaml";
import { describe, expect, it } from "vitest";
import {
  renderCrdTypes,
  writeCrdTypes,
  type Crd,
} from "../../scripts/generate-crd-types.mjs";
import { typeCheck } from "./typecheck";

const dir = "test/generated/__virtual__";

function widgetCrd(
  properties: Record<string, JSONSchema>,
  versions: Crd["spec"]["versions"] = [],
): Crd {
  return {
    metadata: { name: "widgets.example.camunda.io" },
    spec: {
      group: "example.camunda.io",
      names: { kind: "Widget" },
      versions: versions.length
        ? versions
        : [
            {
              name: "v1",
              served: true,
              schema: { openAPIV3Schema: { type: "object", properties } },
            },
          ],
    },
  };
}

const spec: JSONSchema = {
  type: "object",
  properties: { size: { type: "integer" } },
};

// probe returns a module that assigns value to a const of type, imported
// from the generated widget module.
function probe(type: string, value: unknown): string {
  return `import type { ${type} } from "./widget";\nexport const probe: ${type} = ${JSON.stringify(value)};\n`;
}

const widget = {
  apiVersion: "example.camunda.io/v1",
  kind: "Widget",
  metadata: { name: "a" },
  spec: { size: 3 },
};

describe("renderCrdTypes", { timeout: 60_000 }, () => {
  it("types the spec, the status, and the resource with apiVersion, kind, and metadata", async () => {
    const file = await renderCrdTypes(
      widgetCrd({
        spec,
        status: { type: "object", properties: { phase: { type: "string" } } },
      }),
      "widgets.yaml",
    );

    expect(file.name).toBe("widget.ts");
    const result = typeCheck({
      [`${dir}/widget.ts`]: file.source,
      [`${dir}/valid.ts`]: probe("Widget", {
        ...widget,
        status: { phase: "Up" },
      }),
      [`${dir}/spec.ts`]: probe("WidgetSpec", { size: 1 }),
      [`${dir}/status.ts`]: probe("WidgetStatus", { phase: "Up" }),
      [`${dir}/unknown-field.ts`]: probe("Widget", {
        ...widget,
        spec: { size: 3, colour: "red" },
      }),
      [`${dir}/wrong-kind.ts`]: probe("Widget", { ...widget, kind: "Gadget" }),
      [`${dir}/wrong-version.ts`]: probe("Widget", {
        ...widget,
        apiVersion: "example.camunda.io/v2",
      }),
    });

    expect(result[`${dir}/widget.ts`]).toEqual([]);
    expect(result[`${dir}/valid.ts`]).toEqual([]);
    expect(result[`${dir}/spec.ts`]).toEqual([]);
    expect(result[`${dir}/status.ts`]).toEqual([]);
    expect(result[`${dir}/unknown-field.ts`]).not.toEqual([]);
    expect(result[`${dir}/wrong-kind.ts`]).not.toEqual([]);
    expect(result[`${dir}/wrong-version.ts`]).not.toEqual([]);
  });

  it("shows a field that the CRD adds", async () => {
    const before = await renderCrdTypes(widgetCrd({ spec }), "widgets.yaml");
    const after = await renderCrdTypes(
      widgetCrd({
        spec: {
          ...spec,
          properties: { ...spec.properties, colour: { type: "string" } },
        },
      }),
      "widgets.yaml",
    );

    expect(after.source).not.toBe(before.source);
    const result = typeCheck({
      [`${dir}/widget.ts`]: after.source,
      [`${dir}/colour.ts`]: probe("Widget", {
        ...widget,
        spec: { colour: "red" },
      }),
    });
    expect(result[`${dir}/colour.ts`]).toEqual([]);
  });

  it("exports no status type for a kind without a status", async () => {
    const file = await renderCrdTypes(widgetCrd({ spec }), "widgets.yaml");

    const result = typeCheck({
      [`${dir}/widget.ts`]: file.source,
      [`${dir}/status.ts`]: `import type { WidgetStatus } from "./widget";\nexport type S = WidgetStatus;\n`,
    });
    expect(result[`${dir}/status.ts`]).not.toEqual([]);
  });

  it("lets a field with x-kubernetes-preserve-unknown-fields take any key", async () => {
    const file = await renderCrdTypes(
      widgetCrd({
        spec: {
          type: "object",
          properties: {
            values: {
              type: "object",
              "x-kubernetes-preserve-unknown-fields": true,
            },
          },
        },
      }),
      "widgets.yaml",
    );

    const result = typeCheck({
      [`${dir}/widget.ts`]: file.source,
      [`${dir}/values.ts`]: probe("Widget", {
        ...widget,
        spec: { values: { anything: { nested: [1, "two"] } } },
      }),
    });
    expect(result[`${dir}/values.ts`]).toEqual([]);
  });

  it("ignores a version that is not served", async () => {
    const schema: { openAPIV3Schema: JSONSchema } = {
      openAPIV3Schema: { type: "object", properties: { spec } },
    };
    const file = await renderCrdTypes(
      widgetCrd({}, [
        { name: "v1alpha1", served: false, schema },
        { name: "v1", served: true, schema },
      ]),
      "widgets.yaml",
    );

    expect(file.source).toContain('"example.camunda.io/v1"');
    expect(file.source).not.toContain("v1alpha1");
  });

  it("refuses a CRD that serves two versions", async () => {
    const schema: { openAPIV3Schema: JSONSchema } = {
      openAPIV3Schema: { type: "object" },
    };
    await expect(
      renderCrdTypes(
        widgetCrd({}, [
          { name: "v1", served: true, schema },
          { name: "v2", served: true, schema },
        ]),
        "widgets.yaml",
      ),
    ).rejects.toThrow(/serves 2 versions/);
  });
});

describe("writeCrdTypes", { timeout: 60_000 }, () => {
  async function crdDir(crds: Record<string, Crd>): Promise<string> {
    const dir = await fs.mkdtemp(path.join(os.tmpdir(), "crds-"));
    for (const [name, crd] of Object.entries(crds)) {
      await fs.writeFile(path.join(dir, name), stringify(crd));
    }
    return dir;
  }

  it("removes the module of a kind that is gone", async () => {
    const crds = await crdDir({ "widgets.yaml": widgetCrd({ spec }) });
    const out = await fs.mkdtemp(path.join(os.tmpdir(), "out-"));
    await fs.writeFile(path.join(out, "gadget.ts"), "export {};\n");

    await writeCrdTypes(crds, out);

    expect(await fs.readdir(out)).toEqual(["widget.ts"]);
  });

  it("refuses two kinds that would write the same module", async () => {
    const other = widgetCrd({ spec });
    other.metadata.name = "widgets.other.camunda.io";
    other.spec.group = "other.camunda.io";
    const crds = await crdDir({
      "a.yaml": widgetCrd({ spec }),
      "b.yaml": other,
    });
    const out = await fs.mkdtemp(path.join(os.tmpdir(), "out-"));

    await expect(writeCrdTypes(crds, out)).rejects.toThrow(/widget\.ts/);
  });
});
