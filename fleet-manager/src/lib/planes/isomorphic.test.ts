import { describeIsomorphic } from "../../../test/imports/scan";

// These modules are isomorphic under their own guards.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/labels",
  "@/lib/claims/names",
  "@/lib/claims/hosts",
  "@/lib/environments",
  "@/lib/fields/slots",
  "@/lib/policy",
  "@/lib/session/precheck",
  "@/lib/write/create",
]);
