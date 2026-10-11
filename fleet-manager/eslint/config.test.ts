import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { ESLint } from "eslint";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

const appDir = fileURLToPath(new URL("..", import.meta.url));
// The suite lints through the real config, which checks src/lib/ only, so the
// probe module must live there. .gitignore lists the directory.
const probeDir = path.join(appDir, "src", "lib", "__lint_probe__");

async function ruleIds(code: string, file: string): Promise<(string | null)[]> {
  const eslint = new ESLint({ cwd: appDir });
  const [result] = await eslint.lintText(code, {
    filePath: path.join(appDir, file),
  });
  return result.messages.map((message) => message.ruleId);
}

describe("eslint.config.mjs", { timeout: 60_000 }, () => {
  beforeAll(() => {
    fs.mkdirSync(probeDir, { recursive: true });
    fs.writeFileSync(
      path.join(probeDir, "secret.ts"),
      'import "server-only";\n\nexport const secret = "probe";\n',
    );
  });

  afterAll(() => {
    fs.rmSync(probeDir, { recursive: true, force: true });
  });

  it("refuses a client component that imports a server-only module of src/lib", async () => {
    const code = `"use client";\n\nimport { secret } from "@/lib/__lint_probe__/secret";\n\nexport const value = secret;\n`;
    expect(await ruleIds(code, "src/components/probe.tsx")).toContain(
      "fleet/no-server-only-in-client",
    );
  });

  it("lets a server component import the same module", async () => {
    const code = `import { secret } from "@/lib/__lint_probe__/secret";\n\nexport const value = secret;\n`;
    expect(await ruleIds(code, "src/components/probe.tsx")).not.toContain(
      "fleet/no-server-only-in-client",
    );
  });

  it("refuses the Edge runtime on a route", async () => {
    const code = `export const runtime = "edge";\n`;
    expect(await ruleIds(code, "src/app/probe/route.ts")).toContain(
      "no-restricted-syntax",
    );
  });
});
