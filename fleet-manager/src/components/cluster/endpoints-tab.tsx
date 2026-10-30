import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { EndpointRow } from "./endpoints";

/** EndpointsTab is the content of the Endpoints tab: one row for each route. */
export function EndpointsTab({ rows }: { rows: EndpointRow[] }) {
  return (
    <section
      aria-label="Endpoints"
      className="flex flex-col gap-3 rounded-lg border border-border bg-panel p-4"
    >
      <h2 className="font-heading text-base font-semibold">Endpoints</h2>
      {rows.length === 0 ? (
        <p className="text-sm text-muted">
          This cluster reports no endpoints yet.
        </p>
      ) : (
        <Table label="Endpoints table" aria-label="Endpoints">
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Public URL</TableHead>
              <TableHead>Service</TableHead>
              <TableHead>Protocol</TableHead>
              <TableHead>Served by</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={row.key}>
                <th
                  scope="row"
                  className="px-2 py-2 text-left font-medium whitespace-nowrap text-text"
                >
                  {row.name}
                </th>
                <TableCell className="break-all whitespace-normal">
                  <PublicUrl row={row} />
                </TableCell>
                <TableCell>
                  <code className="text-xs text-text-2">
                    {row.service}:{row.port}
                  </code>
                </TableCell>
                <TableCell className="text-text-2">{row.protocol}</TableCell>
                <TableCell className="text-text-2">
                  {row.servedBy ? (
                    <>
                      <span>{row.servedBy.label}</span>
                      {row.servedBy.embedded && (
                        <span className="ml-1 text-muted">(embedded)</span>
                      )}
                    </>
                  ) : (
                    "-"
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </section>
  );
}

function PublicUrl({ row }: { row: EndpointRow }) {
  if (row.url === undefined) {
    return <span className="text-muted">No external URL</span>;
  }
  if (/^https?:\/\//.test(row.url)) {
    return (
      <a
        href={row.url}
        target="_blank"
        rel="noreferrer"
        className="text-info underline-offset-4 hover:underline"
      >
        {row.url}
      </a>
    );
  }
  return <code className="text-xs text-text">{row.url}</code>;
}
