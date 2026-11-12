// This module runs in the browser and on the server.
import { sha256 } from "@noble/hashes/sha2.js";
import { bytesToHex, utf8ToBytes } from "@noble/hashes/utils.js";

// The name is the value of the plugins.camunda.io/request label.
const MAX_NAME = 63;
const HEAD = 54;

/**
 * requestName returns the name of the request of claim for selection in a
 * section of mode, by the rule of Name of pkg/components/claimrequests:
 * <claim>-<section> for One and for an unknown mode, and
 * <claim>-<section>-<registration> for Many. A name over 63 characters is
 * cut to 54, without a trailing "-" or ".", and gets "-" and the first 8 hex
 * characters of the SHA-256 of the full name.
 */
export function requestName(
  claim: string,
  selection: { section: string; registration: string },
  mode: "One" | "Many" | "",
): string {
  let name = `${claim}-${selection.section}`;
  if (mode === "Many") name += `-${selection.registration}`;
  if (name.length <= MAX_NAME) return name;
  const hash = bytesToHex(sha256(utf8ToBytes(name))).slice(0, 8);
  return `${name.slice(0, HEAD).replace(/[-.]+$/, "")}-${hash}`;
}
