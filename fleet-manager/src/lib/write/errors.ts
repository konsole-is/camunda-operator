import "server-only";
import { KubernetesRequestError } from "@/lib/k8s/client";
import type { ApiError, ObjectRef } from "./types";

const managerOfCause = /conflict with "([^"]+)"/;
const pathOfMessage = /is invalid: ([A-Za-z][\w.[\]"=/-]*): /;

/**
 * apiErrorsOf maps a failed request for the object ref to the errors that a
 * page shows. hasPrecondition tells that the request carried the
 * resourceVersion that the page read: a 409 or a 404 then means that the
 * object changed or was deleted since, and gives Stale. A field manager
 * conflict gives Conflict in both cases. Every error carries ref.
 */
export function apiErrorsOf(
  error: unknown,
  ref: ObjectRef,
  hasPrecondition: boolean,
): ApiError[] {
  if (!(error instanceof KubernetesRequestError)) {
    return [{ code: "Unavailable", message: messageOf(error), ref }];
  }
  const causes = error.status?.details?.causes ?? [];
  switch (error.statusCode) {
    case 409: {
      const conflicts = causes.filter(
        (c) => c.reason === "FieldManagerConflict",
      );
      if (conflicts.length > 0) {
        return conflicts.map((c) => {
          const message = c.message ?? error.message;
          const manager = managerOfCause.exec(message)?.[1];
          return {
            code: "Conflict",
            message,
            ...pathOf(c.field),
            ...(manager === undefined ? {} : { manager }),
            ref,
          };
        });
      }
      if (hasPrecondition) return [stale(ref)];
      return [{ code: "Conflict", message: error.message, ref }];
    }
    case 404:
      if (hasPrecondition) return [stale(ref)];
      return [
        {
          code: "Unavailable",
          message: `The API server does not serve ${ref.kind} (${ref.apiVersion}).`,
          ref,
        },
      ];
    case 400:
    case 422: {
      const fields = causes.filter((c) => c.field);
      if (fields.length > 0) {
        return fields.map((c) => ({
          code: "Invalid",
          message: c.message ?? error.message,
          ...pathOf(c.field),
          ref,
        }));
      }
      const path = pathOfMessage.exec(error.message)?.[1];
      return [
        {
          code: "Invalid",
          message: error.message,
          ...(path === undefined ? {} : { path }),
          ref,
        },
      ];
    }
    case 401:
    case 403:
      return [{ code: "Forbidden", message: error.message, ref }];
    default:
      return [{ code: "Unavailable", message: error.message, ref }];
  }
}

function stale(ref: ObjectRef): ApiError {
  return {
    code: "Stale",
    message: `The ${ref.kind} ${ref.name} changed since you opened it.`,
    ref,
  };
}

function pathOf(field: string | undefined): { path?: string } {
  if (!field) return {};
  return { path: field.startsWith(".") ? field.slice(1) : field };
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
