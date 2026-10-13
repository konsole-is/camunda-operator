import type { Story } from "@/gallery/registry";
import { LockedField } from "./locked-field";

export const lockedFieldStories: Story[] = [
  {
    component: "LockedField",
    name: "Short value",
    render: () => (
      <LockedField label="Version" value="8.9.23" reason="Set by the release" />
    ),
  },
  {
    component: "LockedField",
    name: "Long value",
    render: () => (
      <LockedField
        label="Image"
        value="registry.example.com/camunda/zeebe@sha256:4f1c3a7e9b2d8c6f0a5e1b3d7c9f2a4e6b8d0c2f4a6e8b0d2c4f6a8e0b2d4c6f"
        reason="Set by the environment policy"
      />
    ),
  },
];
