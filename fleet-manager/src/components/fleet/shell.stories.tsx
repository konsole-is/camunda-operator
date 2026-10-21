import type { Story } from "@/gallery/registry";
import { AppShell } from "./app-shell";
import { GitBannerView } from "./git-banner";
import { GitWaitingLine } from "./git-waiting-line";
import { Logo } from "./logo";
import { SetupBreadcrumb } from "./setup-breadcrumb";
import { SetupNav } from "./setup-nav";
import { TopNav } from "./top-nav";

const heading = (text: string) => (
  <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">{text}</h1>
);

const now = "2026-10-21T12:00:00Z";
const claimPush = {
  path: "clusters/prod/team-a/camundaclusterclaims/orders.yaml",
  revision: "b",
  pushedAt: "2026-10-21T11:58:00Z",
};
const namespacePush = {
  path: "clusters/prod/_cluster/namespaces/team-a.yaml",
  revision: "a",
  pushedAt: "2026-10-21T11:58:00Z",
};

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
          <GitBannerView count={2}>
            <GitWaitingLine pushed={claimPush} now={now} />
            <GitWaitingLine pushed={namespacePush} now={now} />
          </GitBannerView>
        }
      >
        {heading("Fleet")}
      </AppShell>
    ),
  },
  {
    component: "GitBannerView",
    name: "One change waiting",
    render: () => (
      <GitBannerView count={1}>
        <GitWaitingLine pushed={claimPush} now={now} />
      </GitBannerView>
    ),
  },
  {
    component: "GitBannerView",
    name: "Nothing waiting",
    render: () => (
      <AppShell
        pathname="/fleet"
        banner={<GitBannerView count={0}>{null}</GitBannerView>}
      >
        {heading("Fleet")}
      </AppShell>
    ),
  },
];
