const NAME = /^[A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?$/;
const VALUE = /^([A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?)?$/;
const PREFIX =
  /^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/;

/** isLabelKey says whether key is a valid Kubernetes label key: name or prefix/name. */
export function isLabelKey(key: string): boolean {
  const parts = key.split("/");
  if (parts.length > 2) return false;
  const name = parts.at(-1) ?? "";
  const prefix = parts.length === 2 ? parts[0] : undefined;
  return (
    NAME.test(name) &&
    (prefix === undefined || (prefix.length <= 253 && PREFIX.test(prefix)))
  );
}

/** isLabelValue says whether value is a valid Kubernetes label value. The empty value is valid. */
export function isLabelValue(value: string): boolean {
  return VALUE.test(value);
}
