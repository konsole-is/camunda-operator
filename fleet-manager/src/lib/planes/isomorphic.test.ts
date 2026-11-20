import { describeIsomorphic } from "../../../test/imports/scan";

// These modules are isomorphic under their own guards.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/labels",
  "@/lib/claims/names",
  "@/lib/claims/hosts",
  "@/lib/clusters/draft",
  "@/lib/drafts",
  "@/lib/environments",
  "@/lib/fields",
  "@/lib/fields/slots",
  "@/lib/policy",
  "@/lib/schema",
  "@/lib/session",
  "@/lib/session/precheck",
  "@/lib/text",
  "@/lib/write/create",
]);
