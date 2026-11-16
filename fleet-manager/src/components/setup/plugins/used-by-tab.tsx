import Link from "next/link";
import type { ReactNode } from "react";
import { Panel, Term, Terms } from "@/components/cluster/claim-tab";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { ConditionBadge } from "./condition-badge";
import { LINK, NotKnown, ValueError } from "./tab-parts";
import type { UsedByRow } from "./used-by";

const MUTED = "block text-xs text-muted";

/**
 * UsedByTab shows each request of rows in its own panel: its claim, its
 * environment, OutputsReady and Ready, its headline, its resources, its
 * Grants, and its deletion policy. unread holds the kinds that are not
 * read: with PluginRequest in it, the tab never says that no claim uses the
 * option, and shows no condition and no headline of a request.
 */
export function UsedByTab({
  rows,
  unread,
}: {
  rows: UsedByRow[];
  unread: readonly string[];
}): ReactNode {
  const requestsUnread = unread.includes("PluginRequest");
  if (rows.length === 0)
    return requestsUnread ? (
      <NotKnown kinds={["PluginRequest"]} />
    ) : (
      <p className="text-sm text-muted">No claim uses this option.</p>
    );
  const environmentKinds = ["Namespace", "Environment"].filter((k) =>
    unread.includes(k),
  );
  return (
    <div className="flex flex-col gap-4">
      {requestsUnread && (
        <p className="text-sm text-muted">
          The list can miss requests: PluginRequest is not read.
        </p>
      )}
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        {rows.map((row) => (
          <Panel
            key={`${row.namespace}/${row.name}`}
            label={`${row.namespace}/${row.name}`}
          >
            <Terms>
              <ClaimTerm row={row} />
              <Term label="Environment">
                {row.environment ?? <NotKnown kinds={environmentKinds} />}
              </Term>
              {row.error ? (
                <Term label="Conditions">
                  <ValueError text={row.error} />
                </Term>
              ) : (
                <>
                  <Term label="Outputs">
                    <Condition
                      type="OutputsReady"
                      condition={row.outputsReady}
                      unknown={row.healthUnknown}
                    />
                  </Term>
                  <Term label="Ready">
                    <Condition
                      type="Ready"
                      condition={row.ready}
                      unknown={row.healthUnknown}
                    />
                    {row.ready?.reason === "GrantRejected" &&
                      (row.grantsNotReady ? (
                        row.grantsNotReady.length > 0 && (
                          <span className="block text-xs text-warn">
                            {`Grants with Ready False: ${row.grantsNotReady.join(", ")}`}
                          </span>
                        )
                      ) : (
                        <NotKnown kinds={["Grant"]} />
                      ))}
                  </Term>
                </>
              )}
              <Term label="Headline">
                {row.healthUnknown ? (
                  <NotKnown kinds={["PluginRequest"]} />
                ) : !row.headline ? (
                  <span className="text-muted">None</span>
                ) : "error" in row.headline ? (
                  <ValueError text={row.headline.error} />
                ) : (
                  `${row.headline.label}: ${row.headline.value}`
                )}
              </Term>
              <Term label="Resources">
                {row.resources.length === 0 ? (
                  <span className="text-muted">None reported</span>
                ) : (
                  <ul className="flex flex-col gap-1.5">
                    {row.resources.map((resource, i) =>
                      "error" in resource ? (
                        <li key={i}>
                          <ValueError text={resource.error} />
                        </li>
                      ) : (
                        <li key={`${resource.kind} ${resource.name} ${i}`}>
                          <span className="block font-mono break-all">
                            {`${resource.kind} ${resource.name}`}
                          </span>
                          {(resource.role || resource.location) && (
                            <span className={MUTED}>
                              {[resource.role, resource.location]
                                .filter(Boolean)
                                .join(", ")}
                            </span>
                          )}
                          <span className={MUTED}>
                            {`On Orphan: ${resource.onOrphan}`}
                          </span>
                        </li>
                      ),
                    )}
                  </ul>
                )}
              </Term>
              <Term label="Grants">
                {!row.grants ? (
                  <NotKnown kinds={["Grant"]} />
                ) : row.grants.length === 0 ? (
                  <span className="text-muted">None</span>
                ) : (
                  <ul className="flex flex-col gap-1.5">
                    {row.grants.map((grant) => (
                      <li key={grant.name} className="flex flex-col gap-0.5">
                        <span className="font-mono break-all">
                          {grant.name}
                        </span>
                        {grant.error ? (
                          <ValueError text={grant.error} />
                        ) : (
                          <ConditionBadge
                            type="Ready"
                            condition={grant.ready}
                          />
                        )}
                      </li>
                    ))}
                  </ul>
                )}
              </Term>
              <Term label="Deletion policy">{row.deletionPolicy}</Term>
            </Terms>
          </Panel>
        ))}
      </div>
    </div>
  );
}

function ClaimTerm({ row }: { row: UsedByRow }) {
  const plane = row.claim.kind === "CamundaManagementClusterClaim";
  const href = `/fleet/${plane ? "planes" : "clusters"}/${encodeURIComponent(row.namespace)}/${encodeURIComponent(row.target.name)}`;
  return (
    <Term label={plane ? "Management plane" : "Cluster"}>
      <Link href={href} className={`${LINK} font-mono`}>
        {row.claim.name}
      </Link>
    </Term>
  );
}

// Condition shows the message of a condition that is not True as text, so
// that it does not hide in the hover text of the badge.
function Condition({
  type,
  condition,
  unknown,
}: {
  type: string;
  condition?: ConditionRow;
  unknown?: boolean;
}) {
  return (
    <>
      <ConditionBadge type={type} condition={condition} unknown={unknown} />
      {!unknown && condition?.status !== "True" && condition?.message && (
        <span className="mt-1 block text-xs break-words text-text-2">
          {condition.message}
        </span>
      )}
    </>
  );
}
