"use client";

import { useState } from "react";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { DeletionChoiceField } from "@/components/fleet/deletion-choice";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import { CLUSTER_STEPS, type ClusterDraft } from "@/lib/clusters/draft";
import {
  type StorageData,
  clusterDeletion,
  storageErrors,
  storageViews,
} from "@/lib/clusters/storage";
import type { BuiltinObjects, DeletionChoice } from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import { ClusterStorageStep } from "./cluster-storage-step";
import { useCreateFlow } from "./use-create-flow";

/**
 * StorageStory shows step 2 of the cluster create flow in the gallery. A
 * story passes no event handler, so this wrapper owns the draft and takes
 * only data. initial starts on step 2.
 */
export function StorageStory({
  sections,
  registrations,
  environment,
  objects = {},
  initial,
}: {
  sections: PluginSection[];
  registrations: PluginRegistration[];
  environment?: Environment;
  objects?: BuiltinObjects;
  initial: ClusterDraft;
}) {
  const data: StorageData = { environment, sections, registrations, objects };
  const [cardErrors, setCardErrors] = useState<
    Record<string, ValidationError[]>
  >({});
  const flow = useCreateFlow<ClusterDraft>({
    steps: CLUSTER_STEPS.filter((s) => s.id === "storage").map((s) => ({
      ...s,
      check: (draft: ClusterDraft) =>
        storageErrors(draft, data, cardErrors, []),
    })),
    initial,
  });
  return (
    <WizardStepper
      steps={CLUSTER_STEPS.map((s) => ({
        ...s,
        errors: s.id === "storage" ? flow.errors.storage : undefined,
      }))}
      current="storage"
    >
      <ClusterStorageStep
        draft={flow.draft}
        views={storageViews(flow.draft, data)}
        registrations={registrations}
        sections={sections}
        context={{ environment, objects }}
        hosts={[]}
        conflicts={[]}
        onSection={(name, selections, errors) => {
          flow.set(["storage", name], selections);
          setCardErrors((current) => ({ ...current, [name]: errors }));
        }}
        deletion={clusterDeletion(flow.draft, data)}
        onDeletion={(policy) =>
          flow.set(["claim", "spec", "deletionPolicy"], policy)
        }
      />
    </WizardStepper>
  );
}

/**
 * DeletionChoiceStory shows the deletion choice of step 2 in the gallery.
 * It owns the pick, so a story passes only choice.
 */
export function DeletionChoiceStory({ choice }: { choice: DeletionChoice }) {
  const [policy, setPolicy] = useState(choice.policy);
  return (
    <div className="max-w-3xl">
      <DeletionChoiceField
        subject="cluster"
        choice={{ ...choice, policy }}
        onChange={setPolicy}
      />
    </div>
  );
}
