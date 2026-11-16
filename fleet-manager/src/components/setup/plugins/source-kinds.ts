import type { KindName } from "@/lib/k8s/kinds";
import type { RegistrationSources } from "./sources";

/** REGISTRATION_SOURCE_KINDS is the kind of each list of RegistrationSources. */
export const REGISTRATION_SOURCE_KINDS = {
  sections: "PluginSection",
  registrations: "PluginRegistration",
  requests: "PluginRequest",
  namespaces: "Namespace",
  environments: "Environment",
} as const satisfies Record<keyof RegistrationSources, KindName>;
