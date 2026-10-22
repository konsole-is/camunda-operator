import { describe, expect, it } from "vitest";
import { type TemplateData, parseTemplate, render } from ".";

const data: TemplateData = {
  namespace: "team-a",
  claim: "orders",
  request: "orders-backup-storage",
  serviceAccount: "orders-camunda",
  parameters: { x: "eu-1", size: 3, on: true, obj: { a: 1 }, none: null },
  outputs: { y: "bucket-1" },
  pods: "orders-zeebe-0|orders-zeebe-1",
};

describe("render", () => {
  it.each([
    ["{{ .namespace }}", "team-a"],
    ["{{ .claim }}", "orders"],
    ["{{ .request }}", "orders-backup-storage"],
    ["{{ .serviceAccount }}", "orders-camunda"],
    ["{{ .parameters.x }}", "eu-1"],
    ["{{ .outputs.y }}", "bucket-1"],
  ])("fills %s in a grant template", (template, want) =>
    expect(render(template, data, "grant")).toEqual({ text: want }),
  );

  it("fills .pods, .parameters.x, and .outputs.y in a metric query", () =>
    expect(
      render(
        'up{pod=~"{{ .pods }}",r="{{ .parameters.x }}",b="{{.outputs.y}}"}',
        data,
        "metric",
      ),
    ).toEqual({
      text: 'up{pod=~"orders-zeebe-0|orders-zeebe-1",r="eu-1",b="bucket-1"}',
    }));

  it("keeps the selector of a metric query when a value holds a quote or a backslash", () =>
    expect(
      render(
        'up{r="{{ .parameters.x }}",b="b"}',
        { ...data, parameters: { x: 'a\\",b=~".*\nz' } },
        "metric",
      ),
    ).toEqual({ text: 'up{r="a\\\\\\",b=~\\".*\\nz",b="b"}' }));

  it("fills a value with a quote or a backslash into a grant template as it is", () =>
    expect(
      render(
        '{"r":"{{ .parameters.x }}"}',
        { ...data, parameters: { x: 'a\\"\nz' } },
        "grant",
      ),
    ).toEqual({ text: '{"r":"a\\"\nz"}' }));

  it.each(["grant", "parameter"] as const)(
    "refuses .pods in a %s template",
    (kind) =>
      expect(render("{{ .pods }}", data, kind)).toEqual({
        error: "the placeholder .pods is allowed only in a metric query",
        field: "pods",
      }),
  );

  it("refuses an unknown name and names it", () => {
    const result = render("{{ .foo }}", data, "metric");
    expect(result).toMatchObject({ field: "foo" });
    expect((result as { error: string }).error).toContain(".foo");
  });

  it.each([
    ["{{ .parameters.x.y }}", "parameters.x.y"],
    ["{{ .namespace.x }}", "namespace.x"],
  ])("refuses the nested name %s", (template, field) =>
    expect(render(template, data, "metric")).toMatchObject({ field }),
  );

  it.each([
    ["{{ if .namespace }}x{{ end }}", "if"],
    ["{{ range .parameters }}{{ end }}", "range"],
    ["{{ with .claim }}{{ end }}", "with"],
    ['{{ template "a" }}', "template"],
    ['{{ define "a" }}x{{ end }}', "define"],
    ['{{ block "a" . }}x{{ end }}', "block"],
    ["{{/* note */}}", "comment"],
    ["{{- /* note */ -}}", "comment"],
    ['{{ .namespace | printf "%s" }}', "pipeline"],
    ['{{ printf "%s" .namespace }}', "printf"],
    ["{{ $x := .namespace }}", "variable"],
    ["{{ $ }}", "$"],
    ['{{ "text" }}', '"text"'],
    ["{{ . }}", "{{ . }}"],
  ])("refuses %s and names the construct", (template, construct) => {
    const result = render(template, data, "metric");
    expect("error" in result && result.error).toContain(construct);
  });

  it("does not take a | inside a string for a pipeline", () => {
    const result = render('{{ printf "a|b" .claim }}', data, "metric");
    expect("error" in result && result.error).toContain("printf");
  });

  it("refuses .parameters without a name", () =>
    expect(
      (render("{{ .parameters }}", data, "grant") as { error: string }).error,
    ).toContain(".parameters.<name>"));

  it("refuses .outputs without a name", () =>
    expect(
      (render("{{ .outputs }}", data, "grant") as { error: string }).error,
    ).toContain(".outputs.<name>"));

  it("names a parameter that has no value", () =>
    expect(render("{{ .parameters.missing }}", data, "grant")).toEqual({
      error: 'parameter "missing" has no value',
      field: "parameters.missing",
    }));

  it("names an output that has no value", () =>
    expect(render("{{ .outputs.nope }}", data, "grant")).toEqual({
      error: 'output "nope" has no value',
      field: "outputs.nope",
    }));

  it("writes a number and a boolean parameter as text", () =>
    expect(
      render("{{ .parameters.size }}/{{ .parameters.on }}", data, "parameter"),
    ).toEqual({ text: "3/true" }));

  it.each(["obj", "none"])(
    "refuses the parameter %s, which is not text",
    (name) =>
      expect(
        render(`{{ .parameters.${name} }}`, data, "parameter"),
      ).toMatchObject({ field: `parameters.${name}` }),
  );

  it("fills a parameter whose name has a letter outside ASCII, as Go does", () =>
    expect(
      render(
        "{{ .parameters.région }}",
        { ...data, parameters: { région: "eu" } },
        "grant",
      ),
    ).toEqual({ text: "eu" }));

  it("names the function of a call whose name has a letter outside ASCII", () => {
    const result = render("{{ fünf .claim }}", data, "grant");
    expect("error" in result && result.error).toContain("fünf");
  });

  it("trims whitespace around {{- and -}} as Go does", () =>
    expect(render("a  {{- .claim -}}  b", data, "grant")).toEqual({
      text: "aordersb",
    }));

  it("keeps a - that has no space after it as part of the action", () => {
    const result = render("a {{-3}}", data, "grant");
    expect(result).toHaveProperty("error");
  });

  it("refuses an action that is not closed", () =>
    expect(
      (render("a {{ .claim", data, "grant") as { error: string }).error,
    ).toContain("not closed"));

  it("keeps text without actions", () =>
    expect(render("plain } text", data, "grant")).toEqual({
      text: "plain } text",
    }));

  it("renders the empty template as the empty text", () =>
    expect(render("", data, "grant")).toEqual({ text: "" }));

  it("refuses .pods in a metric query when the data has no pods", () =>
    expect(
      render("{{ .pods }}", { ...data, pods: undefined }, "metric"),
    ).toMatchObject({ field: "pods" }));
});

describe("parseTemplate", () => {
  it("lists the parameters, the outputs, and .pods", () =>
    expect(
      parseTemplate("{{ .parameters.x }}{{ .outputs.y }}{{ .pods }}", "metric"),
    ).toEqual({ parameters: ["x"], outputs: ["y"], usesPods: true }));

  it("lists each name once", () =>
    expect(
      parseTemplate("{{ .parameters.x }}{{ .parameters.x }}", "grant"),
    ).toEqual({ parameters: ["x"], outputs: [], usesPods: false }));

  it("gives the error of an unknown name", () =>
    expect(parseTemplate("{{ .foo }}", "grant")).toEqual({
      error: "unknown placeholder .foo",
      field: "foo",
    }));
});
