"use client";

import { useId, useState } from "react";
import { LogViewer } from "@/components/fleet/log-viewer";
import type { PodLike } from "@/components/fleet/pod-table";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { MAX_LOG_STREAMS } from "@/lib/observe/logs";
import type { LogLine } from "@/lib/observe/types";
import {
  type LogStreamSource,
  type PodLogError,
  useLogStreams,
} from "@/lib/observe/use-log-stream";
import { useControllerPods } from "@/lib/observe/use-observe";

/**
 * ControllerLogs names the controller logs of a request-backed component:
 * the registration whose controller pods to follow, and the request whose
 * lines to keep.
 */
export type ControllerLogs = {
  registration: string;
  request: { namespace: string; name: string };
};

/** LOG_TAIL_LINES is the number of earlier lines that each stream starts with. */
export const LOG_TAIL_LINES = 200;

const ALL = "";
const CONTROLLER = "controller/";

/**
 * DetailLogs is the Logs tab of a component detail. It follows the logs of
 * pods in namespace, and with controller also the lines of its request in
 * the logs of the controller pods. The user picks one pod or all pods; all
 * pods follows the first MAX_LOG_STREAMS, the pods of the component first.
 * It needs a QueryClientProvider above it.
 */
export function DetailLogs({
  namespace,
  pods,
  controller,
}: {
  namespace: string;
  pods: PodLike[];
  controller?: ControllerLogs;
}) {
  const [picked, setPicked] = useState(ALL);
  const found = useControllerPods(controller?.registration);
  const own: LogStreamSource[] = pods.map((p) => ({
    namespace,
    pod: p.metadata?.name ?? "",
  }));
  const controllers: LogStreamSource[] =
    controller && found && found !== "unavailable"
      ? found.pods.map((pod) => ({
          namespace: found.namespace,
          pod,
          request: `${controller.request.namespace}/${controller.request.name}`,
        }))
      : [];
  const all = [...own, ...controllers];
  const one = all.find((s, i) => valueOf(s, i >= own.length) === picked);
  const pick = one ? picked : ALL;
  const { lines, errors } = useLogStreams(
    one ? [one] : all.slice(0, MAX_LOG_STREAMS),
    { tailLines: LOG_TAIL_LINES },
  );
  return (
    <DetailLogsView
      pods={own.map((s) => s.pod)}
      controllerPods={controller && controllers.map((s) => s.pod)}
      controllerUnavailable={
        controller !== undefined && found === "unavailable"
      }
      picked={pick}
      onPick={setPicked}
      lines={lines}
      errors={errors}
    />
  );
}

/**
 * DetailLogsView is DetailLogs with its streams given. picked is "" for all
 * pods, the name of a pod, or "controller/" and the name of a controller
 * pod; onPick gets the same values. Without controllerPods, the picker has
 * no Controller group.
 */
export function DetailLogsView({
  pods,
  controllerPods,
  controllerUnavailable = false,
  picked,
  onPick,
  lines,
  errors,
}: {
  pods: string[];
  controllerPods?: string[];
  controllerUnavailable?: boolean;
  picked: string;
  onPick?: (value: string) => void;
  lines: LogLine[];
  errors: PodLogError[];
}) {
  const id = useId();
  const total = pods.length + (controllerPods?.length ?? 0);
  if (total === 0 && !controllerUnavailable) {
    return (
      <p className="pt-2 text-sm text-muted">
        This component has no pods, so it has no logs.
      </p>
    );
  }
  const picker = (
    <div className="flex flex-col gap-1">
      <label htmlFor={`${id}-pod`} className="text-xs text-muted">
        Pod
      </label>
      <NativeSelect
        id={`${id}-pod`}
        value={picked}
        onChange={(event) => onPick?.(event.target.value)}
        className="w-60"
      >
        <NativeSelectOption
          value={ALL}
        >{`All pods (${total})`}</NativeSelectOption>
        {pods.length > 0 && (
          <optgroup label="Pods">
            {pods.map((pod) => (
              <NativeSelectOption key={pod} value={pod}>
                {pod}
              </NativeSelectOption>
            ))}
          </optgroup>
        )}
        {controllerPods && controllerPods.length > 0 && (
          <optgroup label="Controller">
            {controllerPods.map((pod) => (
              <NativeSelectOption key={pod} value={`${CONTROLLER}${pod}`}>
                {pod}
              </NativeSelectOption>
            ))}
          </optgroup>
        )}
      </NativeSelect>
    </div>
  );
  return (
    <div className="flex flex-col gap-2 pt-2">
      <LogViewer lines={lines} controls={picker} source={picked} />
      {picked === ALL && total > MAX_LOG_STREAMS && (
        <p className="text-sm text-muted">
          {`Showing the logs of ${MAX_LOG_STREAMS} of ${total} pods. Pick a pod to see the others.`}
        </p>
      )}
      {controllerUnavailable && (
        <p className="text-sm text-warn">Could not read the controller pods.</p>
      )}
      {errors.length > 0 && (
        <ul className="flex flex-col gap-1 text-sm text-warn">
          {errors.map((e) => (
            <li key={e.pod}>{`${e.pod}: ${e.error}`}</li>
          ))}
        </ul>
      )}
    </div>
  );
}

function valueOf(source: LogStreamSource, controller: boolean): string {
  return controller ? `${CONTROLLER}${source.pod}` : source.pod;
}
