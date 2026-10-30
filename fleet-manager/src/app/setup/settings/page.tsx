import { AccessSection } from "@/components/setup/access-section";
import { MetricsServerLine } from "@/components/setup/metrics-server-line";
import { PermissionsTable } from "@/components/setup/permissions-table";
import { SettingsForm } from "@/components/setup/settings-form";
import { KindMap } from "@/lib/k8s/kinds";
import { metricsServerFound } from "@/lib/observe/kube";
import { listSecretRefs } from "@/lib/secrets";
import { appPermissions, readSettings } from "@/lib/settings";
import { appNamespace, appServiceName } from "@/lib/settings/keys";
import { messageOf } from "@/lib/write/errors";
import { saveSettings, testGitConnection, testPrometheusQuery } from "./actions";

// The page reads the cluster on each request. A static prerender would call
// the API server at build time.
export const dynamic = "force-dynamic";

const TITLE = "font-heading text-3xl font-bold tracking-[-0.02em]";
const HEADING = "font-heading text-base font-semibold";

export default async function SettingsPage() {
  const namespace = appNamespace();
  const [read, secretNames, metricsServer, permissions] = await Promise.all([
    readSettings().catch((error: unknown) => ({ error: messageOf(error) })),
    listSecretRefs(namespace)
      .then((secrets) => secrets.map((s) => s.name))
      .catch(() => []),
    metricsServerFound(namespace).then(
      (found) => ({ found }),
      (error: unknown) => ({ error: messageOf(error) }),
    ),
    appPermissions().catch((error: unknown) => ({ error: messageOf(error) })),
  ]);
  return (
    <div className="space-y-10">
      <h1 className={TITLE}>Settings</h1>
      {"error" in read ? (
        <p role="alert" className="text-sm text-bad">
          Could not read the settings: {read.error}
        </p>
      ) : (
        <SettingsForm
          settings={read.settings}
          resourceVersion={read.resourceVersion}
          namespace={namespace}
          secretNames={secretNames}
          layoutKinds={{
            namespaced: KindMap.CamundaClusterClaim,
            clusterScoped: KindMap.Environment,
          }}
          save={saveSettings}
          testGit={testGitConnection}
          testPrometheus={testPrometheusQuery}
        />
      )}
      <section className="max-w-2xl space-y-3">
        <h2 className={HEADING}>Live CPU and memory</h2>
        <MetricsServerLine state={metricsServer} />
      </section>
      <section className="max-w-2xl space-y-3">
        <h2 className={HEADING}>Access</h2>
        <AccessSection namespace={namespace} serviceName={appServiceName()} />
      </section>
      <section className="space-y-3">
        <h2 className={HEADING}>Permissions</h2>
        <p className="max-w-2xl text-sm text-muted">
          What the fleet manager may do in namespace {namespace}. The chart
          sets these rules.
        </p>
        <PermissionsTable permissions={permissions} />
      </section>
    </div>
  );
}
