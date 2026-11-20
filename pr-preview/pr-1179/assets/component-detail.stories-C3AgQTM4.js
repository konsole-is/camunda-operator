import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{t as r}from"./hooks-DwXdO6lu.js";import{a as i,i as a,n as o,r as s,t as ee}from"./tabs-B7bEhG1a.js";import{i as c,n as l}from"./topology-DledlgKy.js";import{n as u,t as te}from"./not-read-RuqQXk-B.js";import{n as d,t as ne}from"./condition-list-CcdWx4X0.js";import{n as f,t as re}from"./condition-badge-DJH6kEgF.js";import{n as p,t as m}from"./detail-section-B0yh-law.js";import{n as h,t as ie}from"./status-chip-C8Hgh04V.js";import{i as g,o as _,r as v}from"./cards-DzRZJ_PD.js";import{n as y,t as b}from"./stat-tile-BSk7ytU6.js";import{t as x}from"./use-observe-DX85StN2.js";import{a as S,i as C,n as w,t as T}from"./pod-table-qGaCJ_NX.js";import{n as E}from"./event-list-hkuQEWQ6.js";import"./events-C2nhwmNt.js";import{r as D}from"./detail-logs-ZN_7dYYC.js";import{n as O,r as k}from"./detail-metrics-BFTeONli.js";import{a as A,i as j,n as ae,r as oe,s as M,t as se}from"./fixtures-Dr1JTW37.js";import{n as ce,t as le}from"./partition-table-DoP0y9ym.js";function N({pods:e,usage:t}){let n=_(e),r=e.map(e=>e.metadata?.name??``),i=e.reduce((e,t)=>e+S(t),0);return(0,F.jsxs)(`dl`,{className:`grid grid-cols-2 gap-3 lg:grid-cols-4`,children:[(0,F.jsx)(b,{label:n.label,value:n.value,note:e.some(e=>c(e).tone!==`ok`)?`Not all pods are ready`:void 0}),(0,F.jsx)(P,{label:`Highest CPU`,highest:v(t,r,[`cpu`])}),(0,F.jsx)(P,{label:`Highest memory`,highest:v(t,r,[`memory`])}),(0,F.jsx)(b,{label:`Restarts`,value:String(i),note:`Since each pod started`})]})}function P({label:e,highest:t}){return typeof t==`string`?(0,F.jsx)(b,{label:e,value:t}):(0,F.jsx)(b,{label:e,value:`${Math.round(t.share*100)} %`,note:`${t.pod}, of its limit`})}var F;function I(){return(I=e((()=>{F=n(),g(),C(),y(),l(),N.__docgenInfo={description:`ComponentStats shows the numbers of the pods of a component: the pods
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
}`,signature:{properties:[{key:`ready`,value:{name:`boolean`,required:!0}},{key:`restartCount`,value:{name:`number`,required:!0}}]}}]}],raw:`ContainerStatusLike[]`,required:!1}}]}}],required:!1}}]}}],raw:`PodLike[]`},description:``},usage:{required:!0,tsType:{name:`union`,raw:`PodUsage[] | Unavailable | undefined`,elements:[{name:`Array`,elements:[{name:`PodUsage`}],raw:`PodUsage[]`},{name:`Unavailable`},{name:`undefined`}]},description:``}}}})))()}function L(){return(L=e((()=>{n(),E(),r()})))()}function R({title:e,subtitle:t,conditions:n,ready:r,unread:i,pods:c,usage:l,settings:u,metricsTab:d,logsTab:f,eventsTab:p,children:h,now:g,podless:_=!1,tab:v,onTab:y}){return(0,z.jsxs)(`section`,{"aria-label":e,className:`flex flex-col gap-4 rounded-lg border border-border bg-panel p-4`,children:[(0,z.jsxs)(`header`,{className:`flex flex-col gap-0.5`,children:[(0,z.jsxs)(`div`,{className:`flex flex-wrap items-center gap-x-3 gap-y-1`,children:[(0,z.jsx)(`h2`,{tabIndex:-1,className:`font-heading text-lg font-semibold text-text outline-none`,children:e}),i?(0,z.jsx)(re,{type:`Ready`,unknown:!0}):(0,z.jsx)(ie,{condition:r,readyReason:!0})]}),t&&(0,z.jsx)(`p`,{className:`text-sm text-muted`,children:t})]}),(0,z.jsxs)(ee,{defaultValue:`overview`,value:v,onValueChange:e=>y?.(e),children:[(0,z.jsx)(s,{variant:`line`,children:B.map(e=>(0,z.jsx)(a,{value:e.value,children:e.label},e.value))}),(0,z.jsx)(o,{value:`overview`,children:(0,z.jsxs)(`div`,{className:`flex flex-col gap-6 pt-2`,children:[!_&&(0,z.jsx)(N,{pods:c,usage:l}),(0,z.jsxs)(`div`,{className:`flex flex-wrap items-start gap-6`,children:[(0,z.jsx)(m,{title:`Pods`,className:`flex-[999_1_32rem]`,children:_?(0,z.jsx)(`p`,{className:`text-sm text-muted`,children:T}):(0,z.jsx)(w,{pods:c,usage:l??[]})}),(0,z.jsx)(m,{title:`Conditions`,className:`flex-[1_1_18rem]`,children:i?(0,z.jsx)(te,{kinds:[i]}):(0,z.jsx)(ne,{conditions:n,label:`${e} conditions`,now:g})})]}),h]})}),(0,z.jsx)(o,{value:`metrics`,children:d??(0,z.jsx)(O,{pods:c,usage:l,range:`1h`,podless:_})}),(0,z.jsx)(o,{value:`logs`,children:f??(0,z.jsx)(`p`,{className:`pt-2 text-sm text-muted`,children:`No logs.`})}),(0,z.jsx)(o,{value:`events`,children:p??(0,z.jsx)(`p`,{className:`pt-2 text-sm text-muted`,children:`No events`})}),(0,z.jsx)(o,{value:`settings`,children:(0,z.jsx)(`div`,{className:`pt-2`,children:u??(0,z.jsx)(`p`,{className:`text-sm text-muted`,children:`No settings.`})})})]})]})}var z,B;function V(){return(V=e((()=>{z=n(),t(),d(),u(),h(),f(),i(),r(),x(),I(),L(),D(),k(),p(),C(),B=[{value:`overview`,label:`Overview`},{value:`metrics`,label:`Metrics`},{value:`logs`,label:`Logs`},{value:`events`,label:`Events`},{value:`settings`,label:`Settings`}],R.__docgenInfo={description:`ComponentDetailView is ComponentDetail with its pods and usage given.
An undefined usage is not loaded yet. Without metricsTab, the Metrics
tab shows the live usage of the pods and no charts. Without logsTab or
eventsTab, that tab says there is nothing to show. A podless component
shows no pod numbers and says that it runs no pods. With tab, the caller
holds the open tab, and onTab gets each tab that the user opens.`,methods:[],displayName:`ComponentDetailView`,props:{title:{required:!0,tsType:{name:`string`},description:``},subtitle:{required:!1,tsType:{name:`string`},description:``},conditions:{required:!0,tsType:{name:`Array`,elements:[{name:`ConditionRow`}],raw:`ConditionRow[]`},description:``},ready:{required:!0,tsType:{name:`union`,raw:`ConditionRow | undefined`,elements:[{name:`ConditionRow`},{name:`undefined`}]},description:``},unread:{required:!1,tsType:{name:`string`},description:``},settings:{required:!1,tsType:{name:`ReactNode`},description:``},children:{required:!1,tsType:{name:`ReactNode`},description:``},now:{required:!0,tsType:{name:`string`},description:``},tab:{required:!1,tsType:{name:`union`,raw:`"overview" | "metrics" | "logs" | "events" | "settings"`,elements:[{name:`literal`,value:`"overview"`},{name:`literal`,value:`"metrics"`},{name:`literal`,value:`"logs"`},{name:`literal`,value:`"events"`},{name:`literal`,value:`"settings"`}]},description:``},onTab:{required:!1,tsType:{name:`signature`,type:`function`,raw:`(tab: DetailTab) => void`,signature:{arguments:[{type:{name:`union`,raw:`"overview" | "metrics" | "logs" | "events" | "settings"`,elements:[{name:`literal`,value:`"overview"`},{name:`literal`,value:`"metrics"`},{name:`literal`,value:`"logs"`},{name:`literal`,value:`"events"`},{name:`literal`,value:`"settings"`}]},name:`tab`}],return:{name:`void`}}},description:``},pods:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`ready`,value:{name:`boolean`,required:!0}},{key:`restartCount`,value:{name:`number`,required:!0}}]}}]}],raw:`ContainerStatusLike[]`,required:!1}}]}}],required:!1}}]}}],raw:`PodLike[]`},description:``},podless:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`false`,computed:!1}},usage:{required:!0,tsType:{name:`union`,raw:`PodUsage[] | Unavailable | undefined`,elements:[{name:`Array`,elements:[{name:`PodUsage`}],raw:`PodUsage[]`},{name:`Unavailable`},{name:`undefined`}]},description:``},metricsTab:{required:!1,tsType:{name:`ReactNode`},description:``},logsTab:{required:!1,tsType:{name:`ReactNode`},description:``},eventsTab:{required:!1,tsType:{name:`ReactNode`},description:``}}}})))()}var H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{H=n(),V(),oe(),ce(),U={title:`cluster/ComponentDetail`,component:R,args:{now:j}},W={type:`ZeebeReady`,status:`False`,reason:`Degraded`,message:`2 of 3 pods ready`,lastTransitionTime:`2026-10-30T11:52:00Z`},G={type:`OperateReady`,status:`True`,reason:`Disabled`,message:``,lastTransitionTime:`2026-10-29T08:00:00Z`},K={type:`TasklistReady`,status:`True`,reason:`Healthy`,message:``,lastTransitionTime:`2026-10-29T08:00:00Z`},q={name:`Brokers degraded`,args:{title:`Brokers`,conditions:[W],ready:W,pods:se,usage:[M(`orders-zeebe-0`,.8,2.6),M(`orders-zeebe-1`,1.1,3.1)],children:(0,H.jsx)(le,{partitions:ae})}},J={name:`Embedded Operate in the gateway`,args:{title:`Operate`,subtitle:`Embedded in Gateway`,conditions:[G],ready:G,pods:[A(`orders-gateway-0`,`pool-a-1`)],usage:[M(`orders-gateway-0`,.3,1.2)]}},Y={name:`Usage not available`,args:{title:`Tasklist`,conditions:[K],ready:K,pods:[A(`orders-tasklist-0`,`pool-a-2`)],usage:`unavailable`}},X={type:`Ready`,status:`False`,reason:`WaitingForEndpoints`,message:`The gateway has no address yet`,lastTransitionTime:`2026-10-30T11:58:00Z`},Z={name:`Component without pods`,args:{title:`Access`,subtitle:`Gateway access`,conditions:[X],ready:X,pods:[],usage:[],podless:!0}},Q=[`BrokersDegraded`,`EmbeddedOperateInTheGateway`,`UsageNotAvailable`,`ComponentWithoutPods`],q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "Brokers degraded",
  args: {
    title: "Brokers",
    conditions: [zeebeDegraded],
    ready: zeebeDegraded,
    pods: brokers,
    usage: [usage("orders-zeebe-0", 0.8, 2.6), usage("orders-zeebe-1", 1.1, 3.1)],
    children: <PartitionTable partitions={degraded} />
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Embedded Operate in the gateway",
  args: {
    title: "Operate",
    subtitle: "Embedded in Gateway",
    conditions: [operateDisabled],
    ready: operateDisabled,
    pods: [pod("orders-gateway-0", "pool-a-1")],
    usage: [usage("orders-gateway-0", 0.3, 1.2)]
  }
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Usage not available",
  args: {
    title: "Tasklist",
    conditions: [tasklistHealthy],
    ready: tasklistHealthy,
    pods: [pod("orders-tasklist-0", "pool-a-2")],
    usage: "unavailable"
  }
}`,...Y.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Component without pods",
  args: {
    title: "Access",
    subtitle: "Gateway access",
    conditions: [accessWaiting],
    ready: accessWaiting,
    pods: [],
    usage: [],
    podless: true
  }
}`,...Z.parameters?.docs?.source}}}})))()}$();export{q as BrokersDegraded,Z as ComponentWithoutPods,J as EmbeddedOperateInTheGateway,Y as UsageNotAvailable,Q as __namedExportsOrder,U as default};