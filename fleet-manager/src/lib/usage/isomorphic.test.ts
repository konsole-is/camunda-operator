import { describeIsomorphic } from "../../../test/imports/scan";

// @/lib/policy, @/lib/labels, and @/lib/merge are isomorphic under their
// own guards, and @/lib/policy imports @/lib/fields/slots too.
// @/lib/k8s/unread imports nothing.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/policy",
  "@/lib/labels",
  "@/lib/labels/owner-name",
  "@/lib/merge",
  "@/lib/fields/slots",
  "@/lib/k8s/unread",
]);
