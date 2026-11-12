import type { Story } from "@/gallery/registry";
import {
  WAITING_FOR_GITOPS,
  WAITING_ON_LIVE,
  WaitingPanel,
} from "./waiting-panel";

export const claimEditorStories: Story[] = [
  {
    component: "WaitingPanel",
    name: "A push of this page",
    render: () => (
      <div className="max-w-3xl">
        <WaitingPanel text={WAITING_FOR_GITOPS} onStopWaiting={() => {}} />
      </div>
    ),
  },
  {
    component: "WaitingPanel",
    name: "A push of an earlier visit",
    render: () => (
      <div className="max-w-3xl">
        <WaitingPanel text={WAITING_ON_LIVE} onStopWaiting={() => {}} />
      </div>
    ),
  },
];
