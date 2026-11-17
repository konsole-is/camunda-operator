import Link from "next/link";
import type { ReactNode } from "react";
import { Panel, Term, Terms } from "@/components/cluster/claim-tab";
import { LimitsList } from "@/components/fleet/limits-list";
import { NotRead } from "@/components/fleet/not-read";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { LINK, ValueError } from "./tab-parts";
import { isRecord, isText, listOf } from "./values";

type Limit = { label: string; value: string; description?: string };

/**
 * AboutTab shows what registration is: its description, its documentation,
 * its controller and the other options of that controller in
 * registrations, its ServiceAccount, its mode, and the limits set at
 * install. unread holds the kinds that are not read: while
 * PluginRegistration is in it, the other options are not known.
 */
export function AboutTab({
  registration,
  registrations,
  unread,
}: {
  registration: PluginRegistration;
  registrations: readonly PluginRegistration[];
  unread: readonly string[];
}): ReactNode {
  const { spec } = registration;
  const controller = isRecord(spec.controller) ? spec.controller : undefined;
  const account = isRecord(spec.serviceAccount)
    ? spec.serviceAccount
    : undefined;
  const others = controller
    ? registrations.filter(
        (r) =>
          r.metadata.name !== registration.metadata.name &&
          r.spec.controller?.name === controller.name,
      )
    : [];
  return (
    <div className="flex flex-col gap-4">
      <Panel label="Details">
        <Terms>
          <Term label="Description">
            {isText(spec.description) ? spec.description : <Muted>None</Muted>}
          </Term>
          <Term label="Documentation">
            {isText(spec.docsUrl) ? (
              <a
                href={spec.docsUrl}
                target="_blank"
                rel="noreferrer"
                className={`${LINK} inline-block max-w-full break-all`}
              >
                {spec.docsUrl}
              </a>
            ) : (
              <Muted>None</Muted>
            )}
          </Term>
          <Term label="Controller">
            {controller && isText(controller.name) ? (
              [controller.name, controller.version].filter(Boolean).join(" ")
            ) : (
              <Muted>Not named</Muted>
            )}
          </Term>
          <Term label="ServiceAccount">
            {account && isText(account.namespace) && isText(account.name) ? (
              <span className="font-mono">{`${account.namespace}/${account.name}`}</span>
            ) : (
              <Muted>Not named</Muted>
            )}
          </Term>
          <Term label="Mode">{isText(spec.mode) ? spec.mode : "-"}</Term>
          <Term label="Other options of this controller">
            {unread.includes("PluginRegistration") ? (
              <NotRead kinds={["PluginRegistration"]} />
            ) : others.length === 0 ? (
              <Muted>None</Muted>
            ) : (
              <ul className="flex flex-col gap-1">
                {others.map((r) => (
                  <li key={r.metadata.name}>
                    <Link
                      href={`/setup/plugins/${encodeURIComponent(r.metadata.name ?? "")}`}
                      className={`${LINK} font-mono`}
                    >
                      {r.metadata.name}
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Term>
        </Terms>
      </Panel>
      <Panel label="Limits set at install">
        <Limits value={spec.limits} />
      </Panel>
    </div>
  );
}

function Limits({ value }: { value: unknown }) {
  const items = listOf(value);
  if (!items) return <ValueError text="The limits are not a list." />;
  const bad = items.findIndex(
    (l) =>
      !isRecord(l) ||
      !isText(l.label) ||
      !isText(l.value) ||
      !isText(l.description, true),
  );
  if (bad >= 0)
    return <ValueError text={`Limit ${bad + 1} is not a valid limit.`} />;
  if (items.length === 0)
    return (
      <p className="text-sm text-muted">
        This option sets no limits at install.
      </p>
    );
  return (
    <>
      <LimitsList limits={items as Limit[]} />
      <p className="text-xs text-muted">
        No environment and no claim can change these limits.
      </p>
    </>
  );
}

function Muted({ children }: { children: ReactNode }) {
  return <span className="text-muted">{children}</span>;
}
