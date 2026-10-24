import { describeIsomorphic } from "../../../test/imports/scan";

describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/labels",
  "@/lib/fields/slots",
]);
