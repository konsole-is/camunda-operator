import type { ReactNode } from "react";
import { age } from "@/components/fleet/age";
import { ConditionList } from "@/components/fleet/condition-list";
import { DELETION_LABELS } from "@/components/fleet/deletion-choice";
import { StatusChip } from "@/components/fleet/status-chip";
import { ConditionBadge } from "@/components/setup/plugins/condition-badge";
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { Term, Terms } from "./claim-tab";
import { SeverityMark } from "./component-card";
import { DetailSection } from "./detail-section";
import { NotRead } from "./not-read";
import type {
  ConnectedRef,
  GrantRow,
  Invalid,
  RequestDetail,
  SummaryRow,
} from "./requests";

/** GRANT_CONDITIONS are the condition types of a Grant, in the order its status lists them. */
const GRANT_CONDITIONS = ["Applied", "MatchesTemplate", "Ready"];

/**
 * RequestOverview shows detail below the conditions of a request: one box
 * for each block that has something to show. unread holds the kinds that
 * the page could not read or that are out of date. A box that depends on
 * one of them names the kind, and shows no fate, no condition, and no
 * empty list from it: CamundaClusterClaim for the fates, Grant for the
 * grants, PluginRegistration for the inputs and the users, and the kind of
 * each connected contract for its Ready. now is the time that the ages
 * count to.
 */
export function RequestOverview({
  detail,
  unread,
  now,
}: {
  detail: RequestDetail;
  unread: readonly string[];
  now: string;
}) {
  const unreadKind = (kind: string) => unread.includes(kind);
  const registrationUnread = unreadKind("PluginRegistration");
  const grantsUnread = unreadKind("Grant");
  const showGrants =
    detail.grants.length > 0 || (grantsUnread && detail.clusterGrants);
  return (
    <div className="flex flex-col gap-4">
      {detail.summary.length > 0 && (
        <DetailSection title="Summary" region>
          <Summary rows={detail.summary} />
          {detail.summaryTime && (
            <p className="text-xs text-muted">
              Updated {age(detail.summaryTime, now)} ago
            </p>
          )}
        </DetailSection>
      )}
      {detail.connected && (
        <DetailSection title="Connected" region>
          <Connected refs={detail.connected} unread={unread} />
        </DetailSection>
      )}
      {detail.connectors && (
        <DetailSection title="Principal" region>
          <Terms>
            <Term label="Principal">
              <span className="font-mono text-xs break-all">
                {detail.connectors.principal ?? "Not written yet"}
              </span>
            </Term>
            <Term label="ServiceAccount">
              <span className="font-mono text-xs break-all">
                {detail.connectors.serviceAccount}
              </span>
            </Term>
          </Terms>
        </DetailSection>
      )}
      {showGrants && (
        <DetailSection
          title="Grants"
          note={detail.clusterGrants ? "Every grant of the cluster" : undefined}
          region
        >
          {grantsUnread && <NotRead kinds={["Grant"]} />}
          <Grants
            grants={detail.grants}
            withRequest={detail.clusterGrants}
            unread={grantsUnread}
            now={now}
          />
        </DetailSection>
      )}
      {detail.outputs.length > 0 && (
        <DetailSection title="Outputs" region>
          <Outputs detail={detail} unread={unreadKind("PluginSection")} />
        </DetailSection>
      )}
      {(detail.inputs.length > 0 || registrationUnread) && (
        <DetailSection title="Inputs" region>
          {registrationUnread && <NotRead kinds={["PluginRegistration"]} />}
          {detail.inputs.length > 0 && <Inputs detail={detail} />}
        </DetailSection>
      )}
      {(detail.usedBy.length > 0 || registrationUnread) && (
        <DetailSection title="Used by" region>
          {registrationUnread && <NotRead kinds={["PluginRegistration"]} />}
          {detail.usedBy.length > 0 && (
            <ul className="flex flex-wrap gap-1.5">
              {detail.usedBy.map((label) => (
                <li
                  key={label}
                  className="rounded border border-border px-2 py-0.5 text-sm text-text"
                >
                  {label}
                </li>
              ))}
            </ul>
          )}
        </DetailSection>
      )}
      {detail.links.length > 0 && (
        <DetailSection title="Links" region>
          <ul className="flex flex-col gap-2">
            {detail.links.map((link, i) => (
              <li key={i} className="text-sm">
                {"error" in link ? (
                  <RowError row={link} />
                ) : (
                  <a
                    href={link.url}
                    target="_blank"
                    rel="noreferrer"
                    className="relative font-medium text-text underline underline-offset-4 hit-area hover:text-accent"
                  >
                    {link.name}
                  </a>
                )}
              </li>
            ))}
          </ul>
        </DetailSection>
      )}
      {detail.resources.length > 0 && (
        <DetailSection title="Resources" region>
          <Resources
            detail={detail}
            claimUnread={unreadKind("CamundaClusterClaim")}
          />
        </DetailSection>
      )}
    </div>
  );
}

