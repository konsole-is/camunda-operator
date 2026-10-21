import "server-only";

/**
 * prometheusAuthHeader returns the Authorization header for the values of
 * the credentials Secret of Prometheus: basic auth from username and
 * password, else a bearer token from token. No values give no header.
 * Values without either pair give missing. The header holds a credential,
 * so no caller may return it outside src/lib/.
 */
export function prometheusAuthHeader(
  values: Record<string, string> | undefined,
): { header?: string } | { missing: true } {
  if (!values) return {};
  if (values.username && values.password) {
    const pair = `${values.username}:${values.password}`;
    return { header: "Basic " + Buffer.from(pair).toString("base64") };
  }
  if (values.token) return { header: `Bearer ${values.token}` };
  return { missing: true };
}

/**
 * prometheusURL returns the address of an API path of the Prometheus at
 * address, with params added to the query of address. It keeps the base
 * path of address and drops its fragment.
 */
export function prometheusURL(
  address: string,
  path: string,
  params: Record<string, string>,
): URL {
  const url = new URL(address);
  url.pathname = url.pathname.replace(/\/+$/, "") + path;
  for (const [key, value] of Object.entries(params)) {
    url.searchParams.set(key, value);
  }
  url.hash = "";
  return url;
}
