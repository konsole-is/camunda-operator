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
 * conflict gives Conflict in both cases. Every error carries ref. An error
 * of a Secret has a fixed message, with no text of the API server.
 */
export function apiErrorsOf(
  error: unknown,
  ref: ObjectRef,
  hasPrecondition: boolean,
): ApiError[] {
  const errors = errorsOfAnswer(error, ref, hasPrecondition);
  return ref.kind === "Secret" ? errors.map(withSecretMessage) : errors;
}

// An API server message or an admission webhook can quote the object, and
// no Secret value may leave the app server (contract 6.2).
function withSecretMessage(error: ApiError): ApiError {
  const name = error.ref?.name ?? "";
  const messages: Record<ApiError["code"], string> = {
    Stale: error.message,
    Conflict: `Another field manager owns a field of the Secret ${name}.`,
    Invalid: `The API server refused the Secret ${name}.`,
    Forbidden: `The fleet manager may not write the Secret ${name}.`,
    Unavailable: `The API server did not write the Secret ${name}.`,
  };
  return { ...error, message: messages[error.code] };
}

function errorsOfAnswer(
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
      if (hasPrecondition) return [staleError(ref)];
      return [{ code: "Conflict", message: error.message, ref }];
    }
    case 404:
      if (hasPrecondition) return [staleError(ref)];
      return [
        {
          code: "Unavailable",
          message:
            error.status?.message ??
            `The API server does not serve ${ref.kind} (${ref.apiVersion}).`,
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

/** staleError is the error of a write of ref whose object changed or was deleted since the page read it. */
export function staleError(ref: ObjectRef): ApiError {
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

/** messageOf is the message of error, or error as text when it is not an Error. */
export function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/**
 * reasonOf returns why a connection failed: an error code, an HTTP status,
 * or a timeout. It never returns the text of the error, because a library
 * message can quote the answer of the server, and the answer can hold the
 * credentials.
 */
export function reasonOf(error: unknown): string {
  if (error instanceof KubernetesRequestError) {
    return `the API server answered HTTP ${error.statusCode}`;
  }
  const cause = error instanceof Error && error.cause ? error.cause : error;
  const name = (cause as { name?: unknown }).name;
  const message = (cause as { message?: unknown }).message;
  if (
    name === "AbortError" ||
    name === "TimeoutError" ||
    message === "Request timed out"
  ) {
    return "the server did not answer in time";
  }
  const code = (cause as { code?: unknown }).code;
  if (typeof code === "string" && /^E[A-Z_]+$/.test(code)) return code;
  return "the server did not give a valid answer";
}