function Summary({ rows }: { rows: readonly (SummaryRow | Invalid)[] }) {
  return (
    <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-2 text-sm">
      {rows.map((row, i) =>
        "error" in row ? (
          <dd key={i} className="col-span-2">
            <RowError row={row} />
          </dd>
        ) : (
          <SummaryItem key={i} row={row} />
        ),
      )}
    </dl>
  );
}

function SummaryItem({ row }: { row: SummaryRow }) {
  const severity = row.severity === "Info" ? undefined : row.severity;
  const tone =
    severity === "Error"
      ? "text-bad"
      : severity === "Warning"
        ? "text-warn"
        : "";
  return (
    <>
      <dt className="text-muted">{row.label}</dt>
      <dd className={cn("min-w-0 break-words", tone)}>
        {severity && <SeverityMark severity={severity} />}
        <span className={cn("text-text", tone)}>{row.value}</span>
      </dd>
    </>
  );
}

function Connected({
  refs,
  unread,
}: {
  refs: readonly ConnectedRef[];
  unread: readonly string[];
}) {
  return (
    <table className="w-full text-sm">
      <TableHeader>
        <TableRow>
          <Head>Kind</Head>
          <Head>Name</Head>
          <Head>Ready</Head>
        </TableRow>
      </TableHeader>
      <TableBody>
        {refs.map((ref) => (
          <TableRow key={`${ref.kind}/${ref.name}`}>
            <Cell>{ref.kind}</Cell>
            <Cell mono>{ref.name}</Cell>
            <Cell>
              {unread.includes(ref.kind) ? (
                <ConditionBadge type="Ready" unknown />
              ) : ref.found ? (
                <StatusChip condition={ref.ready} />
              ) : (
                <span className="text-warn">Not in the namespace</span>
              )}
            </Cell>
          </TableRow>
        ))}
      </TableBody>
    </table>
  );
}

function Grants({
  grants,
  withRequest,
  unread,
  now,
}: {
  grants: readonly GrantRow[];
  withRequest: boolean;
  unread: boolean;
  now: string;
}) {
  if (grants.length === 0) return null;
  return (
    <ul className="flex flex-col divide-y divide-border">
      {grants.map((grant) => (
        <li
          key={grant.name}
          className="flex min-w-0 flex-col gap-2 py-3 first:pt-0 last:pb-0"
        >
          <span className="font-mono text-sm break-all text-text">
            {grant.name}
          </span>
          <Terms>
            <Term label="Template">{grant.template || "-"}</Term>
            {withRequest && (
              <Term label="Request">
                <span className="font-mono break-all">
                  {grant.request ?? "-"}
                </span>
              </Term>
            )}
          </Terms>
          {!unread && <GrantStateLine grant={grant} />}
          {grant.errors.map((error) => (
            <p key={error} className="text-sm text-bad">
              {error}
            </p>
          ))}
          {grant.statement !== undefined && (
            <pre
              aria-label={`Statement of ${grant.name}`}
              tabIndex={0}
              className="overflow-x-auto rounded-md bg-panel-2 p-3 text-xs outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              <code>{grant.statement}</code>
            </pre>
          )}
          {unread ? (
            <span className="flex flex-wrap gap-1.5">
              {GRANT_CONDITIONS.map((type) => (
                <ConditionBadge key={type} type={type} unknown />
              ))}
            </span>
          ) : (
            <ConditionList
              label={`Conditions of ${grant.name}`}
              conditions={grant.conditions}
              now={now}
            />
          )}
        </li>
      ))}
    </ul>
  );
}

// GrantStateLine says in words what the conditions of a Grant that is not
// Ready mean. A Grant whose template changed still holds the permission of
// the old template, so it is not refused.
function GrantStateLine({ grant }: { grant: GrantRow }) {
  switch (grant.state) {
    case "oldTemplate":
      return (
        <p className="text-sm text-text-2">
          Applied with the old template. The controller of the option writes the
          Grant again from the new template.
        </p>
      );
    case "refused":
      return (
        <p className="text-sm text-warn">Refused. The conditions say why.</p>
      );
    case "waiting":
      return <p className="text-sm text-muted">Not applied yet.</p>;
    default:
      return null;
  }
}

