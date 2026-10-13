import type { Story } from "@/gallery/registry";
import { AppShell } from "./app-shell";
import { Logo } from "./logo";
import { SetupBreadcrumb } from "./setup-breadcrumb";
import { SetupNav } from "./setup-nav";
import { TopNav } from "./top-nav";

const heading = (text: string) => (
  <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">{text}</h1>
);

export const shellStories: Story[] = [
  { component: "Logo", name: "Default", render: () => <Logo /> },
  {
    component: "TopNav",
    name: "Fleet active",
    render: () => <TopNav pathname="/fleet" />,
  },
  {
    component: "TopNav",
    name: "Setup active",
    render: () => <TopNav pathname="/setup/presets" />,
  },
  {
    component: "SetupNav",
    name: "Presets current",
    render: () => <SetupNav pathname="/setup/presets" />,
  },
  {
    component: "SetupBreadcrumb",
    name: "Detail page",
    render: () => <SetupBreadcrumb pathname="/setup/presets/gold" />,
  },
  {
    component: "SetupBreadcrumb",
    name: "Long name",
    render: () => (
      <SetupBreadcrumb pathname="/setup/releases/camunda-8-9-long-term-support-with-extended-security-patches-eu-north-1" />
    ),
  },
  {
    component: "AppShell",
    name: "Fleet page",
    render: () => <AppShell pathname="/fleet">{heading("Fleet")}</AppShell>,
  },
  {
    component: "AppShell",
    name: "Setup detail page",
    render: () => (
      <AppShell
        pathname="/setup/presets/gold"
        sideNav={<SetupNav pathname="/setup/presets/gold" />}
      >
        <SetupBreadcrumb pathname="/setup/presets/gold" />
        {heading("gold")}
      </AppShell>
    ),
  },
  {
    component: "AppShell",
    name: "Setup detail with a long name",
    render: () => (
      <AppShell
        pathname="/setup/releases/x"
        sideNav={<SetupNav pathname="/setup/releases/x" />}
      >
        <SetupBreadcrumb pathname="/setup/releases/camunda-8-9-long-term-support-with-extended-security-patches-eu-north-1" />
        {heading("Release")}
      </AppShell>
    ),
  },
  {
    component: "AppShell",
    name: "With Git waiting banner",
    render: () => (
      <AppShell
        pathname="/fleet"
        banner={
          <div
            role="status"
            className="flex flex-wrap items-center gap-2.5 rounded-lg border border-border bg-panel px-3.5 py-2.5 text-text-2"
          >
            <span
              aria-hidden
              className="size-2 rounded-full bg-info shadow-[0_0_8px_var(--info)]"
            />
            Waiting for GitOps to apply
          </div>
        }
      >
        {heading("Fleet")}
      </AppShell>
    ),
  },
];
