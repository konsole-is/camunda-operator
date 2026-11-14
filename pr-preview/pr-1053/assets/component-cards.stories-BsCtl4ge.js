import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{f as n,l as r,o as i,u as a}from"./labels-lxF_8UjP.js";import{a as o,i as s,n as c,t as l}from"./topology-PI2JSrmt.js";import{i as u,n as d,r as f,t as p}from"./camundaoptimize-BTOwQKfp.js";import{t as m}from"./placement-BPmEgwCN.js";import{n as h,t as g}from"./status-dot-XlX2dgDj.js";function _(e){let t=[...e.pods].sort((e,t)=>(e.metadata?.name??``).localeCompare(t.metadata?.name??``)),n=e=>t.filter(t=>e.includes(t.metadata?.labels?.[`camunda.io/component`]??``)),i=(t,n)=>[v(n),t===`zeebe`?b(e.partitions):y(e.usage,S(n))],s=o(e.spec,e.pods).map(t=>{let r=n([t.process]);return{id:t.process,label:t.label,used:!0,condition:m(f,e.conditions,t.process)[0],embedded:t.embedded,pods:S(r),numbers:i(t.process,r)}});if(s.some(e=>e.id===`connectors`)||s.push(x(`connectors`,l.connectors)),!e.optimize)s.push(x(`optimize`,C));else{let t=e.optimize.metadata?.uid,o=n(r).filter(e=>e.metadata?.labels?.[a]===t);s.push({id:`optimize`,label:C,used:!0,condition:m(p,e.optimize.status?.conditions,`optimize`).find(e=>e.type===w),embedded:[],pods:S(o),numbers:i(`optimize`,o)})}return s}function v(e){return{label:`Pods ready`,value:`${e.filter(e=>s(e).tone===`ok`).length}/${e.length}`}}function y(e,t){let n=`Highest use`;if(e===void 0)return{label:n,value:`Loading`};if(e===`unavailable`)return{label:n,value:`Not available`};let r=e.filter(e=>t.includes(e.pod));if(r.length===0)return{label:n,value:`Not available`};let i;for(let e of r)for(let[t,n,r]of[[`cpu`,e.cpu.usedCores,e.cpu.limitCores],[`memory`,e.memory.usedBytes,e.memory.limitBytes]]){if(!r)continue;let e=n/r;(!i||e>i.share)&&(i={share:e,resource:t})}return i?{label:n,value:`${Math.round(i.share*100)} % ${i.resource}`}:{label:n,value:`No limit`}}function b(e){let t=`Partitions with a leader`;return e===void 0?{label:t,value:`Loading`}:e===`unavailable`?{label:t,value:`Not available`}:{label:t,value:`${e.filter(e=>e.leader).length}/${e.length}`}}function x(e,t){return{id:e,label:t,used:!1,embedded:[],pods:[],numbers:[]}}function S(e){return e.map(e=>e.metadata?.name??``)}var C,w;function T(){return(T=e((()=>{u(),d(),n(),c(),C=`Optimize`,w=`Ready`})))()}function E({cards:e,hrefOf:t,onOpen:n}){return(0,k.jsxs)(`section`,{"aria-label":`Components`,className:`flex flex-col gap-3 rounded-lg border border-border bg-panel p-4`,children:[(0,k.jsx)(`h2`,{className:`font-heading text-base font-semibold`,children:`Components`}),(0,k.jsx)(`ul`,{className:`grid gap-3 sm:grid-cols-[repeat(auto-fill,minmax(14rem,1fr))]`,children:e.map(e=>(0,k.jsx)(`li`,{className:`flex`,children:(0,k.jsx)(D,{card:e,hrefOf:e.used?t:void 0,onOpen:n})},e.id))})]})}function D({card:e,hrefOf:t,onOpen:n}){let r=t?.(e.id),i=e.used?`border-border bg-panel-2`:`border-dashed border-faint bg-panel`;return(0,k.jsxs)(`article`,{"aria-label":e.label,className:`relative flex w-full flex-col gap-3 rounded-md border p-3 ${i} ${r?`hover:bg-panel`:``}`,children:[(0,k.jsxs)(`div`,{className:`flex items-center gap-2`,children:[(0,k.jsx)(`span`,{className:`relative z-10 flex`,children:(0,k.jsx)(g,{label:e.label,condition:e.condition,off:!e.used})}),r?(0,k.jsx)(`a`,{href:r,onClick:O(e.id,n),className:`font-medium text-text outline-none after:absolute after:inset-0 after:rounded-md focus-visible:after:ring-3 focus-visible:after:ring-ring/50`,children:e.label}):(0,k.jsx)(`span`,{className:`font-medium text-text`,children:e.label})]}),e.embedded.length>0&&(0,k.jsx)(`ul`,{className:`flex flex-wrap gap-1.5`,children:e.embedded.map(e=>{let r=t?.(e.process),i=`rounded border border-dashed border-faint px-2 py-0.5 text-xs text-text-2`;return(0,k.jsx)(`li`,{className:`flex`,children:r?(0,k.jsxs)(`a`,{href:r,onClick:O(e.process,n),className:`relative z-10 ${i} outline-none hover:border-text-2 hover:text-text focus-visible:ring-3 focus-visible:ring-ring/50`,children:[`Embedded: `,e.label]}):(0,k.jsxs)(`span`,{className:i,children:[`Embedded: `,e.label]})},e.process)})}),e.used?(0,k.jsx)(`dl`,{className:`mt-auto grid grid-cols-2 gap-3`,children:e.numbers.map(e=>(0,k.jsxs)(`div`,{className:`flex flex-col-reverse gap-0.5`,children:[(0,k.jsx)(`dt`,{className:`text-xs text-muted`,children:e.label}),(0,k.jsx)(`dd`,{className:`font-heading text-lg font-semibold text-text tabular-nums`,children:e.value})]},e.label))}):(0,k.jsx)(`p`,{className:`mt-auto text-sm text-muted`,children:`Not used`})]})}function O(e,t){if(t)return n=>{n.button!==0||n.metaKey||n.ctrlKey||n.shiftKey||n.altKey||(n.preventDefault(),t(e))}}var k;function A(){return(A=e((()=>{k=t(),h(),E.__docgenInfo={description:`ComponentCards shows one card for each model, in the order given.
hrefOf gives the link of a used card and of each app embedded in it. A
card without one, and a card that is not used, is no link. onOpen, when
set, takes a plain click on a link in place of the navigation.`,methods:[],displayName:`ComponentCards`,props:{cards:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  id: CardId;
  label: string;
  used: boolean;
  condition?: Condition;
  embedded: { process: Process; label: string }[];
  pods: string[];
  numbers: CardNumber[];
}`,signature:{properties:[{key:`id`,value:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`,required:!0},{name:`literal`,value:`"optimize"`}],required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`used`,value:{name:`boolean`,required:!0}},{key:`condition`,value:{name:`intersection`,raw:`ConditionLike & { type: string }`,elements:[{name:`ConditionLike`},{name:`signature`,type:`object`,raw:`{ type: string }`,signature:{properties:[{key:`type`,value:{name:`string`,required:!0}}]}}],required:!1}},{key:`embedded`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ process: Process; label: string }`,signature:{properties:[{key:`process`,value:{name:`CoreComponent`,required:!0}},{key:`label`,value:{name:`string`,required:!0}}]}}],raw:`{ process: Process; label: string }[]`,required:!0}},{key:`pods`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`numbers`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ label: string; value: string }`,signature:{properties:[{key:`label`,value:{name:`string`,required:!0}},{key:`value`,value:{name:`string`,required:!0}}]}}],raw:`CardNumber[]`,required:!0}}]}}],raw:`ComponentCardModel[]`},description:``},hrefOf:{required:!1,tsType:{name:`signature`,type:`function`,raw:`(id: CardId) => string | undefined`,signature:{arguments:[{type:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},name:`id`}],return:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]}}},description:``},onOpen:{required:!1,tsType:{name:`signature`,type:`function`,raw:`(id: CardId) => void`,signature:{arguments:[{type:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},name:`id`}],return:{name:`void`}}},description:``}}}})))()}function j(e,t,n=!0){return{kind:`Pod`,metadata:{name:e,labels:{[i]:t,...t.startsWith(`optimize-`)&&{"camunda.io/optimize-uid":`uid-orders-optimize`}}},status:n?{phase:`Running`,conditions:[{type:`Ready`,status:`True`}]}:{phase:`Running`,containerStatuses:[{name:`zeebe`,image:`camunda/zeebe`,imageID:``,ready:!1,restartCount:4,state:{waiting:{reason:`CrashLoopBackOff`}}}]}}}function M(e,t,n){return{pod:e,cpu:{usedCores:t,limitCores:1},memory:{usedBytes:n,limitBytes:2*1024**3}}}var N,P,F,I,L,R,z,B,V,H,U,W;function G(){return(G=e((()=>{n(),T(),A(),{fn:N}=__STORYBOOK_MODULE_TEST__,P=e=>({type:e,status:`True`,reason:`Healthy`}),F={operate:{mode:`Standalone`},tasklist:{mode:`Standalone`},admin:{mode:`Standalone`},connectors:{enabled:!0}},I=[j(`orders-zeebe-0`,`zeebe`),j(`orders-zeebe-1`,`zeebe`),j(`orders-zeebe-2`,`zeebe`),j(`orders-gateway-7c9f-a`,`gateway`),j(`orders-operate-5d8b-a`,`operate`),j(`orders-tasklist-6b4c-a`,`tasklist`),j(`orders-admin-8f2d-a`,`admin`),j(`orders-connectors-4a1e-a`,`connectors`),j(`orders-optimize-webapp-a`,`optimize-webapp`),j(`orders-optimize-importer-a`,`optimize-importer`)],L={kind:`CamundaOptimize`,metadata:{name:`orders-optimize`,namespace:`team-a`,uid:`uid-orders-optimize`},spec:{clusterRef:{name:`orders`}},status:{conditions:[P(`Ready`)]}},R=[{partition:1,leader:`orders-zeebe-0`,followers:[`orders-zeebe-1`,`orders-zeebe-2`],health:`unknown`},{partition:2,leader:`orders-zeebe-1`,followers:[`orders-zeebe-0`,`orders-zeebe-2`],health:`unknown`},{partition:3,leader:`orders-zeebe-2`,followers:[`orders-zeebe-0`,`orders-zeebe-1`],health:`unknown`}],z={title:`cluster/ComponentCards`,component:E,args:{onOpen:N()}},B={name:`All used and ready`,args:{cards:_({spec:F,pods:I,conditions:[`ZeebeReady`,`GatewayReady`,`OperateReady`,`TasklistReady`,`AdminReady`,`ConnectorsReady`].map(P),optimize:L,usage:I.map((e,t)=>M(e.metadata?.name??``,.1*(t+1),.8*1024**3)),partitions:R.map(e=>({...e,followers:[...e.followers]}))}),hrefOf:e=>`?tab=overview&component=${e}`}},V={name:`Degraded brokers`,args:{cards:_({spec:{},pods:[j(`orders-zeebe-0`,`zeebe`),j(`orders-zeebe-1`,`zeebe`),j(`orders-zeebe-2`,`zeebe`,!1),j(`orders-gateway-7c9f-a`,`gateway`)],conditions:[{type:`ZeebeReady`,status:`False`,reason:`Degraded`,message:`2 of 3 pods are ready`},P(`GatewayReady`)],usage:[M(`orders-gateway-7c9f-a`,.35,1.1*1024**3)],partitions:[{partition:1,leader:`orders-zeebe-0`,followers:[`orders-zeebe-1`],health:`unknown`},{partition:2,leader:`orders-zeebe-1`,followers:[`orders-zeebe-0`],health:`unknown`},{partition:3,followers:[`orders-zeebe-0`],health:`unknown`}]})}},H={name:`Embedded apps, Connectors and Optimize not used`,args:{cards:_({spec:{},pods:[j(`orders-zeebe-0`,`zeebe`),j(`orders-gateway-7c9f-a`,`gateway`)],conditions:[P(`ZeebeReady`),P(`GatewayReady`),{type:`OperateReady`,status:`True`,reason:`Disabled`}],usage:[M(`orders-gateway-7c9f-a`,.2,.5*1024**3)],partitions:[{partition:1,leader:`orders-zeebe-0`,followers:[],health:`unknown`}]})}},U={name:`Usage not available`,args:{cards:_({spec:F,pods:I,conditions:[P(`ZeebeReady`),P(`GatewayReady`)],optimize:L,usage:`unavailable`,partitions:`unavailable`})}},W=[`AllUsedAndReady`,`DegradedBrokers`,`EmbeddedAppsConnectorsAndOptimizeNotUsed`,`UsageNotAvailable`],B.parameters={...B.parameters,docs:{...B.parameters?.docs,source:{originalSource:`{
  name: "All used and ready",
  args: {
    cards: componentCards({
      spec: standalone,
      pods: allPods,
      conditions: ["ZeebeReady", "GatewayReady", "OperateReady", "TasklistReady", "AdminReady", "ConnectorsReady"].map(healthy),
      optimize,
      usage: allPods.map((p, i) => usage(p.metadata?.name ?? "", 0.1 * (i + 1), 0.8 * 1024 ** 3)),
      partitions: threePartitions.map(p => ({
        ...p,
        followers: [...p.followers]
      }))
    }),
    hrefOf: id => \`?tab=overview&component=\${id}\`
  }
}`,...B.parameters?.docs?.source}}},V.parameters={...V.parameters,docs:{...V.parameters?.docs,source:{originalSource:`{
  name: "Degraded brokers",
  args: {
    cards: componentCards({
      spec: {} as CamundaClusterSpec,
      pods: [pod("orders-zeebe-0", "zeebe"), pod("orders-zeebe-1", "zeebe"), pod("orders-zeebe-2", "zeebe", false), pod("orders-gateway-7c9f-a", "gateway")],
      conditions: [{
        type: "ZeebeReady",
        status: "False",
        reason: "Degraded",
        message: "2 of 3 pods are ready"
      }, healthy("GatewayReady")],
      usage: [usage("orders-gateway-7c9f-a", 0.35, 1.1 * 1024 ** 3)],
      partitions: [{
        partition: 1,
        leader: "orders-zeebe-0",
        followers: ["orders-zeebe-1"],
        health: "unknown"
      }, {
        partition: 2,
        leader: "orders-zeebe-1",
        followers: ["orders-zeebe-0"],
        health: "unknown"
      }, {
        partition: 3,
        followers: ["orders-zeebe-0"],
        health: "unknown"
      }]
    })
  }
}`,...V.parameters?.docs?.source}}},H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
  name: "Embedded apps, Connectors and Optimize not used",
  args: {
    cards: componentCards({
      spec: {} as CamundaClusterSpec,
      pods: [pod("orders-zeebe-0", "zeebe"), pod("orders-gateway-7c9f-a", "gateway")],
      conditions: [healthy("ZeebeReady"), healthy("GatewayReady"), {
        type: "OperateReady",
        status: "True",
        reason: "Disabled"
      }],
      usage: [usage("orders-gateway-7c9f-a", 0.2, 0.5 * 1024 ** 3)],
      partitions: [{
        partition: 1,
        leader: "orders-zeebe-0",
        followers: [],
        health: "unknown"
      }]
    })
  }
}`,...H.parameters?.docs?.source}}},U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
  name: "Usage not available",
  args: {
    cards: componentCards({
      spec: standalone,
      pods: allPods,
      conditions: [healthy("ZeebeReady"), healthy("GatewayReady")],
      optimize,
      usage: "unavailable",
      partitions: "unavailable"
    })
  }
}`,...U.parameters?.docs?.source}}}})))()}G();export{B as AllUsedAndReady,V as DegradedBrokers,H as EmbeddedAppsConnectorsAndOptimizeNotUsed,U as UsageNotAvailable,W as __namedExportsOrder,z as default};