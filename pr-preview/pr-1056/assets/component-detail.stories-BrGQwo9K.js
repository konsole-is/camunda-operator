import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{t as n}from"./hooks-Cuq9NGnJ.js";import{a as r,i,n as a,r as o,t as s}from"./tabs-nUIOMVms.js";import{n as c,t as l}from"./conditions-table-LsPXyfdj.js";import{r as u,t as d}from"./pod-table-CnXWdE2G.js";import{t as f}from"./use-observe-ChGTcfBH.js";import{n as p}from"./event-list-LVw0MuWn.js";import{r as m}from"./detail-logs-Ce-FOWv1.js";import{n as h,r as g}from"./detail-metrics-CNXytxYX.js";import{a as _,i as v,n as y,r as b,s as x,t as S}from"./fixtures-Dju11n-g.js";import{n as C,t as w}from"./partition-table-D3nSdy71.js";function T(){return(T=e((()=>{t(),p(),n()})))()}function E({title:e,subtitle:t,conditions:n,pods:r,usage:c,settings:u,metricsTab:f,logsTab:p,eventsTab:m,children:g,now:_}){return(0,D.jsxs)(`section`,{"aria-label":e,className:`flex flex-col gap-4 rounded-lg border border-border bg-panel p-4`,children:[(0,D.jsxs)(`header`,{className:`flex flex-col gap-0.5`,children:[(0,D.jsx)(`h2`,{tabIndex:-1,className:`font-heading text-lg font-semibold text-text outline-none`,children:e}),t&&(0,D.jsx)(`p`,{className:`text-sm text-muted`,children:t})]}),(0,D.jsxs)(s,{defaultValue:`overview`,children:[(0,D.jsx)(o,{variant:`line`,children:O.map(e=>(0,D.jsx)(i,{value:e.value,children:e.label},e.value))}),(0,D.jsx)(a,{value:`overview`,children:(0,D.jsxs)(`div`,{className:`flex flex-col gap-4 pt-2`,children:[(0,D.jsx)(l,{conditions:n,label:`${e} conditions`,now:_}),(0,D.jsx)(d,{pods:r,usage:c??[],now:_}),g]})}),(0,D.jsx)(a,{value:`metrics`,children:f??(0,D.jsx)(h,{pods:r,usage:c,range:`1h`})}),(0,D.jsx)(a,{value:`logs`,children:p??(0,D.jsx)(`p`,{className:`pt-2 text-sm text-muted`,children:`No logs.`})}),(0,D.jsx)(a,{value:`events`,children:m??(0,D.jsx)(`p`,{className:`pt-2 text-sm text-muted`,children:`No events`})}),(0,D.jsx)(a,{value:`settings`,children:(0,D.jsx)(`div`,{className:`pt-2`,children:u??(0,D.jsx)(`p`,{className:`text-sm text-muted`,children:`No settings.`})})})]})]})}var D,O;function k(){return(k=e((()=>{D=t(),c(),u(),r(),n(),f(),T(),m(),g(),O=[{value:`overview`,label:`Overview`},{value:`metrics`,label:`Metrics`},{value:`logs`,label:`Logs`},{value:`events`,label:`Events`},{value:`settings`,label:`Settings`}],E.__docgenInfo={description:`ComponentDetailView is ComponentDetail with its pods and usage given.
An undefined usage is not loaded yet. Without metricsTab, the Metrics
tab shows the live usage of the pods and no charts. Without logsTab or
eventsTab, that tab says there is nothing to show.`,methods:[],displayName:`ComponentDetailView`,props:{title:{required:!0,tsType:{name:`string`},description:``},subtitle:{required:!1,tsType:{name:`string`},description:``},conditions:{required:!0,tsType:{name:`Array`,elements:[{name:`ConditionRow`}],raw:`ConditionRow[]`},description:``},settings:{required:!1,tsType:{name:`ReactNode`},description:``},children:{required:!1,tsType:{name:`ReactNode`},description:``},now:{required:!0,tsType:{name:`string`},description:``},pods:{required:!0,tsType:{name:`Array`,elements:[{name:`PodLike`}],raw:`PodLike[]`},description:``},usage:{required:!0,tsType:{name:`union`,raw:`PodUsage[] | Unavailable | undefined`,elements:[{name:`Array`,elements:[{name:`PodUsage`}],raw:`PodUsage[]`},{name:`Unavailable`},{name:`undefined`}]},description:``},metricsTab:{required:!1,tsType:{name:`ReactNode`},description:``},logsTab:{required:!1,tsType:{name:`ReactNode`},description:``},eventsTab:{required:!1,tsType:{name:`ReactNode`},description:``}}}})))()}var A,j,M,N,P,F;function I(){return(I=e((()=>{A=t(),k(),b(),C(),j={title:`cluster/ComponentDetail`,component:E,args:{now:v}},M={name:`Brokers degraded`,args:{title:`Brokers`,conditions:[{type:`ZeebeReady`,status:`False`,reason:`Degraded`,message:`2 of 3 pods ready`,lastTransitionTime:`2026-10-30T11:52:00Z`}],pods:S,usage:[x(`orders-zeebe-0`,.8,2.6),x(`orders-zeebe-1`,1.1,3.1)],children:(0,A.jsx)(w,{partitions:y})}},N={name:`Embedded Operate in the gateway`,args:{title:`Operate`,subtitle:`Embedded in Gateway`,conditions:[{type:`OperateReady`,status:`True`,reason:`Disabled`,message:``,lastTransitionTime:`2026-10-29T08:00:00Z`}],pods:[_(`orders-gateway-0`,`pool-a-1`)],usage:[x(`orders-gateway-0`,.3,1.2)]}},P={name:`Usage not available`,args:{title:`Tasklist`,conditions:[{type:`TasklistReady`,status:`True`,reason:`Healthy`,message:``,lastTransitionTime:`2026-10-29T08:00:00Z`}],pods:[_(`orders-tasklist-0`,`pool-a-2`)],usage:`unavailable`}},F=[`BrokersDegraded`,`EmbeddedOperateInTheGateway`,`UsageNotAvailable`],M.parameters={...M.parameters,docs:{...M.parameters?.docs,source:{originalSource:`{
  name: "Brokers degraded",
  args: {
    title: "Brokers",
    conditions: [{
      type: "ZeebeReady",
      status: "False",
      reason: "Degraded",
      message: "2 of 3 pods ready",
      lastTransitionTime: "2026-10-30T11:52:00Z"
    }],
    pods: brokers,
    usage: [usage("orders-zeebe-0", 0.8, 2.6), usage("orders-zeebe-1", 1.1, 3.1)],
    children: <PartitionTable partitions={degraded} />
  }
}`,...M.parameters?.docs?.source}}},N.parameters={...N.parameters,docs:{...N.parameters?.docs,source:{originalSource:`{
  name: "Embedded Operate in the gateway",
  args: {
    title: "Operate",
    subtitle: "Embedded in Gateway",
    conditions: [{
      type: "OperateReady",
      status: "True",
      reason: "Disabled",
      message: "",
      lastTransitionTime: "2026-10-29T08:00:00Z"
    }],
    pods: [pod("orders-gateway-0", "pool-a-1")],
    usage: [usage("orders-gateway-0", 0.3, 1.2)]
  }
}`,...N.parameters?.docs?.source}}},P.parameters={...P.parameters,docs:{...P.parameters?.docs,source:{originalSource:`{
  name: "Usage not available",
  args: {
    title: "Tasklist",
    conditions: [{
      type: "TasklistReady",
      status: "True",
      reason: "Healthy",
      message: "",
      lastTransitionTime: "2026-10-29T08:00:00Z"
    }],
    pods: [pod("orders-tasklist-0", "pool-a-2")],
    usage: "unavailable"
  }
}`,...P.parameters?.docs?.source}}}})))()}I();export{M as BrokersDegraded,N as EmbeddedOperateInTheGateway,P as UsageNotAvailable,F as __namedExportsOrder,j as default};