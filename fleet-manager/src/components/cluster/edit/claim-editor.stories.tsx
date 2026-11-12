import type { Story } from "@/gallery/registry";
import { ClaimEditorStory } from "./claim-editor-story";
import {
  WAITING_FOR_GITOPS,
  WAITING_ON_LIVE,
  WaitingPanel,
} from "./waiting-panel";

export const claimEditorStories: Story[] = [
  {
    component: "Claim editor",
    name: "Read view with Edit buttons",
    render: () => <ClaimEditorStory />,
  },
  {
    component: "Claim editor",
    name: "Basics in edit",
    render: () => (
      <ClaimEditorStory
        panel="basics"
        changes={[[[{ field: "spec" }, { field: "suspend" }], true]]}
      />
    ),
  },
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
