import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r,t as i}from"./link-DOLMGIR2.js";import{n as a,r as o,t as s}from"./native-select-BF_LoX0G.js";import{a as c,c as l,i as u,n as d,o as f,s as p,t as m}from"./table-CcIT7E8t.js";import{r as h,t as g}from"./use-observe-DX85StN2.js";import{r as _,t as v}from"./usage-bars-CnFuBYAj.js";import{i as y,r as b,t as x}from"./pod-table-DkBwBnTA.js";import{n as S,t as C}from"./metric-chart-elt03BS6.js";function w({namespace:e,source:t,pods:n,usage:r,podless:i=!1}){let[a,o]=(0,A.useState)(`1h`),s=h(e,t,a);return(0,k.jsx)(T,{pods:n,usage:r,podless:i,charts:t?s??`loading`:void 0,range:a,onRange:o})}function T({pods:e,usage:t,charts:n,range:r,onRange:i,podless:a=!1}){return(0,k.jsxs)(`div`,{className:`flex flex-col gap-6 pt-2`,children:[(0,k.jsx)(E,{pods:e,usage:t,podless:a}),n!==void 0&&(0,k.jsx)(D,{charts:n,range:r,onRange:i})]})}function E({pods:e,usage:t,podless:n}){return(0,k.jsxs)(`section`,{className:`flex min-w-0 flex-col gap-2`,children:[(0,k.jsx)(`h3`,{className:`font-heading text-sm font-semibold text-text`,children:`Live usage`}),n?(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:x}):t===`unavailable`?(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:b}):(0,k.jsxs)(m,{label:`Live usage`,children:[(0,k.jsx)(f,{children:(0,k.jsxs)(p,{children:[(0,k.jsx)(c,{children:`Pod`}),(0,k.jsx)(c,{children:`CPU and memory`})]})}),(0,k.jsx)(d,{children:e.map(e=>{let n=e.metadata?.name??``,r=t?.find(e=>e.pod===n);return(0,k.jsxs)(p,{children:[(0,k.jsx)(`th`,{scope:`row`,className:`px-2 py-2 text-left align-top font-medium text-text`,children:n}),(0,k.jsx)(u,{children:r?(0,k.jsx)(v,{usage:r}):`-`})]},n)})})]})]})}function D({charts:e,range:t,onRange:n}){let r=(0,A.useId)();return(0,k.jsxs)(`section`,{className:`flex min-w-0 flex-col gap-3`,children:[(0,k.jsxs)(`div`,{className:`flex flex-wrap items-end justify-between gap-3`,children:[(0,k.jsx)(`h3`,{className:`font-heading text-sm font-semibold text-text`,children:`History`}),(0,k.jsxs)(`div`,{className:`flex items-center gap-2`,children:[(0,k.jsx)(`label`,{htmlFor:`${r}-range`,className:`text-sm text-muted`,children:`Range`}),(0,k.jsx)(s,{id:`${r}-range`,className:`w-40`,value:t,onChange:e=>n?.(e.target.value),children:Object.entries(j).map(([e,t])=>(0,k.jsx)(a,{value:e,children:t},e))})]})]}),(0,k.jsx)(O,{charts:e})]})}function O({charts:e}){if(e===`loading`)return(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:`Loading the charts`});if(e===`failed`)return(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:`Could not load the charts.`});if(e.prometheus===`notConfigured`)return(0,k.jsxs)(`p`,{className:`text-sm text-muted`,children:[`Prometheus is not configured, so the charts do not show.`,` `,(0,k.jsx)(r,{href:`/setup/settings`,className:`text-info underline underline-offset-2`,children:`Set the Prometheus URL in Settings`})]});if(e.prometheus===`unavailable`)return(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:`Prometheus did not answer. The charts show again when it answers.`});let t=e.hiddenForVersion;return(0,k.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[t&&(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:t.version?`The app has no Camunda charts for version ${t.version} yet.`:`The cluster reports no Camunda version yet, so the Camunda charts do not show.`}),e.charts.length===0?(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:`No metric has data for this range.`}):(0,k.jsx)(`div`,{className:`grid grid-cols-1 gap-3 md:grid-cols-2`,children:e.charts.map(e=>(0,k.jsx)(C,{title:e.label,unit:e.unit,note:e.declaredBy&&`Declared by ${e.declaredBy}`,series:e.series},`${e.declaredBy??``}/${e.label}`))}),e.errors.length>0&&(0,k.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,k.jsx)(`h4`,{className:`text-sm font-medium text-text`,children:`Queries that do not render`}),(0,k.jsx)(`ul`,{className:`flex flex-col gap-1 text-sm text-warn`,children:e.errors.map(e=>(0,k.jsx)(`li`,{children:`${e.label}, declared by ${e.declaredBy}: ${e.error}`},`${e.declaredBy}/${e.label}`))})]})]})}var k,A,j;function M(){return(M=e((()=>{k=n(),i(),A=t(),S(),y(),_(),o(),l(),g(),j={"1h":`Last hour`,"6h":`Last 6 hours`,"24h":`Last 24 hours`},w.__docgenInfo={description:`DetailMetrics is the Metrics tab of a component detail: the live usage of
pods, then the charts of source over the range the user picks, the last
hour first. Without source it asks no charts and shows no history. A
podless component shows NO_PODS in place of the live usage. It needs a
QueryClientProvider above it.`,methods:[],displayName:`DetailMetrics`,props:{namespace:{required:!0,tsType:{name:`string`},description:``},source:{required:!1,tsType:{name:`MetricsRef`},description:``},pods:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  metadata?: NonNullable<PodStateLike["metadata"]> & {
    creationTimestamp?: string | Date;
  };
  spec?: { containers?: unknown[]; nodeName?: string };
  status?: Omit<
    NonNullable<PodStateLike["status"]>,
    "containerStatuses" | "initContainerStatuses"
  > & {
    initContainerStatuses?: ContainerStatusLike[];
    containerStatuses?: ContainerStatusLike[];
  };
}`,signature:{properties:[{key:`metadata`,value:{name:`intersection`,raw:`NonNullable<PodStateLike["metadata"]> & {
  creationTimestamp?: string | Date;
}`,elements:[{name:`NonNullable`,elements:[{name:`signature`,raw:`PodStateLike["metadata"]`}],raw:`NonNullable<PodStateLike["metadata"]>`},{name:`signature`,type:`object`,raw:`{
  creationTimestamp?: string | Date;
}`,signature:{properties:[{key:`creationTimestamp`,value:{name:`union`,raw:`string | Date`,elements:[{name:`string`},{name:`Date`}],required:!1}}]}}],required:!1}},{key:`spec`,value:{name:`signature`,type:`object`,raw:`{ containers?: unknown[]; nodeName?: string }`,signature:{properties:[{key:`containers`,value:{name:`Array`,elements:[{name:`unknown`}],raw:`unknown[]`,required:!1}},{key:`nodeName`,value:{name:`string`,required:!1}}]},required:!1}},{key:`status`,value:{name:`intersection`,raw:`Omit<
  NonNullable<PodStateLike["status"]>,
  "containerStatuses" | "initContainerStatuses"
> & {
  initContainerStatuses?: ContainerStatusLike[];
  containerStatuses?: ContainerStatusLike[];
}`,elements:[{name:`Omit`,elements:[{name:`NonNullable`,elements:[{name:`signature`,raw:`PodStateLike["status"]`}],raw:`NonNullable<PodStateLike["status"]>`},{name:`union`,raw:`"containerStatuses" | "initContainerStatuses"`,elements:[{name:`literal`,value:`"containerStatuses"`},{name:`literal`,value:`"initContainerStatuses"`}]}],raw:`Omit<
  NonNullable<PodStateLike["status"]>,
  "containerStatuses" | "initContainerStatuses"
>`},{name:`signature`,type:`object`,raw:`{
  initContainerStatuses?: ContainerStatusLike[];
  containerStatuses?: ContainerStatusLike[];
}`,signature:{properties:[{key:`initContainerStatuses`,value:{name:`Array`,elements:[{name:`intersection`,raw:`ContainerStateLike & {
  ready: boolean;
  restartCount: number;
}`,elements:[{name:`signature`,type:`object`,raw:`{ state?: { waiting?: { reason?: string } } }`,signature:{properties:[{key:`state`,value:{name:`signature`,type:`object`,raw:`{ waiting?: { reason?: string } }`,signature:{properties:[{key:`waiting`,value:{name:`signature`,type:`object`,raw:`{ reason?: string }`,signature:{properties:[{key:`reason`,value:{name:`string`,required:!1}}]},required:!1}}]},required:!1}}]}},{name:`signature`,type:`object`,raw:`{
  ready: boolean;
  restartCount: number;
}`,signature:{properties:[{key:`ready`,value:{name:`boolean`,required:!0}},{key:`restartCount`,value:{name:`number`,required:!0}}]}}]}],raw:`ContainerStatusLike[]`,required:!1}},{key:`containerStatuses`,value:{name:`Array`,elements:[{name:`intersection`,raw:`ContainerStateLike & {
  ready: boolean;
  restartCount: number;
}`,elements:[{name:`signature`,type:`object`,raw:`{ state?: { waiting?: { reason?: string } } }`,signature:{properties:[{key:`state`,value:{name:`signature`,type:`object`,raw:`{ waiting?: { reason?: string } }`,signature:{properties:[{key:`waiting`,value:{name:`signature`,type:`object`,raw:`{ reason?: string }`,signature:{properties:[{key:`reason`,value:{name:`string`,required:!1}}]},required:!1}}]},required:!1}}]}},{name:`signature`,type:`object`,raw:`{
  ready: boolean;
  restartCount: number;
}`,signature:{properties:[{key:`ready`,value:{name:`boolean`,required:!0}},{key:`restartCount`,value:{name:`number`,required:!0}}]}}]}],raw:`ContainerStatusLike[]`,required:!1}}]}}],required:!1}}]}}],raw:`PodLike[]`},description:``},usage:{required:!0,tsType:{name:`union`,raw:`PodUsage[] | Unavailable | undefined`,elements:[{name:`Array`,elements:[{name:`PodUsage`}],raw:`PodUsage[]`},{name:`Unavailable`},{name:`undefined`}]},description:``},podless:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`false`,computed:!1}}}},T.__docgenInfo={description:`DetailMetricsView is DetailMetrics with its answer given. An undefined
usage is not loaded yet. Without charts, the view has no history
section.`,methods:[],displayName:`DetailMetricsView`,props:{pods:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  metadata?: NonNullable<PodStateLike["metadata"]> & {
    creationTimestamp?: string | Date;
  };
  spec?: { containers?: unknown[]; nodeName?: string };
  status?: Omit<
    NonNullable<PodStateLike["status"]>,
    "containerStatuses" | "initContainerStatuses"
  > & {
    initContainerStatuses?: ContainerStatusLike[];
    containerStatuses?: ContainerStatusLike[];
  };
}`,signature:{properties:[{key:`metadata`,value:{name:`intersection`,raw:`NonNullable<PodStateLike["metadata"]> & {
  creationTimestamp?: string | Date;
}`,elements:[{name:`NonNullable`,elements:[{name:`signature`,raw:`PodStateLike["metadata"]`}],raw:`NonNullable<PodStateLike["metadata"]>`},{name:`signature`,type:`object`,raw:`{
  creationTimestamp?: string | Date;
}`,signature:{properties:[{key:`creationTimestamp`,value:{name:`union`,raw:`string | Date`,elements:[{name:`string`},{name:`Date`}],required:!1}}]}}],required:!1}},{key:`spec`,value:{name:`signature`,type:`object`,raw:`{ containers?: unknown[]; nodeName?: string }`,signature:{properties:[{key:`containers`,value:{name:`Array`,elements:[{name:`unknown`}],raw:`unknown[]`,required:!1}},{key:`nodeName`,value:{name:`string`,required:!1}}]},required:!1}},{key:`status`,value:{name:`intersection`,raw:`Omit<
  NonNullable<PodStateLike["status"]>,
  "containerStatuses" | "initContainerStatuses"
> & {
  initContainerStatuses?: ContainerStatusLike[];
  containerStatuses?: ContainerStatusLike[];
}`,elements:[{name:`Omit`,elements:[{name:`NonNullable`,elements:[{name:`signature`,raw:`PodStateLike["status"]`}],raw:`NonNullable<PodStateLike["status"]>`},{name:`union`,raw:`"containerStatuses" | "initContainerStatuses"`,elements:[{name:`literal`,value:`"containerStatuses"`},{name:`literal`,value:`"initContainerStatuses"`}]}],raw:`Omit<
  NonNullable<PodStateLike["status"]>,
  "containerStatuses" | "initContainerStatuses"
>`},{name:`signature`,type:`object`,raw:`{
  initContainerStatuses?: ContainerStatusLike[];
  containerStatuses?: ContainerStatusLike[];
}`,signature:{properties:[{key:`initContainerStatuses`,value:{name:`Array`,elements:[{name:`intersection`,raw:`ContainerStateLike & {
  ready: boolean;
  restartCount: number;
}`,elements:[{name:`signature`,type:`object`,raw:`{ state?: { waiting?: { reason?: string } } }`,signature:{properties:[{key:`state`,value:{name:`signature`,type:`object`,raw:`{ waiting?: { reason?: string } }`,signature:{properties:[{key:`waiting`,value:{name:`signature`,type:`object`,raw:`{ reason?: string }`,signature:{properties:[{key:`reason`,value:{name:`string`,required:!1}}]},required:!1}}]},required:!1}}]}},{name:`signature`,type:`object`,raw:`{
  ready: boolean;
  restartCount: number;
}`,signature:{properties:[{key:`ready`,value:{name:`boolean`,required:!0}},{key:`restartCount`,value:{name:`number`,required:!0}}]}}]}],raw:`ContainerStatusLike[]`,required:!1}},{key:`containerStatuses`,value:{name:`Array`,elements:[{name:`intersection`,raw:`ContainerStateLike & {
  ready: boolean;
  restartCount: number;
}`,elements:[{name:`signature`,type:`object`,raw:`{ state?: { waiting?: { reason?: string } } }`,signature:{properties:[{key:`state`,value:{name:`signature`,type:`object`,raw:`{ waiting?: { reason?: string } }`,signature:{properties:[{key:`waiting`,value:{name:`signature`,type:`object`,raw:`{ reason?: string }`,signature:{properties:[{key:`reason`,value:{name:`string`,required:!1}}]},required:!1}}]},required:!1}}]}},{name:`signature`,type:`object`,raw:`{
  ready: boolean;
  restartCount: number;
}`,signature:{properties:[{key:`ready`,value:{name:`boolean`,required:!0}},{key:`restartCount`,value:{name:`number`,required:!0}}]}}]}],raw:`ContainerStatusLike[]`,required:!1}}]}}],required:!1}}]}}],raw:`PodLike[]`},description:``},usage:{required:!0,tsType:{name:`union`,raw:`PodUsage[] | Unavailable | undefined`,elements:[{name:`Array`,elements:[{name:`PodUsage`}],raw:`PodUsage[]`},{name:`Unavailable`},{name:`undefined`}]},description:``},charts:{required:!1,tsType:{name:`union`,raw:`MetricsAnswer | "failed" | "loading"`,elements:[{name:`MetricsAnswer`},{name:`literal`,value:`"failed"`},{name:`literal`,value:`"loading"`}]},description:``},range:{required:!0,tsType:{name:`MetricsRange`},description:``},onRange:{required:!1,tsType:{name:`signature`,type:`function`,raw:`(range: MetricsRange) => void`,signature:{arguments:[{type:{name:`MetricsRange`},name:`range`}],return:{name:`void`}}},description:``},podless:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`false`,computed:!1}}}}})))()}export{T as n,M as r,w as t};