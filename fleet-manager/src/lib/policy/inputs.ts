import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import { byCodeUnit, own, setOwn } from "./json";
import type { InputRef, PluginSelection } from "./types";

/**
 * InputValues are the resolved inputs of one request. values holds the value
 * of each input by input name. waiting holds the sections, sorted and
 * distinct, whose outputs an input waits for.
 */
export type InputValues = {
  values?: Record<string, string>;
  waiting?: string[];
};

/**
 * resolveInputs resolves the inputs of registration from the outputs of the
 * requests of a claim, by the rule of Inputs of pkg/claimpolicy. An input
 * reads the request of the first selection of its section, in claim order,
 * and only through readyOutputs. Until then the section is in waiting. An
 * input whose section has no selection is left out and does not wait.
 */
export function resolveInputs(
  registration: PluginRegistration,
  selections: readonly PluginSelection[],
  requests: readonly PluginRequest[],
): InputValues {
  const values: Record<string, string> = {};
  const waiting = new Set<string>();
  for (const input of registration.spec.inputs ?? []) {
    const { section, output } = splitFrom(input.from);
    const selection = selections.find((s) => s.section === section);
    if (!selection) continue;

    const outputs = readyOutputs(requests, section, selection.registration);
    const value = outputs && own(outputs, output);
    if (value === undefined) waiting.add(section);
    else setOwn(values, input.name, value);
  }

  return {
    ...(Object.keys(values).length > 0 && { values }),
    ...(waiting.size > 0 && { waiting: [...waiting].sort(byCodeUnit) }),
  };
}

/**
 * readyOutputs returns the outputs of the request of section and
 * registration while it reports OutputsReady=True for its current
 * generation, else undefined.
 */
export function readyOutputs(
  requests: readonly PluginRequest[],
  section: string,
  registration: string,
): Record<string, string> | undefined {
  const request = requests.find(
    (r) => r.spec.section === section && r.spec.registration === registration,
  );
  const ready = request?.status?.conditions?.find(
    (c) => c.type === "OutputsReady",
  );
  if (
    !request ||
    ready?.status !== "True" ||
    (ready.observedGeneration ?? 0) !== (request.metadata.generation ?? 0)
  ) {
    return undefined;
  }
  return request.status?.outputs;
}

/**
 * missingInputs returns the required inputs of the registration of
 * selection whose section has no selection in selections, in input order.
 * The claim controller refuses such a claim with MissingInput. An unknown
 * registration gives no input.
 */
export function missingInputs(
  selection: PluginSelection,
  selections: readonly PluginSelection[],
  registrations: readonly PluginRegistration[],
): InputRef[] {
  const registration = registrations.find(
    (r) => r.metadata.name === selection.registration,
  );
  if (!registration) return [];

  return (registration.spec.inputs ?? [])
    .filter(
      (input) =>
        input.required &&
        !selections.some((s) => s.section === splitFrom(input.from).section),
    )
    .map((input) => ({
      name: input.name,
      from: input.from,
      ...splitFrom(input.from),
    }));
}

function splitFrom(from: string): { section: string; output: string } {
  const slash = from.indexOf("/");
  return slash < 0
    ? { section: from, output: "" }
    : { section: from.slice(0, slash), output: from.slice(slash + 1) };
}
