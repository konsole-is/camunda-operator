import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,o as r}from"./iframe-CtT9Kmuz.js";import{r as i,s as a,t as o}from"./status-B-qMFQRy.js";import{a as s,c,i as l,n as u,o as d,s as f,t as p}from"./table-6JI0HgF6.js";import{i as m,n as h}from"./topology-i-JGxj_n.js";import{n as g,r as _}from"./usage-bars-Ba3yD4sY.js";function v(e){return[...e.status?.initContainerStatuses??[],...e.status?.containerStatuses??[]].reduce((e,t)=>e+t.restartCount,0)}function y({pods:e,usage:t}){let r=t!==`unavailable`;return(0,b.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-2`,children:[(0,b.jsxs)(p,{label:`Pods`,children:[(0,b.jsx)(d,{children:(0,b.jsxs)(f,{children:[(0,b.jsx)(s,{children:`Pod`}),(0,b.jsx)(s,{children:`State`}),(0,b.jsx)(s,{children:`Restarts`}),r&&(0,b.jsx)(s,{children:`CPU`}),r&&(0,b.jsx)(s,{children:`Memory`})]})}),(0,b.jsx)(u,{children:e.map(e=>{let a=e.metadata?.name??``,s=m(e),c=r?t.find(e=>e.pod===a):void 0;return(0,b.jsxs)(f,{children:[(0,b.jsxs)(`th`,{scope:`row`,className:`px-2 py-2 text-left align-top font-normal whitespace-nowrap`,children:[(0,b.jsx)(`span`,{className:`block font-mono text-xs text-text`,children:a}),(0,b.jsx)(`span`,{className:`block text-xs text-muted`,children:e.spec?.nodeName??`-`})]}),(0,b.jsx)(l,{className:`align-top`,children:(0,b.jsxs)(`span`,{className:n(`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium`,o[s.tone]),children:[(0,b.jsx)(`span`,{"aria-hidden":!0,className:n(`size-1.5 shrink-0 rounded-full`,i[s.tone])}),s.state]})}),(0,b.jsx)(l,{className:`align-top tabular-nums`,children:v(e)}),r&&[`cpu`,`memory`].map(e=>(0,b.jsx)(l,{className:`align-top`,children:c?(0,b.jsx)(g,{usage:c,resource:e}):`-`},e))]},a)})})]}),(0,b.jsx)(`p`,{className:`text-xs text-muted`,children:r?C:x})]})}var b,x,S,C;function w(){return(w=e((()=>{b=t(),a(),_(),c(),r(),h(),x=`Usage is not available. The cluster has no metrics server, or the app could not read the usage.`,S=`This component runs no pods.`,C=`CPU and memory show the use now, against the limit of the pod, or its request without a limit. They come from the metrics API of Kubernetes, so they need no Prometheus.`,y.__docgenInfo={description:`PodTable lists pods with their node, their state in the words of the
topology, the restarts of their containers, and their CPU and memory.
When usage is "unavailable", the CPU and memory columns are hidden and a
note says why. A pod without usage data shows "-" in those columns.`,methods:[],displayName:`PodTable`,props:{pods:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`ready`,value:{name:`boolean`,required:!0}},{key:`restartCount`,value:{name:`number`,required:!0}}]}}]}],raw:`ContainerStatusLike[]`,required:!1}}]}}],required:!1}}]}}],raw:`PodLike[]`},description:``},usage:{required:!0,tsType:{name:`union`,raw:`PodUsage[] | Unavailable`,elements:[{name:`Array`,elements:[{name:`PodUsage`}],raw:`PodUsage[]`},{name:`Unavailable`}]},description:``}}}})))()}export{v as a,w as i,y as n,x as r,S as t};