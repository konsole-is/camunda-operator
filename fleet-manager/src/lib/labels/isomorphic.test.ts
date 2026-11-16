import { describeIsomorphic } from "../../../test/imports/scan";

describeIsomorphic(new URL(".", import.meta.url), [
  "@noble/hashes/sha2.js",
  "@noble/hashes/utils.js",
]);
