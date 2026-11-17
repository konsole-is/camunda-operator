import { describeIsomorphic } from "../../../test/imports/scan";

// Each of these is isomorphic under its own guard. @/lib/write/refs and
// @/lib/write/ownership are isomorphic modules of the server-only write
// folder.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/clusters/storage",
  "@/lib/drafts",
  "@/lib/fields",
  "@/lib/labels",
  "@/lib/merge",
  "@/lib/planes/components",
  "@/lib/planes/draft",
  "@/lib/planes/identity",
  "@/lib/planes/storage",
  "@/lib/planes/urls",
  "@/lib/planes/versions",
  "@/lib/policy",
  "@/lib/schema",
  "@/lib/write/ownership",
  "@/lib/write/refs",
]);
