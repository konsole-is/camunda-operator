import type { Story } from "@/gallery/registry";
import { InlineConfirm } from "./inline-confirm";
import { Toast } from "./toast";
import { TypedConfirm } from "./typed-confirm";

export const confirmStories: Story[] = [
  {
    component: "TypedConfirm",
    name: "Waiting for the name",
    render: () => (
      <TypedConfirm name="orders" confirmLabel="Delete cluster">
        The cluster orders and its data go. The backups in the bucket stay.
      </TypedConfirm>
    ),
  },
  {
    component: "InlineConfirm",
    name: "Closed",
    render: () => (
      <InlineConfirm
        triggerLabel="Delete backup"
        confirmLabel="Delete"
        goes={["The stored artifacts of the backup"]}
      />
    ),
  },
  {
    component: "InlineConfirm",
    name: "Open",
    render: () => (
      <InlineConfirm
        defaultOpen
        triggerLabel="Delete backup"
        confirmLabel="Delete"
        goes={["The stored artifacts of the backup"]}
        stays={[
          "The other backups of orders",
          "The schedule, which prunes its own backups",
        ]}
      />
    ),
  },
  {
    component: "Toast",
    name: "Saved",
    render: () => (
      <Toast result={{ kind: "saved", message: "Saved the claim orders" }} />
    ),
  },
  {
    component: "Toast",
    name: "Waiting for GitOps",
    render: () => (
      <Toast result={{ kind: "waiting", commit: "4f1c3a7e9b2d8c6f0a5e1b3d" }} />
    ),
  },
  {
    component: "Toast",
    name: "Applied",
    render: () => (
      <Toast
        result={{
          kind: "applied",
          message: "GitOps applied the claim orders",
          commit: "4f1c3a7e",
        }}
      />
    ),
  },
  {
    component: "Toast",
    name: "Error",
    render: () => (
      <Toast
        result={{
          kind: "error",
          message:
            "The claim changed since you opened it. Reload it to see the change.",
        }}
      />
    ),
  },
];
