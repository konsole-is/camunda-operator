import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { AppShell } from "./app-shell";
import { claimPush, heading, namespacePush, now } from "./fixtures";
import { GitBannerView } from "./git-banner";
import { GitWaitingLine } from "./git-waiting-line";
import { SetupBreadcrumb } from "./setup-breadcrumb";
import { SetupNav } from "./setup-nav";

const longRelease =
  "/setup/releases/camunda-8-9-long-term-support-with-extended-security-patches-eu-north-1";

const meta = {
  title: "fleet/AppShell",
  component: AppShell,
  args: { pathname: "/fleet", children: heading("Fleet") },
} satisfies Meta<typeof AppShell>;
export default meta;
type Story = StoryObj<typeof meta>;

export const FleetPage: Story = { name: "Fleet page" };

export const SetupDetailPage: Story = {
  name: "Setup detail page",
  args: {
    pathname: "/setup/presets/gold",
    sideNav: <SetupNav pathname="/setup/presets/gold" />,
    children: (
      <>
        <SetupBreadcrumb pathname="/setup/presets/gold" />
        {heading("gold")}
      </>
    ),
  },
};

export const SetupDetailWithALongName: Story = {
  name: "Setup detail with a long name",
  args: {
    pathname: "/setup/releases/x",
    sideNav: <SetupNav pathname="/setup/releases/x" />,
    children: (
      <>
        <SetupBreadcrumb pathname={longRelease} />
        {heading("Release")}
      </>
    ),
  },
};

export const WithGitWaitingBanner: Story = {
  name: "With Git waiting banner",
  args: {
    banner: (
      <GitBannerView count={2}>
        <GitWaitingLine pushed={claimPush} now={now} />
        <GitWaitingLine pushed={namespacePush} now={now} />
      </GitBannerView>
    ),
  },
};