function Outputs({
  detail,
  unread,
}: {
  detail: RequestDetail;
  unread: boolean;
}) {
  return (
    <table className="w-full text-sm">
      <TableHeader>
        <TableRow>
          <Head>Output</Head>
          <Head>Value</Head>
          <Head>Fills</Head>
        </TableRow>
      </TableHeader>
      <TableBody>
        {detail.outputs.map((row, i) =>
          "error" in row ? (
            <ErrorRow key={i} row={row} span={3} />
          ) : (
            <TableRow key={row.key}>
              <Cell mono>{row.key}</Cell>
              <Cell mono>{row.value}</Cell>
              <Cell>
                {unread
                  ? "Not known"
                  : row.fills.length > 0
                    ? row.fills.join(", ")
                    : "No field"}
              </Cell>
            </TableRow>
          ),
        )}
      </TableBody>
    </table>
  );
}

function Inputs({ detail }: { detail: RequestDetail }) {
  return (
    <table className="w-full text-sm">
      <TableHeader>
        <TableRow>
          <Head>Input</Head>
          <Head>From</Head>
          <Head>Value</Head>
        </TableRow>
      </TableHeader>
      <TableBody>
        {detail.inputs.map((input) => (
          <TableRow key={input.name}>
            <Cell mono>{input.name}</Cell>
            <Cell>{`${input.sectionLabel} / ${input.output}`}</Cell>
            <Cell mono={input.value !== undefined}>
              {input.value ?? "Waiting"}
            </Cell>
          </TableRow>
        ))}
      </TableBody>
    </table>
  );
}

function Resources({
  detail,
  claimUnread,
}: {
  detail: RequestDetail;
  claimUnread: boolean;
}) {
  const fateOf = (fate: string | undefined) =>
    claimUnread
      ? "Not known"
      : detail.controlled
        ? (fate ?? "-")
        : "Not controlled by this claim";
  return (
    <>
      <p className="text-xs text-muted">
        {claimUnread
          ? "Not known: the page could not read CamundaClusterClaim, so it cannot say what happens to each resource when the claim is deleted."
          : detail.controlled
            ? `When the claim is deleted: ${DELETION_LABELS[detail.policy]}.`
            : "The claim does not control this request, so its deletion does not decide what happens to these resources."}
      </p>
      <table className="w-full text-sm">
        <TableHeader>
          <TableRow>
            <Head>Kind</Head>
            <Head>Name</Head>
            <Head>ID</Head>
            <Head>Location</Head>
            <Head>Role</Head>
            <Head>After the claim is deleted</Head>
          </TableRow>
        </TableHeader>
        <TableBody>
          {detail.resources.map((row, i) =>
            "error" in row ? (
              <ErrorRow key={i} row={row} span={6} />
            ) : (
              <TableRow key={`${row.kind}/${row.name}`}>
                <Cell>{row.kind}</Cell>
                <Cell mono>{row.name}</Cell>
                <Cell mono>{row.id ?? "-"}</Cell>
                <Cell>{row.location ?? "-"}</Cell>
                <Cell>{row.role ?? "-"}</Cell>
                <Cell>{fateOf(row.fate)}</Cell>
              </TableRow>
            ),
          )}
        </TableBody>
      </table>
    </>
  );
}

// Cell wraps a long value at any character, so that a table fits a narrow
// screen without a sideways scroll.
function Cell({
  mono = false,
  children,
}: {
  mono?: boolean;
  children: ReactNode;
}) {
  return (
    <TableCell
      className={cn(
        "align-top whitespace-normal wrap-anywhere",
        mono && "font-mono text-xs",
      )}
    >
      {children}
    </TableCell>
  );
}

// Head wraps the words of a long column name, so that a table fits a
// narrow screen.
function Head({ children }: { children: ReactNode }) {
  return (
    <TableHead className="h-auto py-2 align-bottom whitespace-normal">
      {children}
    </TableHead>
  );
}

function ErrorRow({ row, span }: { row: Invalid; span: number }) {
  return (
    <TableRow>
      <TableCell colSpan={span} className="whitespace-normal">
        <RowError row={row} />
      </TableCell>
    </TableRow>
  );
}

function RowError({ row }: { row: Invalid }) {
  return <span className="text-sm text-bad">{row.error}</span>;
}
