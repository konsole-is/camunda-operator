import type { ReactNode } from "react";
import { Panel } from "@/components/cluster/claim-tab";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { ContractInput, ContractOutput, ContractView } from "./contract";
import { fillsText } from "./groups";
import { NotRead } from "@/components/fleet/not-read";
import { ValueError } from "./tab-parts";
import { type Invalid, isInvalid } from "./values";

const MUTED = "block text-xs text-muted";

/** ContractTab shows the sections, the inputs, the outputs, and the deletion policies of contract. */
export function ContractTab({
  contract,
}: {
  contract: ContractView;
}): ReactNode {
  return (
    <div className="flex flex-col gap-4">
      <Panel label="Sections">
        <ul className="flex flex-col gap-3 text-sm">
          {contract.sections.map((section, i) =>
            isInvalid(section) ? (
              <li key={i}>
                <ValueError text={section.error} />
              </li>
            ) : (
              <li key={section.name} className="flex flex-col gap-0.5">
                <span className="font-medium">
                  {section.label ?? section.name}
                </span>
                <span className={`${MUTED} font-mono`}>{section.name}</span>
                {section.label === undefined ? (
                  contract.sectionsUnknown ? (
                    <NotRead kinds={["PluginSection"]} />
                  ) : (
                    <span className="text-xs text-warn">
                      Not a section on this cluster
                    </span>
                  )
                ) : section.fields.length === 0 ? (
                  <span className={MUTED}>Fills no field</span>
                ) : (
                  section.fields.map((field) => (
                    <span
                      key={`${field.claimKind} ${field.kind} ${field.path}`}
                      className={MUTED}
                    >
                      {fillsText(field)}
                    </span>
                  ))
                )}
              </li>
            ),
          )}
        </ul>
      </Panel>
      <Panel label="Inputs">
        <Items
          items={contract.inputs}
          none="This option takes no input."
          table={(rows) => <InputsTable inputs={rows} />}
        />
      </Panel>
      <Panel label="Outputs">
        <Items
          items={contract.outputs}
          none="This option gives no output."
          table={(rows) => <OutputsTable outputs={rows} />}
        />
      </Panel>
      <Panel label="Deletion">
        <DeletionPolicies policies={contract.deletionPolicies} />
      </Panel>
    </div>
  );
}

function DeletionPolicies({
  policies,
}: {
  policies: ContractView["deletionPolicies"];
}) {
  return (
    <ul className="flex flex-col gap-1 text-sm">
      {(["Delete", "Orphan"] as const).map((policy) => (
        <li key={policy}>
          {`${policy}: ${policies.includes(policy) ? "supported" : "not supported"}`}
        </li>
      ))}
    </ul>
  );
}

function Items<T extends object>({
  items,
  none,
  table,
}: {
  items: (T | Invalid)[];
  none: string;
  table: (rows: T[]) => ReactNode;
}) {
  if (items.length === 0) return <p className="text-sm text-muted">{none}</p>;
  const valid = items.filter((item): item is T => !isInvalid(item));
  return (
    <>
      {items.filter(isInvalid).map((item, i) => (
        <ValueError key={i} text={item.error} />
      ))}
      {valid.length > 0 && table(valid)}
    </>
  );
}

function InputsTable({ inputs }: { inputs: ContractInput[] }) {
  return (
    <Table label="Declared inputs">
      <TableHeader>
        <TableRow>
          <TableHead>Input</TableHead>
          <TableHead>From</TableHead>
          <TableHead>Formats</TableHead>
          <TableHead>Required</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {inputs.map((input) => (
          <TableRow key={input.name}>
            <TableCell className="font-mono">{input.name}</TableCell>
            <TableCell>
              {input.fromLabel}
              <span className={`${MUTED} font-mono`}>{input.from}</span>
            </TableCell>
            <TableCell className="font-mono">
              {input.formats.length > 0 ? input.formats.join(", ") : "Any"}
            </TableCell>
            <TableCell>{input.required ? "Required" : "Optional"}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

function OutputsTable({ outputs }: { outputs: ContractOutput[] }) {
  return (
    <Table label="Declared outputs">
      <TableHeader>
        <TableRow>
          <TableHead>Output</TableHead>
          <TableHead>Description</TableHead>
          <TableHead>Format</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {outputs.map((output) => (
          <TableRow key={output.key}>
            <TableCell className="font-mono">{output.key}</TableCell>
            <TableCell className="min-w-48 whitespace-normal">
              {output.description ?? "-"}
            </TableCell>
            <TableCell className="font-mono">
              {output.format ?? "Any"}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
