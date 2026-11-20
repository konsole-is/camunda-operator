"use client";

import { errorsAt } from "@/components/cluster/settings-editor";
import type { Step } from "@/lib/fields";
import { runningOfClaim } from "@/lib/planes/components";
import {
  type PlaneVersions,
  VERSION_LABEL,
  runningFields,
  versionChangeWarnings,
  versionMatrixWarning,
  versionWarnings,
} from "@/lib/planes/versions";
import { claimAfter } from "@/lib/session";
import { ClaimTextField, Notes } from "./fields";
import { usePlaneClaimEdit } from "./plane-claim-edit";

/**
 * VersionsPanel edits the version of each app that the plane runs after
 * the change. An error blocks Review. A version that moves down, skips a
 * minor, or leaves the apps on two minors gives a warning that does not
 * block. It renders nothing without a session.
 */
export function VersionsPanel() {
  const { edit } = usePlaneClaimEdit();
  const session = edit?.session;
  if (!edit || !session) return null;
  const after = claimAfter(session);
  const running = runningOfClaim(after.spec?.management ?? undefined);
  const before = (session.claim.spec?.versions ?? {}) as PlaneVersions;
  const versions = (after.spec?.versions ?? {}) as PlaneVersions;
  const matrix = versionMatrixWarning(versions, running);
  const warnings = [
    ...versionChangeWarnings(before, versions, running),
    ...(matrix ? [matrix] : []),
    ...versionWarnings(versions, running),
  ];
  return (
    <div className="flex min-w-0 flex-col gap-4">
      {runningFields(running).map((field) => {
        const steps: Step[] = [
          { field: "spec" },
          { field: "versions" },
          { field },
        ];
        const value = versions[field];
        return (
          <ClaimTextField
            key={field}
            label={`${VERSION_LABEL[field]} version`}
            value={typeof value === "string" ? value : ""}
            required
            errors={errorsAt(edit.errors, steps)}
            onChange={(text) =>
              text === "" ? edit.reset(steps) : edit.set(steps, text)
            }
          />
        );
      })}
      <Notes notes={[...new Set(warnings)]} />
    </div>
  );
}
