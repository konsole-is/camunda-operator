import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { valueImports } from "../../../test/imports/scan";
import { filterLines, parseLogLine } from "./logs";

describe("parseLogLine", () => {
  it("parses a JSON line into its fields", () => {
    const raw =
      '{"ts":"2026-10-21T10:00:00Z","level":"info","msg":"Reconciled","logger":"plugin","extra":1}';
    expect(parseLogLine(raw)).toEqual({
      raw,
      time: "2026-10-21T10:00:00Z",
      level: "info",
      message: "Reconciled",
      logger: "plugin",
      fields: { extra: 1 },
    });
  });

  it("reads the request of a plugin controller line", () => {
    const raw =
      '{"level":"error","msg":"Failed","request":"team-a/orders-access"}';
    expect(parseLogLine(raw)).toMatchObject({
      request: "team-a/orders-access",
      level: "error",
      fields: {},
    });
  });

  it.each([
    ['{"name":"basic-delete","namespace":"ns"}', "ns/basic-delete"],
    ['{"name":"basic-delete"}', "basic-delete"],
  ])("reads the request object %s as %s", (request, want) => {
    const line = parseLogLine(`{"msg":"m","request":${request}}`);
    expect(line.request).toBe(want);
    expect(line.fields).toEqual({});
  });

  it("keeps a request object without a name in the fields", () =>
    expect(parseLogLine('{"msg":"m","request":{"id":1}}')).toMatchObject({
      fields: { request: { id: 1 } },
    }));

  it("keeps a text line as raw and message only", () =>
    expect(parseLogLine("Started broker 0")).toEqual({
      raw: "Started broker 0",
      message: "Started broker 0",
      fields: {},
    }));

  it.each(["[1,2]", '"text"', "{not json", "null"])(
    "treats %s as a text line",
    (raw) =>
      expect(parseLogLine(raw)).toEqual({ raw, message: raw, fields: {} }),
  );

  it("reads the Spring Boot keys of a Camunda JSON line", () =>
    expect(
      parseLogLine(
        '{"timestamp":"t","severity":"WARN","message":"m","logger_name":"io.camunda"}',
      ),
    ).toMatchObject({
      time: "t",
      level: "WARN",
      message: "m",
      logger: "io.camunda",
    }));

  it("uses the raw line as the message of a JSON line without one", () => {
    const raw = '{"level":"info"}';
    expect(parseLogLine(raw)).toEqual({
      raw,
      level: "info",
      message: raw,
      fields: {},
    });
  });

  it("keeps a known key that is not a string in the fields", () =>
    expect(parseLogLine('{"msg":"m","level":30}')).toEqual({
      raw: '{"msg":"m","level":30}',
      message: "m",
      fields: { level: 30 },
    }));
});

describe("filterLines", () => {
  it("the request filter keeps only the lines of that request", () => {
    const lines = [
      '{"msg":"a","request":"ns/one"}',
      '{"msg":"b","request":"ns/two"}',
      "plain",
    ].map(parseLogLine);
    expect(
      filterLines(lines, { request: "ns/one" }).map((l) => l.message),
    ).toEqual(["a"]);
  });

  it("the request filter keeps a line with a request object", () => {
    const lines = [
      '{"msg":"a","request":{"name":"one","namespace":"ns"}}',
      '{"msg":"b","request":"ns/two"}',
    ].map(parseLogLine);
    expect(
      filterLines(lines, { request: "ns/one" }).map((l) => l.message),
    ).toEqual(["a"]);
  });

  it("filters by level without case and by text in the raw line", () => {
    const lines = [
      '{"level":"ERROR","msg":"disk full"}',
      '{"level":"info","msg":"disk ok"}',
    ].map(parseLogLine);
    expect(filterLines(lines, { level: "error" })).toHaveLength(1);
    expect(filterLines(lines, { text: "OK" }).map((l) => l.message)).toEqual([
      "disk ok",
    ]);
  });

  it("an empty filter keeps every line", () => {
    const lines = ["a", "b"].map(parseLogLine);
    expect(filterLines(lines, {})).toEqual(lines);
  });
});

it("logs.ts imports values only from its own directory", () =>
  expect(
    valueImports(
      readFileSync(new URL("./logs.ts", import.meta.url), "utf8"),
    ).filter((s) => !s.startsWith("./")),
  ).toEqual([]));
