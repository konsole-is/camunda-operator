// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TemplateEditor } from "./template-editor";

afterEach(cleanup);

const field = () =>
  screen.getByLabelText("Inline policy") as HTMLTextAreaElement;

describe("TemplateEditor", () => {
  it("inserts the chip at the cursor", () => {
    const onChange = vi.fn();
    render(
      <TemplateEditor
        label="Inline policy"
        value="arn:aws:sqs:*:*:-*"
        object={false}
        parameters={["region"]}
        onChange={onChange}
      />,
    );
    field().setSelectionRange(16, 16);
    fireEvent.click(screen.getByRole("button", { name: "{{ .namespace }}" }));
    expect(field().value).toBe("arn:aws:sqs:*:*:{{ .namespace }}-*");
    expect(onChange).toHaveBeenLastCalledWith(
      "arn:aws:sqs:*:*:{{ .namespace }}-*",
      [],
    );
  });

  it("offers a chip for each placeholder of a template parameter", () => {
    render(
      <TemplateEditor
        label="Inline policy"
        value={undefined}
        object={false}
        parameters={["region", "account"]}
        onChange={() => {}}
      />,
    );
    expect(screen.getAllByRole("button").map((b) => b.textContent)).toEqual([
      "{{ .namespace }}",
      "{{ .claim }}",
      "{{ .request }}",
      "{{ .serviceAccount }}",
      "{{ .parameters.region }}",
      "{{ .parameters.account }}",
    ]);
  });

  it("has no chip for .outputs or .pods", () => {
    render(
      <TemplateEditor
        label="Inline policy"
        value={undefined}
        object
        parameters={[]}
        onChange={() => {}}
      />,
    );
    const chips = screen.getAllByRole("button").map((b) => b.textContent);
    expect(chips.some((c) => c?.includes(".outputs"))).toBe(false);
    expect(chips.some((c) => c?.includes(".pods"))).toBe(false);
  });

  it("shows the error of an unknown placeholder at the field", () => {
    const onChange = vi.fn();
    render(
      <TemplateEditor
        label="Inline policy"
        value={{ Resource: "x" }}
        object
        parameters={[]}
        onChange={onChange}
      />,
    );
    fireEvent.change(field(), {
      target: { value: "Resource: '{{ .claimName }}'\n" },
    });
    expect(field().getAttribute("aria-invalid")).toBe("true");
    expect(
      screen.getByText("Resource: unknown placeholder .claimName"),
    ).toBeTruthy();
    expect(onChange).toHaveBeenLastCalledWith(
      { Resource: "{{ .claimName }}" },
      [{ path: "Resource", message: "unknown placeholder .claimName" }],
    );
  });

  it("shows the error of .outputs in a template parameter at the field", () => {
    render(
      <TemplateEditor
        label="Inline policy"
        value={undefined}
        object={false}
        parameters={[]}
        onChange={() => {}}
      />,
    );
    fireEvent.change(field(), { target: { value: "{{ .outputs.bucket }}" } });
    expect(
      screen.getByText(
        "the placeholder .outputs.bucket is not allowed: a template parameter cannot read the outputs",
      ),
    ).toBeTruthy();
  });

  it("edits an object as YAML and gives the parsed object", () => {
    const onChange = vi.fn();
    render(
      <TemplateEditor
        label="Inline policy"
        value={{ Version: "2012-10-17" }}
        object
        parameters={[]}
        onChange={onChange}
      />,
    );
    expect(field().value).toBe("Version: 2012-10-17\n");
    fireEvent.change(field(), {
      target: { value: "Version: '2012-10-17'\nAction: sqs:*\n" },
    });
    expect(onChange).toHaveBeenLastCalledWith(
      { Version: "2012-10-17", Action: "sqs:*" },
      [],
    );
  });

  it("gives no value and the error of YAML that does not parse", () => {
    const onChange = vi.fn();
    render(
      <TemplateEditor
        label="Inline policy"
        value={undefined}
        object
        parameters={[]}
        onChange={onChange}
      />,
    );
    fireEvent.change(field(), { target: { value: "a: [" } });
    const [value, errors] = onChange.mock.lastCall ?? [];
    expect(value).toBeUndefined();
    expect(errors).toHaveLength(1);
  });

  it("gives no value for empty text", () => {
    const onChange = vi.fn();
    render(
      <TemplateEditor
        label="Inline policy"
        value="x"
        object={false}
        parameters={[]}
        onChange={onChange}
      />,
    );
    fireEvent.change(field(), { target: { value: "" } });
    expect(onChange).toHaveBeenLastCalledWith(undefined, []);
  });
});
