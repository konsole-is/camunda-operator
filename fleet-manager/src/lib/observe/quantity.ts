// This module runs in the browser and on the server, so it imports nothing.

const QUANTITY =
  /^([+-]?(?:\d+(?:\.\d*)?|\.\d+))(?:[eE]([+-]?\d+)|(Ki|Mi|Gi|Ti|Pi|Ei|n|u|m|k|M|G|T|P|E))?$/;

const SUFFIX: Record<string, number> = {
  n: 1e-9,
  u: 1e-6,
  m: 1e-3,
  k: 1e3,
  M: 1e6,
  G: 1e9,
  T: 1e12,
  P: 1e15,
  E: 1e18,
  Ki: 2 ** 10,
  Mi: 2 ** 20,
  Gi: 2 ** 30,
  Ti: 2 ** 40,
  Pi: 2 ** 50,
  Ei: 2 ** 60,
};

/**
 * parseQuantity returns the number of a Kubernetes quantity, such as
 * "250m" or "128Mi". It throws an error that quotes value when value is not
 * a quantity.
 */
export function parseQuantity(value: string): number {
  const match = QUANTITY.exec(value);
  if (!match) throw new Error(`"${value}" is not a Kubernetes quantity`);
  const [, number, exponent, suffix] = match;
  const scale = suffix ? SUFFIX[suffix] : 10 ** Number(exponent ?? 0);
  return Number(number) * scale;
}
