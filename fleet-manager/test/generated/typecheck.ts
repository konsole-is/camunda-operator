import path from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const appDir = fileURLToPath(new URL("../..", import.meta.url));

/**
 * typeCheck type-checks sources with the compiler options of the app and
 * returns the diagnostic messages of each source. A key is a path relative to
 * the app directory. A source can import the other sources and any file or
 * package of the app.
 */
export function typeCheck(
  sources: Record<string, string>,
): Record<string, string[]> {
  const config = ts.readConfigFile(
    path.join(appDir, "tsconfig.json"),
    ts.sys.readFile,
  );
  const { options } = ts.parseJsonConfigFileContent(
    config.config,
    ts.sys,
    appDir,
  );
  options.incremental = false;

  const files = new Map(
    Object.entries(sources).map(([name, text]) => [
      path.join(appDir, name),
      text,
    ]),
  );
  const host = ts.createCompilerHost(options);
  const { getSourceFile, fileExists, readFile } = host;
  host.getSourceFile = (fileName, language, ...rest) => {
    const text = files.get(fileName);
    return text === undefined
      ? getSourceFile.call(host, fileName, language, ...rest)
      : ts.createSourceFile(fileName, text, language);
  };
  host.fileExists = (fileName) =>
    files.has(fileName) || fileExists.call(host, fileName);
  host.readFile = (fileName) =>
    files.get(fileName) ?? readFile.call(host, fileName);
  // Module resolution skips a directory that does not exist, and the
  // directory of a source does not exist on disk.
  host.directoryExists = (dirName) =>
    [...files.keys()].some((name) => name.startsWith(dirName + path.sep)) ||
    ts.sys.directoryExists(dirName);

  const program = ts.createProgram([...files.keys()], options, host);
  return Object.fromEntries(
    Object.keys(sources).map((name) => [
      name,
      ts
        .getPreEmitDiagnostics(
          program,
          program.getSourceFile(path.join(appDir, name)),
        )
        .map((d) => ts.flattenDiagnosticMessageText(d.messageText, "\n")),
    ]),
  );
}
