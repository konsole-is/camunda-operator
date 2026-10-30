/**
 * AccessSection shows the kubectl port-forward command that opens the app.
 * Without serviceName, the command names a placeholder for the Service.
 */
export function AccessSection({
  namespace,
  serviceName,
}: {
  namespace: string;
  serviceName?: string;
}) {
  const service = serviceName ?? "<fleet manager Service>";
  return (
    <div className="space-y-2 text-sm">
      <pre className="overflow-x-auto rounded-md bg-panel-2 px-3 py-2">
        <code className="font-mono text-xs text-text">
          {`kubectl -n ${namespace} port-forward service/${service} 3000:3000`}
        </code>
      </pre>
      <p className="text-muted">Then open http://localhost:3000.</p>
    </div>
  );
}
