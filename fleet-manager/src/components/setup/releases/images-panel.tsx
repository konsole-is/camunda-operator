"use client";

import type { ReactNode } from "react";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { ReleaseImageRow } from "./images";

type PinField = "camunda" | "connectors";

const PIN_LABELS: Record<PinField, string> = {
  camunda: "Camunda",
  connectors: "Connectors",
};

/**
 * ImagesPanel shows each image of a release with the image that the
 * release pins, from pinned, and the default for the version. Only the
 * Camunda and the Connectors image can be pinned: onPin gets the field and
 * the typed reference, empty for unset. Without rows, the images table has
 * no entry for minor, and the panel still offers the two pins.
 */
export function ImagesPanel({
  rows,
  minor,
  pinned = {},
  onPin,
}: {
  rows: ReleaseImageRow[] | undefined;
  minor: string | undefined;
  pinned?: Partial<Record<PinField, string>>;
  onPin: (field: PinField, value: string) => void;
}): ReactNode {
  const pin = (field: PinField, value: string | undefined) => (
    <Input
      aria-label={`Pinned ${PIN_LABELS[field]} image`}
      placeholder="Not pinned"
      spellCheck={false}
      autoComplete="off"
      className="min-w-48 font-mono text-xs"
      value={value ?? ""}
      onChange={(event) => onPin(field, event.target.value)}
    />
  );

  if (!rows) {
    return (
      <div className="flex flex-col gap-3">
        <p className="text-sm text-muted">
          {`The images table has no entry for Camunda ${minor ?? "this version"}. Pin the images here, or set them in the platform config.`}
        </p>
        {(["camunda", "connectors"] as const).map((field) => (
          <div key={field} className="flex max-w-xl flex-col gap-1.5">
            <span className="text-sm font-medium text-text">
              {PIN_LABELS[field]}
            </span>
            {pin(field, pinned[field])}
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <Table label="Images of the release">
        <TableHeader>
          <TableRow>
            <TableHead>Image</TableHead>
            <TableHead>Pinned by the release</TableHead>
            <TableHead>Default for the version</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.image}>
              <TableCell className="font-medium">{row.label}</TableCell>
              <TableCell>
                {row.pinField ? (
                  pin(row.pinField, pinned[row.pinField])
                ) : row.image === "optimize" ? (
                  <span className="text-sm text-muted">
                    The release pins no Optimize image
                  </span>
                ) : (
                  <span className="text-muted">-</span>
                )}
              </TableCell>
              <TableCell>
                <DefaultImage row={row} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <p className="text-xs text-muted">
        The repository is the default of the images table. The platform config
        of each cluster sets the repository it pulls from.
      </p>
    </div>
  );
}

function DefaultImage({ row }: { row: ReleaseImageRow }) {
  if (row.tag === undefined)
    return (
      <span className="text-sm text-muted">
        Each cluster sets its own connectors version
      </span>
    );
  if (row.repository === undefined)
    return (
      <span className="flex flex-col">
        <span className="font-mono text-xs">{row.tag}</span>
        <span className="text-xs text-muted">Picked by its operator</span>
      </span>
    );
  return (
    <span className="font-mono text-xs break-all">{`${row.repository}:${row.tag}`}</span>
  );
}
