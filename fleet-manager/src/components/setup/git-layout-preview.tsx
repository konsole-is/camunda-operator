import { gitBase, gitPath } from "@/lib/git-layout";
import type { KindInfo } from "@/lib/k8s/kinds";

/**
 * GitLayoutPreview shows the file that a save in Git mode writes for the
 * claim team-a/orders of namespaced and for the object production of
 * clusterScoped, under path. A path that names no folder shows an alert and
 * no file.
 */
export function GitLayoutPreview({
  path,
  namespaced,
  clusterScoped,
}: {
  path: string;
  namespaced: KindInfo;
  clusterScoped: KindInfo;
}) {
  const base = gitBase(path);
  if (base === undefined) {
    return (
      <p role="alert" className="text-sm text-bad">
        git.path must name a folder in the repository.
      </p>
    );
  }
  const files = [
    gitPath(base, namespaced, example("orders", "team-a")),
    gitPath(base, clusterScoped, example("production")),
  ];
  return (
    <div className="space-y-1 text-sm">
      <p className="text-muted">A save writes each object to its own file:</p>
      <ul className="space-y-0.5">
        {files.map((file) => (
          <li key={file}>
            <code className="font-mono text-xs text-text-2 [overflow-wrap:anywhere]">
              {file}
            </code>
          </li>
        ))}
      </ul>
    </div>
  );
}

function example(name: string, namespace?: string) {
  return { apiVersion: "", kind: "", name, namespace };
}
