// This module runs in the browser and on the server.

/** MAX_CLAIM_NAME is the longest name that a claim of either kind can have. */
export const MAX_CLAIM_NAME = 40;

const DNS_1035 = /^[a-z]([-a-z0-9]*[a-z0-9])?$/;

/**
 * claimNameErrors returns every error of name as the name of a new
 * CamundaClusterClaim or CamundaManagementClusterClaim, or none. It applies
 * the rule of both CRDs: a DNS-1035 label of at most 40 characters, because
 * the claim adds a suffix to its name in the names of the resources that it
 * writes. A name in taken also fails. An empty name gives only "Enter a name".
 */
export function claimNameErrors(
  name: string,
  taken: readonly string[],
): string[] {
  if (name === "") return ["Enter a name"];
  const errors: string[] = [];
  if (!DNS_1035.test(name)) {
    errors.push(
      "Start with a lowercase letter. Use only lowercase letters, digits, and '-'. End with a letter or a digit.",
    );
  }
  if (name.length > MAX_CLAIM_NAME) {
    errors.push(`Use at most ${MAX_CLAIM_NAME} characters.`);
  }
  if (taken.includes(name)) {
    errors.push(`The name ${name} is taken in this namespace.`);
  }
  return errors;
}

/** namesIn returns the names of the objects of every list in namespace, each once, in the order of the lists. */
export function namesIn(
  namespace: string,
  ...lists: readonly (readonly {
    metadata?: { name?: string; namespace?: string };
  }[])[]
): string[] {
  const names = new Set<string>();
  for (const list of lists) {
    for (const object of list) {
      const name = object.metadata?.name;
      if (name && object.metadata?.namespace === namespace) names.add(name);
    }
  }
  return [...names];
}
