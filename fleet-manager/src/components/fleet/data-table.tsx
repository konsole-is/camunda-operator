"use client";

import { ArrowDown, ArrowUp } from "lucide-react";
import Link from "next/link";
import { type ReactNode, useEffect, useId, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export type DataTableColumn = {
  key: string;
  label: string;
  sortable?: boolean;
};
export type DataTableFilter = {
  key: string;
  label: string;
  options: { value: string; label: string }[];
};

/**
 * DataTableRow is one row. cells holds the content of each column by key.
 * search is the text that the search box matches. sort holds the value of
 * each sortable column. filters holds the value of each filter. applicable
 * says whether the row can be selected, true when it is not set.
 */
export type DataTableRow = {
  id: string;
  label?: string;
  cells: Record<string, ReactNode>;
  href?: string;
  search?: string;
  sort?: Record<string, string | number>;
  filters?: Record<string, string>;
  applicable?: boolean;
};

type Sort = { key: string; direction: "ascending" | "descending" };

/**
 * DataTable is a table with search, filters, sort, and pages. The first cell
 * of a row with href links to it. With selection, each row has a checkbox
 * and a button "Select all <applicableLabel>" selects every applicable row
 * that the search and the filters show, on every page. Nothing is selected
 * at first. A row that a new search or filter hides, that new rows drop,
 * or that new rows mark as not applicable leaves the selection.
 * onChange gets the selected ids in row order after each change.
 */
export function DataTable({
  label,
  columns,
  rows,
  filters = [],
  pageSize = 25,
  selection,
  emptyText = "No rows match the search and the filters.",
}: {
  label: string;
  columns: DataTableColumn[];
  rows: DataTableRow[];
  filters?: DataTableFilter[];
  pageSize?: number;
  selection?: { applicableLabel: string; onChange?: (ids: string[]) => void };
  emptyText?: string;
}) {
  const id = useId();
  const [query, setQuery] = useState("");
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});
  // A value counts only while the filters still offer it, because the user
  // cannot clear a filter that is gone.
  const activeFilters = Object.fromEntries(
    Object.entries(filterValues).filter(([key, value]) =>
      filters.some(
        (f) => f.key === key && f.options.some((o) => o.value === value),
      ),
    ),
  );
  const [sort, setSort] = useState<Sort | undefined>();
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [seenRows, setSeenRows] = useState(rows);

  const matches = (q: string, values: Record<string, string>) =>
    rows.filter(
      (row) =>
        (row.search ?? "").toLowerCase().includes(q.trim().toLowerCase()) &&
        Object.entries(values).every(
          ([key, value]) => !value || row.filters?.[key] === value,
        ),
    );
  const visible = sorted(matches(query, activeFilters), sort);
  const pages = Math.max(1, Math.ceil(visible.length / pageSize));
  const current = Math.min(page, pages - 1);
  const shown = visible.slice(current * pageSize, (current + 1) * pageSize);
  const applicable = (row: DataTableRow) => row.applicable !== false;

  const select = setSelected;
  const narrow = (q: string, values: Record<string, string>) => {
    setPage(0);
    const keep = new Set(matches(q, values).map((row) => row.id));
    if ([...selected].some((id) => !keep.has(id))) {
      select(new Set([...selected].filter((id) => keep.has(id))));
    }
  };

  if (rows !== seenRows) {
    setSeenRows(rows);
    const keep = new Set(visible.filter(applicable).map((row) => row.id));
    if ([...selected].some((id) => !keep.has(id))) {
      setSelected(new Set([...selected].filter((id) => keep.has(id))));
    }
  }
  const reported = useRef<string | undefined>(undefined);
  const selectedIds = rows
    .filter((row) => selected.has(row.id))
    .map((row) => row.id);
  useEffect(() => {
    const key = selectedIds.join("\n");
    if (reported.current !== undefined && reported.current !== key) {
      selection?.onChange?.(selectedIds);
    }
    reported.current = key;
  });

  return (
    <div className="flex min-w-0 flex-col gap-3">
      <div className="flex flex-wrap items-end gap-3">
        <div className="flex min-w-48 flex-1 flex-col gap-1">
          <label htmlFor={`${id}-search`} className="text-sm text-muted">
            Search
          </label>
          <Input
            id={`${id}-search`}
            type="search"
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              narrow(event.target.value, activeFilters);
            }}
          />
        </div>
        {filters.map((filter) => (
          <div key={filter.key} className="flex min-w-40 flex-col gap-1">
            <label
              htmlFor={`${id}-filter-${filter.key}`}
              className="text-sm text-muted"
            >
              {filter.label}
            </label>
            <NativeSelect
              id={`${id}-filter-${filter.key}`}
              value={activeFilters[filter.key] ?? ""}
              onChange={(event) => {
                const next = {
                  ...activeFilters,
                  [filter.key]: event.target.value,
                };
                setFilterValues(next);
                narrow(query, next);
              }}
            >
              <NativeSelectOption value="">All</NativeSelectOption>
              {filter.options.map((option) => (
                <NativeSelectOption key={option.value} value={option.value}>
                  {option.label}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>
        ))}
        {selection && (
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              disabled={!visible.some(applicable)}
              onClick={() =>
                select(new Set(visible.filter(applicable).map((row) => row.id)))
              }
            >
              {`Select all ${selection.applicableLabel}`}
            </Button>
            {selected.size > 0 && (
              <Button variant="ghost" onClick={() => select(new Set())}>
                Clear selection
              </Button>
            )}
            <span aria-live="polite" className="text-sm text-muted">
              {selected.size > 0 ? `${selected.size} selected` : ""}
            </span>
          </div>
        )}
      </div>
      <Table label={label}>
        <TableHeader>
          <TableRow>
            {selection && (
              <TableHead className="w-10">
                <span className="sr-only">Selected</span>
              </TableHead>
            )}
            {columns.map((column) => (
              <TableHead
                key={column.key}
                aria-sort={
                  sort?.key === column.key ? sort.direction : undefined
                }
              >
                {column.sortable ? (
                  <button
                    type="button"
                    className="inline-flex items-center gap-1 hover:text-text"
                    onClick={() =>
                      setSort(
                        sort?.key === column.key &&
                          sort.direction === "ascending"
                          ? { key: column.key, direction: "descending" }
                          : { key: column.key, direction: "ascending" },
                      )
                    }
                  >
                    {column.label}
                    {sort?.key === column.key &&
                      (sort.direction === "ascending" ? (
                        <ArrowUp aria-hidden className="size-3.5" />
                      ) : (
                        <ArrowDown aria-hidden className="size-3.5" />
                      ))}
                  </button>
                ) : (
                  column.label
                )}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {shown.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={columns.length + (selection ? 1 : 0)}
                className="py-6 text-center text-muted"
              >
                {emptyText}
              </TableCell>
            </TableRow>
          ) : (
            shown.map((row) => (
              <TableRow
                key={row.id}
                data-state={selected.has(row.id) ? "selected" : undefined}
              >
                {selection && (
                  <TableCell>
                    <Checkbox
                      aria-label={`Select ${row.label ?? row.id}`}
                      disabled={!applicable(row)}
                      checked={selected.has(row.id)}
                      onCheckedChange={(value) => {
                        const next = new Set(selected);
                        if (value === true) next.add(row.id);
                        else next.delete(row.id);
                        select(next);
                      }}
                    />
                  </TableCell>
                )}
                {columns.map((column, index) => (
                  <TableCell key={column.key}>
                    {index === 0 && row.href ? (
                      <Link
                        href={row.href}
                        className="font-medium text-text hover:text-info"
                      >
                        {row.cells[column.key]}
                      </Link>
                    ) : (
                      row.cells[column.key]
                    )}
                  </TableCell>
                ))}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
      {pages > 1 && (
        <div className="flex items-center justify-end gap-2 text-sm text-muted">
          <Button
            variant="ghost"
            size="sm"
            aria-label="Previous page"
            disabled={current === 0}
            onClick={() => setPage(current - 1)}
          >
            Previous
          </Button>
          <span>{`Page ${current + 1} of ${pages}`}</span>
          <Button
            variant="ghost"
            size="sm"
            aria-label="Next page"
            disabled={current >= pages - 1}
            onClick={() => setPage(current + 1)}
          >
            Next
          </Button>
        </div>
      )}
    </div>
  );
}

function sorted(rows: DataTableRow[], sort: Sort | undefined): DataTableRow[] {
  if (!sort) return rows;
  const sign = sort.direction === "ascending" ? 1 : -1;
  return [...rows].sort((a, b) => {
    const x = a.sort?.[sort.key];
    const y = b.sort?.[sort.key];
    if (x === undefined) return y === undefined ? 0 : 1;
    if (y === undefined) return -1;
    return (
      sign *
      (typeof x === "number" && typeof y === "number"
        ? x - y
        : String(x).localeCompare(String(y)))
    );
  });
}
