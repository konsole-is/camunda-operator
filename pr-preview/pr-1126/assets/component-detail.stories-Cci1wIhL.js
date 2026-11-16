import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{t as n}from"./hooks-B0uqLHHX.js";import{a as r,i,n as a,r as o,t as s}from"./tabs-CtFKz6UY.js";import{i as c,n as l}from"./topology-3Ta4rKpp.js";import{n as u,t as d}from"./status-chip-CDD4orMY.js";import{i as f,n as p,r as m}from"./cards-BY3mzYH7.js";import{n as h,t as g}from"./stat-tile-BSk7ytU6.js";import{n as _,t as v}from"./condition-list-cIXzm0Ue.js";import{t as y}from"./use-observe-DX85StN2.js";import{i as b,r as x,t as S}from"./pod-table-C7UfxN8F.js";import{n as C}from"./event-list-CmAIeWf_.js";import{r as ee}from"./detail-logs-BVQVfQnM.js";import{n as w,r as T}from"./detail-metrics-CyaB17kR.js";import{i as E,n as D,r as O,t as k}from"./partition-table-Ip4Yzbcq.js";import{a as A,i as j,n as M,r as N,s as P,t as te}from"./fixtures-Dr1JTW37.js";function F({pods:e,usage:t}){let n=f(e),r=e.map(e=>e.metadata?.name??``),i=e.reduce((e,t)=>e+b(t),0);return(0,L.jsxs)(`dl`,{className:`grid grid-cols-2 gap-3 lg:grid-cols-4`,children:[(0,L.jsx)(g,{label:n.label,value:n.value,note:e.some(e=>c(e).tone!==`ok`)?`Not all pods are ready`:void 0}),(0,L.jsx)(I,{label:`Highest CPU`,highest:p(t,r,[`cpu`])}),(0,L.jsx)(I,{label:`Highest memory`,highest:p(t,r,[`memory`])}),(0,L.jsx)(g,{label:`Restarts`,value:String(i),note:`Since each pod started`})]})}function I({label:e,highest:t}){return typeof t==`string`?(0,L.jsx)(g,{label:e,value:t}):(0,L.jsx)(g,{label:e,value:`${Math.round(t.share*100)} %`,note:`${t.pod}, of its limit`})}var L;function R(){return(R=e((()=>{L=t(),m(),x(),h(),l(),F.__docgenInfo={description:`ComponentStats shows the numbers of the pods of a component: the pods
ready, the highest CPU and the highest memory of one pod against its
limit, and the restarts of all containers. An undefined usage is not
loaded yet.`,methods:[],displayName:`ComponentStats`,props:{pods:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`ready`,value:{name:`boolean`,required:!0}},{key:`restartCount`,value:{name:`number`,required:!0}}]}}]}],raw:`ContainerStatusLike[]`,required:!1}}]}}],required:!1}}]}}],raw:`PodLike[]`},description:``},usage:{required:!0,tsType:{name:`union`,raw:`PodUsage[] | Unavailable | undefined`,elements:[{name:`Array`,elements:[{name:`PodUsage`}],raw:`PodUsage[]`},{name:`Unavailable`},{name:`undefined`}]},description:``}}}})))()}function z(){return(z=e((()=>{t(),C(),n()})))()}function B({title:e,subtitle:t,conditions:n,ready:r,pods:c,usage:l,settings:u,metricsTab:f,logsTab:p,eventsTab:m,children:h,now:g}){return(0,V.jsxs)(`section`,{"aria-label":e,className:`flex flex-col gap-4 rounded-lg border border-border bg-panel p-4`,children:[(0,V.jsxs)(`header`,{className:`flex flex-col gap-0.5`,children:[(0,V.jsxs)(`div`,{className:`flex flex-wrap items-center gap-x-3 gap-y-1`,children:[(0,V.jsx)(`h2`,{tabIndex:-1,className:`font-heading text-lg font-semibold text-text outline-none`,children:e}),(0,V.jsx)(d,{condition:r,readyReason:!0})]}),t&&(0,V.jsx)(`p`,{className:`text-sm text-muted`,children:t})]}),(0,V.jsxs)(s,{defaultValue:`overview`,children:[(0,V.jsx)(o,{variant:`line`,children:H.map(e=>(0,V.jsx)(i,{value:e.value,children:e.label},e.value))}),(0,V.jsx)(a,{value:`overview`,children:(0,V.jsxs)(`div`,{className:`flex flex-col gap-6 pt-2`,children:[(0,V.jsx)(F,{pods:c,usage:l}),(0,V.jsxs)(`div`,{className:`flex flex-wrap items-start gap-6`,children:[(0,V.jsx)(O,{title:`Pods`,className:`flex-[999_1_32rem]`,children:(0,V.jsx)(S,{pods:c,usage:l??[]})}),(0,V.jsx)(O,{title:`Conditions`,className:`flex-[1_1_18rem]`,children:(0,V.jsx)(v,{conditions:n,label:`${e} conditions`,now:g})})]}),h]})}),(0,V.jsx)(a,{value:`metrics`,children:f??(0,V.jsx)(w,{pods:c,usage:l,range:`1h`})}),(0,V.jsx)(a,{value:`logs`,children:p??(0,V.jsx)(`p`,{className:`pt-2 text-sm text-muted`,children:`No logs.`})}),(0,V.jsx)(a,{value:`events`,children:m??(0,V.jsx)(`p`,{className:`pt-2 text-sm text-muted`,children:`No events`})}),(0,V.jsx)(a,{value:`settings`,children:(0,V.jsx)(`div`,{className:`pt-2`,children:u??(0,V.jsx)(`p`,{className:`text-sm text-muted`,children:`No settings.`})})})]})]})}var V,H;function U(){return(U=e((()=>{V=t(),_(),u(),r(),n(),y(),R(),z(),ee(),T(),E(),x(),H=[{value:`overview`,label:`Overview`},{value:`metrics`,label:`Metrics`},{value:`logs`,label:`Logs`},{value:`events`,label:`Events`},{value:`settings`,label:`Settings`}],B.__docgenInfo={description:`ComponentDetailView is ComponentDetail with its pods and usage given.
An undefined usage is not loaded yet. Without metricsTab, the Metrics
tab shows the live usage of the pods and no charts. Without logsTab or
eventsTab, that tab says there is nothing to show.`,methods:[],displayName:`ComponentDetailView`,props:{title:{required:!0,tsType:{name:`string`},description:``},subtitle:{required:!1,tsType:{name:`string`},description:``},conditions:{required:!0,tsType:{name:`Array`,elements:[{name:`ConditionRow`}],raw:`ConditionRow[]`},description:``},ready:{required:!0,tsType:{name:`union`,raw:`ConditionRow | undefined`,elements:[{name:`ConditionRow`},{name:`undefined`}]},description:``},settings:{required:!1,tsType:{name:`ReactNode`},description:``},children:{required:!1,tsType:{name:`ReactNode`},description:``},now:{required:!0,tsType:{name:`string`},description:``},pods:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`ready`,value:{name:`boolean`,required:!0}},{key:`restartCount`,value:{name:`number`,required:!0}}]}}]}],raw:`ContainerStatusLike[]`,required:!1}}]}}],required:!1}}]}}],raw:`PodLike[]`},description:``},usage:{required:!0,tsType:{name:`union`,raw:`PodUsage[] | Unavailable | undefined`,elements:[{name:`Array`,elements:[{name:`PodUsage`}],raw:`PodUsage[]`},{name:`Unavailable`},{name:`undefined`}]},description:``},metricsTab:{required:!1,tsType:{name:`ReactNode`},description:``},logsTab:{required:!1,tsType:{name:`ReactNode`},description:``},eventsTab:{required:!1,tsType:{name:`ReactNode`},description:``}}}})))()}var W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{W=t(),U(),N(),D(),G={title:`cluster/ComponentDetail`,component:B,args:{now:j}},K={type:`ZeebeReady`,status:`False`,reason:`Degraded`,message:`2 of 3 pods ready`,lastTransitionTime:`2026-10-30T11:52:00Z`},q={type:`OperateReady`,status:`True`,reason:`Disabled`,message:``,lastTransitionTime:`2026-10-29T08:00:00Z`},J={type:`TasklistReady`,status:`True`,reason:`Healthy`,message:``,lastTransitionTime:`2026-10-29T08:00:00Z`},Y={name:`Brokers degraded`,args:{title:`Brokers`,conditions:[K],ready:K,pods:te,usage:[P(`orders-zeebe-0`,.8,2.6),P(`orders-zeebe-1`,1.1,3.1)],children:(0,W.jsx)(k,{partitions:M})}},X={name:`Embedded Operate in the gateway`,args:{title:`Operate`,subtitle:`Embedded in Gateway`,conditions:[q],ready:q,pods:[A(`orders-gateway-0`,`pool-a-1`)],usage:[P(`orders-gateway-0`,.3,1.2)]}},Z={name:`Usage not available`,args:{title:`Tasklist`,conditions:[J],ready:J,pods:[A(`orders-tasklist-0`,`pool-a-2`)],usage:`unavailable`}},Q=[`BrokersDegraded`,`EmbeddedOperateInTheGateway`,`UsageNotAvailable`],Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Brokers degraded",
  args: {
    title: "Brokers",
    conditions: [zeebeDegraded],
    ready: zeebeDegraded,
    pods: brokers,
    usage: [usage("orders-zeebe-0", 0.8, 2.6), usage("orders-zeebe-1", 1.1, 3.1)],
    children: <PartitionTable partitions={degraded} />
  }
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Embedded Operate in the gateway",
  args: {
    title: "Operate",
    subtitle: "Embedded in Gateway",
    conditions: [operateDisabled],
    ready: operateDisabled,
    pods: [pod("orders-gateway-0", "pool-a-1")],
    usage: [usage("orders-gateway-0", 0.3, 1.2)]
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Usage not available",
  args: {
    title: "Tasklist",
    conditions: [tasklistHealthy],
    ready: tasklistHealthy,
    pods: [pod("orders-tasklist-0", "pool-a-2")],
    usage: "unavailable"
  }
}`,...Z.parameters?.docs?.source}}}})))()}$();export{Y as BrokersDegraded,X as EmbeddedOperateInTheGateway,Z as UsageNotAvailable,Q as __namedExportsOrder,G as default};