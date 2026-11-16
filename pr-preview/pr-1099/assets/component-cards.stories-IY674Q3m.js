import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{h as n,o as r}from"./labels-a4ptf10d.js";import{r as i,t as a}from"./cards-BY3mzYH7.js";import{n as o,t as s}from"./status-dot-BrpKHAiR.js";import{n as c,r as l}from"./stat-tile-BSk7ytU6.js";function u({cards:e,hrefOf:t,onOpen:n}){return(0,p.jsxs)(`section`,{"aria-label":`Components`,className:`flex flex-col gap-3 rounded-lg border border-border bg-panel p-4`,children:[(0,p.jsx)(`h2`,{className:`font-heading text-base font-semibold`,children:`Components`}),(0,p.jsx)(`ul`,{className:`grid gap-3 sm:grid-cols-[repeat(auto-fill,minmax(14rem,1fr))]`,children:e.map(e=>(0,p.jsx)(`li`,{className:`flex`,children:(0,p.jsx)(d,{card:e,hrefOf:e.used?t:void 0,onOpen:n})},e.id))})]})}function d({card:e,hrefOf:t,onOpen:n}){let r=t?.(e.id),i=e.used?`border-border bg-panel-2`:`border-dashed border-faint bg-panel`;return(0,p.jsxs)(`article`,{"aria-label":e.label,className:`relative flex w-full flex-col gap-3 rounded-md border p-3 ${i} ${r?`hover:bg-panel`:``}`,children:[(0,p.jsxs)(`div`,{className:`flex items-center gap-2`,children:[(0,p.jsx)(`span`,{className:`relative z-10 flex`,children:(0,p.jsx)(s,{label:e.label,condition:e.condition,off:!e.used})}),r?(0,p.jsx)(`a`,{href:r,onClick:f(e.id,n),className:`font-medium text-text outline-none after:absolute after:inset-0 after:rounded-md focus-visible:after:ring-3 focus-visible:after:ring-ring/50`,children:e.label}):(0,p.jsx)(`span`,{className:`font-medium text-text`,children:e.label})]}),e.embedded.length>0&&(0,p.jsx)(`ul`,{className:`flex flex-wrap gap-1.5`,children:e.embedded.map(e=>{let r=t?.(e.process),i=`rounded border border-dashed border-faint px-2 py-0.5 text-xs text-text-2`;return(0,p.jsx)(`li`,{className:`flex`,children:r?(0,p.jsxs)(`a`,{href:r,onClick:f(e.process,n),className:`relative z-10 ${i} outline-none hover:border-text-2 hover:text-text focus-visible:ring-3 focus-visible:ring-ring/50`,children:[`Embedded: `,e.label]}):(0,p.jsxs)(`span`,{className:i,children:[`Embedded: `,e.label]})},e.process)})}),e.used?(0,p.jsx)(`dl`,{className:`mt-auto grid grid-cols-2 gap-3`,children:e.numbers.map(e=>(0,p.jsxs)(`div`,{className:`flex flex-col-reverse gap-0.5`,children:[(0,p.jsx)(`dt`,{className:`text-xs text-muted`,children:e.label}),(0,p.jsx)(`dd`,{className:l(e.value),children:e.value})]},e.label))}):(0,p.jsx)(`p`,{className:`mt-auto text-sm text-muted`,children:`Not used`})]})}function f(e,t){if(t)return n=>{n.button!==0||n.metaKey||n.ctrlKey||n.shiftKey||n.altKey||(n.preventDefault(),t(e))}}var p;function m(){return(m=e((()=>{p=t(),o(),c(),u.__docgenInfo={description:`ComponentCards shows one card for each model, in the order given.
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
}`,signature:{properties:[{key:`id`,value:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`,required:!0},{name:`literal`,value:`"optimize"`}],required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`used`,value:{name:`boolean`,required:!0}},{key:`condition`,value:{name:`intersection`,raw:`ConditionLike & { type: string }`,elements:[{name:`ConditionLike`},{name:`signature`,type:`object`,raw:`{ type: string }`,signature:{properties:[{key:`type`,value:{name:`string`,required:!0}}]}}],required:!1}},{key:`embedded`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ process: Process; label: string }`,signature:{properties:[{key:`process`,value:{name:`CoreComponent`,required:!0}},{key:`label`,value:{name:`string`,required:!0}}]}}],raw:`{ process: Process; label: string }[]`,required:!0}},{key:`pods`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`numbers`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ label: string; value: string }`,signature:{properties:[{key:`label`,value:{name:`string`,required:!0}},{key:`value`,value:{name:`string`,required:!0}}]}}],raw:`CardNumber[]`,required:!0}}]}}],raw:`ComponentCardModel[]`},description:``},hrefOf:{required:!1,tsType:{name:`signature`,type:`function`,raw:`(id: CardId) => string | undefined`,signature:{arguments:[{type:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},name:`id`}],return:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]}}},description:``},onOpen:{required:!1,tsType:{name:`signature`,type:`function`,raw:`(id: CardId) => void`,signature:{arguments:[{type:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},name:`id`}],return:{name:`void`}}},description:``}}}})))()}function h(e,t,n=!0){return{kind:`Pod`,metadata:{name:e,labels:{[r]:t,...t.startsWith(`optimize-`)&&{"camunda.io/optimize-uid":`uid-orders-optimize`}}},status:n?{phase:`Running`,conditions:[{type:`Ready`,status:`True`}]}:{phase:`Running`,containerStatuses:[{name:`zeebe`,image:`camunda/zeebe`,imageID:``,ready:!1,restartCount:4,state:{waiting:{reason:`CrashLoopBackOff`}}}]}}}function g(e,t,n){return{pod:e,cpu:{usedCores:t,limitCores:1},memory:{usedBytes:n,limitBytes:2*1024**3}}}var _,v,y,b,x,S,C,w,T,E,D,O;function k(){return(k=e((()=>{n(),i(),m(),{fn:_}=__STORYBOOK_MODULE_TEST__,v=e=>({type:e,status:`True`,reason:`Healthy`}),y={operate:{mode:`Standalone`},tasklist:{mode:`Standalone`},admin:{mode:`Standalone`},connectors:{enabled:!0}},b=[h(`orders-zeebe-0`,`zeebe`),h(`orders-zeebe-1`,`zeebe`),h(`orders-zeebe-2`,`zeebe`),h(`orders-gateway-7c9f-a`,`gateway`),h(`orders-operate-5d8b-a`,`operate`),h(`orders-tasklist-6b4c-a`,`tasklist`),h(`orders-admin-8f2d-a`,`admin`),h(`orders-connectors-4a1e-a`,`connectors`),h(`orders-optimize-webapp-a`,`optimize-webapp`),h(`orders-optimize-importer-a`,`optimize-importer`)],x={kind:`CamundaOptimize`,metadata:{name:`orders-optimize`,namespace:`team-a`,uid:`uid-orders-optimize`},spec:{clusterRef:{name:`orders`}},status:{conditions:[v(`Ready`)]}},S=[{partition:1,leader:`orders-zeebe-0`,followers:[`orders-zeebe-1`,`orders-zeebe-2`],health:`unknown`},{partition:2,leader:`orders-zeebe-1`,followers:[`orders-zeebe-0`,`orders-zeebe-2`],health:`unknown`},{partition:3,leader:`orders-zeebe-2`,followers:[`orders-zeebe-0`,`orders-zeebe-1`],health:`unknown`}],C={title:`cluster/ComponentCards`,component:u,args:{onOpen:_()}},w={name:`All used and ready`,args:{cards:a({spec:y,pods:b,conditions:[`ZeebeReady`,`GatewayReady`,`OperateReady`,`TasklistReady`,`AdminReady`,`ConnectorsReady`].map(v),optimize:x,usage:b.map((e,t)=>g(e.metadata?.name??``,.1*(t+1),.8*1024**3)),partitions:S.map(e=>({...e,followers:[...e.followers]}))}),hrefOf:e=>`?tab=overview&component=${e}`}},T={name:`Degraded brokers`,args:{cards:a({spec:{},pods:[h(`orders-zeebe-0`,`zeebe`),h(`orders-zeebe-1`,`zeebe`),h(`orders-zeebe-2`,`zeebe`,!1),h(`orders-gateway-7c9f-a`,`gateway`)],conditions:[{type:`ZeebeReady`,status:`False`,reason:`Degraded`,message:`2 of 3 pods are ready`},v(`GatewayReady`)],usage:[g(`orders-gateway-7c9f-a`,.35,1.1*1024**3)],partitions:[{partition:1,leader:`orders-zeebe-0`,followers:[`orders-zeebe-1`],health:`unknown`},{partition:2,leader:`orders-zeebe-1`,followers:[`orders-zeebe-0`],health:`unknown`},{partition:3,followers:[`orders-zeebe-0`],health:`unknown`}]})}},E={name:`Embedded apps, Connectors and Optimize not used`,args:{cards:a({spec:{},pods:[h(`orders-zeebe-0`,`zeebe`),h(`orders-gateway-7c9f-a`,`gateway`)],conditions:[v(`ZeebeReady`),v(`GatewayReady`),{type:`OperateReady`,status:`True`,reason:`Disabled`}],usage:[g(`orders-gateway-7c9f-a`,.2,.5*1024**3)],partitions:[{partition:1,leader:`orders-zeebe-0`,followers:[],health:`unknown`}]})}},D={name:`Usage not available`,args:{cards:a({spec:y,pods:b,conditions:[v(`ZeebeReady`),v(`GatewayReady`)],optimize:x,usage:`unavailable`,partitions:`unavailable`})}},O=[`AllUsedAndReady`,`DegradedBrokers`,`EmbeddedAppsConnectorsAndOptimizeNotUsed`,`UsageNotAvailable`],w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
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
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
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
}`,...T.parameters?.docs?.source}}},E.parameters={...E.parameters,docs:{...E.parameters?.docs,source:{originalSource:`{
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
}`,...E.parameters?.docs?.source}}},D.parameters={...D.parameters,docs:{...D.parameters?.docs,source:{originalSource:`{
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
}`,...D.parameters?.docs?.source}}}})))()}k();export{w as AllUsedAndReady,T as DegradedBrokers,E as EmbeddedAppsConnectorsAndOptimizeNotUsed,D as UsageNotAvailable,O as __namedExportsOrder,C as default};