import { ReviewPanel } from "@/components/fleet/review-panel";
import type { Story } from "@/gallery/registry";
import { objectDiff } from "@/lib/diff";
import { BackupReviewStory } from "./backup-review-story";

export const backupReviewStories: Story[] = [
  {
    component: "ClaimReview",
    name: "Backup caution",
    render: () => <BackupReviewStory change="rolls" mode="direct" />,
  },
  {
    component: "ClaimReview",
    name: "Backup caution in Git mode",
    render: () => <BackupReviewStory change="rolls" mode="git" />,
  },
  {
    component: "ClaimReview",
    name: "Backup does not roll",
    render: () => <BackupReviewStory change="keeps" mode="direct" />,
  },
  {
    component: "ReviewPanel",
    name: "Typed confirm",
    render: () => (
      <ReviewPanel
        lines={objectDiff(
          { spec: { cluster: { zeebe: { replicas: 3 } } } },
          { spec: { cluster: { zeebe: { replicas: 5 } } } },
        )}
        mode="direct"
        confirmLabel="Change anyway"
        danger
        typed="orders"
        effects={["Broker pods restart."]}
      />
    ),
  },
];
