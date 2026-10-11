import fs from "node:fs";
import path from "node:path";

const SERVER_ONLY_MARKER = /^import\s*(["'])server-only\1/;
const EXTENSIONS = [".ts", ".tsx", ".mts", ".js", ".jsx", ".mjs"];
// TypeScript ESM code names a source file by the extension it compiles to.
const SOURCE_EXTENSIONS = new Map([
  [".js", [".ts", ".tsx"]],
  [".jsx", [".tsx"]],
  [".mjs", [".mts"]],
]);

/** @type {Map<string, { mtimeMs: number, serverOnly: boolean }>} */
const markerCache = new Map();

/** @param {string} text */
function withoutLeadingComments(text) {
  let rest = text.replace(/^﻿/, "");
  for (;;) {
    rest = rest.trimStart();
    if (rest.startsWith("//")) {
      const end = rest.indexOf("\n");
      rest = end < 0 ? "" : rest.slice(end + 1);
    } else if (rest.startsWith("/*")) {
      const end = rest.indexOf("*/");
      rest = end < 0 ? "" : rest.slice(end + 2);
    } else {
      return rest;
    }
  }
}

/** @param {string} file */
function startsWithServerOnly(file) {
  const { mtimeMs } = fs.statSync(file);
  const cached = markerCache.get(file);
  if (cached && cached.mtimeMs === mtimeMs) return cached.serverOnly;
  const serverOnly = SERVER_ONLY_MARKER.test(
    withoutLeadingComments(fs.readFileSync(file, "utf8")),
  );
  markerCache.set(file, { mtimeMs, serverOnly });
  return serverOnly;
}

/** @param {string} base */
function resolveModuleFile(base) {
  const ext = path.extname(base);
  const stem = base.slice(0, base.length - ext.length);
  const candidates = [
    base,
    ...(SOURCE_EXTENSIONS.get(ext) ?? []).map((source) => stem + source),
    ...EXTENSIONS.map((added) => base + added),
    ...EXTENSIONS.map((added) => path.join(base, `index${added}`)),
  ];
  return candidates.find(
    (file) => fs.existsSync(file) && fs.statSync(file).isFile(),
  );
}

/**
 * @param {any} node An import or export declaration of typescript-eslint.
 * @param {"importKind" | "exportKind"} kind
 */
function isTypeOnly(node, kind) {
  if (node[kind] === "type") return true;
  return (
    node.specifiers.length > 0 &&
    node.specifiers.every((specifier) => specifier[kind] === "type")
  );
}

/** @param {import("estree").Program} program */
function isClientFile(program) {
  for (const statement of program.body) {
    if (
      statement.type !== "ExpressionStatement" ||
      typeof statement.directive !== "string"
    ) {
      return false;
    }
    if (statement.directive === "use client") return true;
  }
  return false;
}

/**
 * Reports an import of a server-only module of `<srcDir>/lib/` from a file
 * whose directive prologue holds "use client". A module is server-only when
 * its first statement, after comments, is `import "server-only"`. Type-only
 * imports are allowed. Imports through another module are not checked:
 * `next build` refuses those.
 *
 * Option `srcDir`: the absolute path of the src directory. The alias `@/`
 * resolves to it.
 *
 * @type {import("eslint").Rule.RuleModule}
 */
const rule = {
  meta: {
    type: "problem",
    docs: {
      description:
        'Refuse an import of a server-only module of src/lib from a "use client" file',
    },
    schema: [
      {
        type: "object",
        properties: { srcDir: { type: "string" } },
        required: ["srcDir"],
        additionalProperties: false,
      },
    ],
    messages: {
      serverOnlyImport:
        '"{{source}}" starts with import "server-only". A "use client" file cannot import it.',
    },
  },
  create(context) {
    if (!isClientFile(context.sourceCode.ast)) return {};
    const { srcDir } = context.options[0];
    const libDir = path.join(srcDir, "lib");
    const fromDir = path.dirname(context.filename);

    /**
     * @param {import("estree").Node} node
     * @param {unknown} source
     */
    function check(node, source) {
      if (typeof source !== "string") return;
      let base;
      if (source.startsWith("@/")) base = path.join(srcDir, source.slice(2));
      else if (source.startsWith(".")) base = path.resolve(fromDir, source);
      if (!base) return;
      if (base !== libDir && !base.startsWith(libDir + path.sep)) return;
      const file = resolveModuleFile(base);
      if (file && startsWithServerOnly(file)) {
        context.report({
          node,
          messageId: "serverOnlyImport",
          data: { source },
        });
      }
    }

    return {
      ImportDeclaration(node) {
        if (!isTypeOnly(node, "importKind")) check(node, node.source.value);
      },
      ExportNamedDeclaration(node) {
        if (node.source && !isTypeOnly(node, "exportKind"))
          check(node, node.source.value);
      },
      ExportAllDeclaration(node) {
        if (node.exportKind !== "type") check(node, node.source.value);
      },
      ImportExpression(node) {
        const { source } = node;
        if (source.type === "Literal") check(node, source.value);
        else if (
          source.type === "TemplateLiteral" &&
          !source.expressions.length
        )
          check(node, source.quasis[0].value.cooked);
      },
    };
  },
};

export default rule;
