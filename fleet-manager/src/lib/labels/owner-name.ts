// This module runs in the browser and on the server.
import { sha256 } from "@noble/hashes/sha2.js";
import { bytesToHex, utf8ToBytes } from "@noble/hashes/utils.js";

// The rule of labels.BoundedName in pkg/labels, with the bound of a label
// value. test/fixtures/labels/owner-names.json keeps the two the same.
const LABEL_VALUE_MAX = 63;
const HASH_LENGTH = 10;

/**
 * ownerName returns the value that the operator puts in an owner label, such
 * as camunda.io/cluster, for an owner of the given name. It is OwnerName of
 * pkg/labels: a name of more than 63 characters gives a different value.
 */
export function ownerName(name: string): string {
  if (name.length <= LABEL_VALUE_MAX) return name;
  const hash = bytesToHex(sha256(utf8ToBytes(name)));
  const head = name
    .slice(0, LABEL_VALUE_MAX - 1 - HASH_LENGTH)
    .replace(/[-.]+$/, "");
  return `${head}-${hash.slice(0, HASH_LENGTH)}`;
}
