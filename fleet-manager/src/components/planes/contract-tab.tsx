import Link from "next/link";
import type { ReactNode } from "react";
import { Term, Terms } from "@/components/cluster/claim-tab";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { Resource } from "@/lib/k8s/kinds";
import { UnreadNote } from "./unread-note";

const NOT_SET = <span className="text-muted">Not set</span>;

/**
 * ContractTab is the Contract tab of a plane. name is the contract that the
 * plane writes, undefined before it wrote one. contract is the
 * ManagementAuthConfig of that name, undefined when it is not in the list.
 * The client secret shows as a reference only. readers are the
 * CamundaOptimize objects that read the contract. unread names the kinds
 * that the page could not read. While unread is not empty, the tab never
 * says that the contract or its readers are missing.
 */
export function ContractTab({
  name,
  contract,
  readers,
  unread = [],
}: {
  name: string | undefined;
  contract: Resource<"ManagementAuthConfig"> | undefined;
  readers: readonly Resource<"CamundaOptimize">[];
  unread?: readonly string[];
}) {
  const sure = unread.length === 0;
  return (
    <section
      aria-label="Contract"
      className="flex flex-col gap-3 rounded-lg border border-border bg-panel p-4"
    >
      <h2 className="font-heading text-base font-semibold">Contract</h2>
      <UnreadNote kinds={unread} />
      {name === undefined ? (
        sure && (
          <p className="text-sm text-muted">
            This plane has not written its contract yet.
          </p>
        )
      ) : contract ? (
        <ContractTerms name={name} contract={contract} />
      ) : (
        sure && (
          <p className="text-sm text-muted">
            {`The contract ${name} does not exist.`}
          </p>
        )
      )}
      {name !== undefined && (
        <>
          <h3 className="font-heading text-sm font-semibold">Readers</h3>
          {readers.length === 0 ? (
            sure && (
              <p className="text-sm text-muted">
                No Optimize reads this contract.
              </p>
            )
          ) : (
            <Readers readers={readers} />
          )}
        </>
      )}
    </section>
  );
}

function ContractTerms({
  name,
  contract,
}: {
  name: string;
  contract: Resource<"ManagementAuthConfig">;
}) {
  const spec: unknown = contract.spec;
  const field = (key: string) => valueOf(spec, key);
  const secret = valueOf(spec, "clientSecretRef");
  const ref = (key: string) =>
    typeof secret === "object" && secret !== null
      ? valueOf(secret, key)
      : undefined;
  const backend = field("issuerBackendUrl");
  return (
    <Terms>
      <Term label="Name">{name}</Term>
      <Term label="Base URL">{shown(field("baseUrl"))}</Term>
      <Term label="Issuer URL">{shown(field("issuerUrl"))}</Term>
      <Term
        label="Issuer URL inside the cluster"
        note={
          backend === undefined || backend === ""
            ? "Optimize uses the issuer URL."
            : undefined
        }
      >
        {shown(backend)}
      </Term>
      <Term label="Authorization URL">{shown(field("authUrl"))}</Term>
      <Term label="Token URL">{shown(field("tokenUrl"))}</Term>
      <Term label="JWKS URL">{shown(field("jwksUrl"))}</Term>
      <Term label="Client ID">{shown(field("clientId"))}</Term>
      <Term label="Audience">{shown(field("audience"))}</Term>
      <Term label="Client secret namespace">{shown(ref("namespace"))}</Term>
      <Term label="Client secret name">{shown(ref("name"))}</Term>
      <Term label="Client secret key">{shown(ref("key"))}</Term>
    </Terms>
  );
}

function Readers({
  readers,
}: {
  readers: readonly Resource<"CamundaOptimize">[];
}) {
  return (
    <Table label="Readers table" aria-label="Readers">
      <TableHeader>
        <TableRow>
          <TableHead>Namespace</TableHead>
          <TableHead>Name</TableHead>
          <TableHead>Cluster</TableHead>
          <TableHead>External URL</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {readers.map((o) => {
          const namespace = o.metadata?.namespace ?? "";
          const cluster: unknown = o.spec?.clusterRef?.name;
          const url: unknown = o.spec?.externalUrl;
          return (
            <TableRow key={`${namespace}/${o.metadata?.name}`}>
              <TableCell className="text-text-2">{namespace}</TableCell>
              <th
                scope="row"
                className="px-2 py-2 text-left font-medium whitespace-nowrap text-text"
              >
                {o.metadata?.name}
              </th>
              <TableCell>
                {typeof cluster === "string" && cluster !== "" ? (
                  <Link
                    href={`/fleet/clusters/${encodeURIComponent(namespace)}/${encodeURIComponent(cluster)}`}
                    className="relative font-medium text-text hit-area hover:text-info"
                  >
                    {cluster}
                  </Link>
                ) : (
                  NOT_SET
                )}
              </TableCell>
              <TableCell className="min-w-48 break-all whitespace-normal">
                {shown(url)}
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}

function valueOf(object: unknown, key: string): unknown {
  return typeof object === "object" && object !== null
    ? (object as Record<string, unknown>)[key]
    : undefined;
}

// shown gives a string value as code text, Not set for an empty or missing
// value, and Not valid for a value of another type.
function shown(value: unknown): ReactNode {
  if (value === undefined || value === null || value === "") return NOT_SET;
  if (typeof value !== "string") {
    return <span className="text-bad">Not valid</span>;
  }
  return <code className="text-xs break-all text-text">{value}</code>;
}
