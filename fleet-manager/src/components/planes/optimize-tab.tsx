import { NotRead, notReadText } from "@/components/fleet/not-read";
import type { ConditionLike } from "@/components/fleet/status";
import { StatusChip } from "@/components/fleet/status-chip";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { OptimizeRow } from "./plane-optimize";

/**
 * OptimizeTab is the Optimize tab of a plane: the state of callbacks once,
 * then one row for each Optimize whose login callback the plane needs. In
 * the oidc mode it tells the reader to add the callbacks to the identity
 * provider. unread names the kinds that the page could not read. While
 * unread is not empty, the tab never says that there is no Optimize.
 */
export function OptimizeTab({
  rows,
  callbacks,
  oidc,
  unread = [],
}: {
  rows: OptimizeRow[];
  callbacks: ConditionLike | undefined;
  oidc: boolean;
  unread?: readonly string[];
}) {
  return (
    <section
      aria-label="Optimize"
      className="flex flex-col gap-3 rounded-lg border border-border bg-panel p-4"
    >
      <h2 className="font-heading text-base font-semibold">Optimize</h2>
      <div className="flex flex-col gap-1 text-sm">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-muted">Callbacks</span>
          <StatusChip condition={callbacks} readyReason />
        </div>
        {callbacks?.message && (
          <p className="text-text-2">{callbacks.message}</p>
        )}
      </div>
      {oidc && (
        <p className="text-sm text-text">
          Add the callbacks to your identity provider.
        </p>
      )}
      {rows.length === 0 ? (
        <p className="text-sm text-muted">
          {unread.length > 0
            ? notReadText(unread, true)
            : "No Optimize with an external URL reads the contract of this plane."}
        </p>
      ) : (
        <>
          <NotRead kinds={unread} tab />
          <Table label="Optimize table" aria-label="Optimize">
            <TableHeader>
              <TableRow>
                <TableHead>Namespace</TableHead>
                <TableHead>Name</TableHead>
                <TableHead>External URL</TableHead>
                <TableHead>Callback URL</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TableRow key={`${row.namespace}/${row.name}`}>
                  <TableCell className="text-text-2">{row.namespace}</TableCell>
                  <th
                    scope="row"
                    className="px-2 py-2 text-left font-medium whitespace-nowrap text-text"
                  >
                    {row.name}
                  </th>
                  <TableCell className="min-w-48 break-all whitespace-normal">
                    <ExternalUrl url={row.externalUrl} />
                  </TableCell>
                  <TableCell className="min-w-48 break-all whitespace-normal">
                    {row.callbackUrl === undefined ? (
                      <span className="text-bad">Not a valid URL</span>
                    ) : (
                      <code className="text-xs text-text">
                        {row.callbackUrl}
                      </code>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </>
      )}
    </section>
  );
}

function ExternalUrl({ url }: { url: string | undefined }) {
  if (url === undefined) {
    return <span className="text-bad">Not a valid URL</span>;
  }
  if (/^https?:\/\//.test(url)) {
    return (
      <a
        href={url}
        target="_blank"
        rel="noreferrer"
        className="relative inline-block text-info underline-offset-4 hit-area hover:underline"
      >
        {url}
      </a>
    );
  }
  return <code className="text-xs text-text">{url}</code>;
}
