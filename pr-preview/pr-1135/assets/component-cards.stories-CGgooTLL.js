import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{b as n,o as r}from"./labels-CoLYcJ9C.js";import{n as i,t as a}from"./circle-exclamation-point-jpEoxFxv.js";import{n as o,t as s}from"./triangle-exclamation-point-Ds-wrQRI.js";import{a as c,o as l}from"./iframe-BDrm4oIw.js";import{i as u,o as d,r as f,t as p}from"./cards-DSfJuNPW.js";import{n as m,t as h}from"./status-dot-DZ9owZdh.js";import{n as g,r as _}from"./stat-tile-BSk7ytU6.js";function v({cards:e,hrefOf:t,onOpen:n}){return(0,C.jsxs)(`section`,{"aria-label":`Components`,className:`flex flex-col gap-3 rounded-lg border border-border bg-panel p-4`,children:[(0,C.jsx)(`h2`,{className:`font-heading text-base font-semibold`,children:`Components`}),(0,C.jsx)(`ul`,{className:`grid gap-3 sm:grid-cols-[repeat(auto-fill,minmax(14rem,1fr))]`,children:e.map(e=>(0,C.jsx)(`li`,{className:`flex`,children:(0,C.jsx)(y,{card:e,hrefOf:e.used?t:void 0,onOpen:n})},e.id))})]})}function y({card:e,hrefOf:t,onOpen:n}){let r=t?.(e.id),i=e.used?`border-border bg-panel-2`:`border-dashed border-faint bg-panel`;return(0,C.jsxs)(`article`,{"aria-label":e.label,className:`relative flex w-full flex-col gap-3 rounded-md border p-3 ${i} ${r?`hover:bg-panel`:``}`,children:[(0,C.jsxs)(`div`,{className:`flex items-center gap-2`,children:[(0,C.jsx)(`span`,{className:`relative z-10 flex`,children:(0,C.jsx)(h,{label:e.label,condition:e.condition,off:!e.used})}),r?(0,C.jsx)(`a`,{href:r,onClick:S(e.id,n),className:`font-medium text-text outline-none after:absolute after:inset-0 after:rounded-md focus-visible:after:ring-3 focus-visible:after:ring-ring/50`,children:e.label}):(0,C.jsx)(`span`,{className:`font-medium text-text`,children:e.label})]}),e.embedded.length>0&&(0,C.jsx)(`ul`,{className:`flex flex-wrap gap-1.5`,children:e.embedded.map(e=>{let r=t?.(e.process),i=`rounded border border-dashed border-faint px-2 py-0.5 text-xs text-text-2`;return(0,C.jsx)(`li`,{className:`flex`,children:r?(0,C.jsxs)(`a`,{href:r,onClick:S(e.process,n),className:`relative z-10 ${i} outline-none hover:border-text-2 hover:text-text focus-visible:ring-3 focus-visible:ring-ring/50`,children:[`Embedded: `,e.label]}):(0,C.jsxs)(`span`,{className:i,children:[`Embedded: `,e.label]})},e.process)})}),e.used?(0,C.jsx)(`dl`,{className:`mt-auto grid grid-cols-2 gap-3`,children:e.numbers.map((e,t)=>(0,C.jsxs)(`div`,{className:`flex min-w-0 flex-col-reverse gap-0.5`,children:[(0,C.jsx)(`dt`,{className:`text-xs text-muted`,children:e.label}),(0,C.jsxs)(`dd`,{className:x(e),children:[e.severity&&(0,C.jsx)(b,{severity:e.severity}),e.value]})]},t))}):(0,C.jsx)(`p`,{className:`mt-auto text-sm text-muted`,children:`Not used`})]})}function b({severity:e}){return(0,C.jsxs)(C.Fragment,{children:[(0,C.jsx)(e===`Error`?a:s,{"aria-hidden":!0,className:`mr-1 inline size-3.5 align-[-0.125em]`}),(0,C.jsxs)(`span`,{className:`sr-only`,children:[e,`: `]})]})}function x(e){return e.severity?c(`text-sm break-words`,e.severity===`Error`?`text-bad`:`text-warn`):c(_(e.value),`break-words`)}function S(e,t){if(t)return n=>{n.button!==0||n.metaKey||n.ctrlKey||n.shiftKey||n.altKey||(n.preventDefault(),t(e))}}var C;function w(){return(w=e((()=>{C=t(),i(),o(),m(),l(),g(),v.__docgenInfo={description:`ComponentCards shows one card for each model, in the order given.
hrefOf gives the link of a used card and of each app embedded in it. A
card without one, and a card that is not used, is no link. onOpen, when
set, takes a plain click on a link in place of the navigation.`,methods:[],displayName:`ComponentCards`,props:{cards:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  id: Id;
  label: string;
  used: boolean;
  condition?: Condition;
  embedded: { process: Id; label: string }[];
  pods: string[];
  numbers: CardNumber[];
}`,signature:{properties:[{key:`id`,value:{name:`Id`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`used`,value:{name:`boolean`,required:!0}},{key:`condition`,value:{name:`intersection`,raw:`ConditionLike & { type: string }`,elements:[{name:`ConditionLike`},{name:`signature`,type:`object`,raw:`{ type: string }`,signature:{properties:[{key:`type`,value:{name:`string`,required:!0}}]}}],required:!1}},{key:`embedded`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ process: Id; label: string }`,signature:{properties:[{key:`process`,value:{name:`Id`,required:!0}},{key:`label`,value:{name:`string`,required:!0}}]}}],raw:`{ process: Id; label: string }[]`,required:!0}},{key:`pods`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`numbers`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  label: string;
  value: string;
  severity?: "Warning" | "Error";
}`,signature:{properties:[{key:`label`,value:{name:`string`,required:!0}},{key:`value`,value:{name:`string`,required:!0}},{key:`severity`,value:{name:`union`,raw:`"Warning" | "Error"`,elements:[{name:`literal`,value:`"Warning"`},{name:`literal`,value:`"Error"`}],required:!1}}]}}],raw:`CardNumber[]`,required:!0}}]}}],raw:`ComponentCardModel<Id>[]`},description:``},hrefOf:{required:!1,tsType:{name:`signature`,type:`function`,raw:`(id: Id) => string | undefined`,signature:{arguments:[{type:{name:`Id`},name:`id`}],return:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]}}},description:``},onOpen:{required:!1,tsType:{name:`signature`,type:`function`,raw:`(id: Id) => void`,signature:{arguments:[{type:{name:`Id`},name:`id`}],return:{name:`void`}}},description:``}}}})))()}function T(e,t,n=!0){return{kind:`Pod`,metadata:{name:e,labels:{[r]:t,...t.startsWith(`optimize-`)&&{"camunda.io/optimize-uid":`uid-orders-optimize`}}},status:n?{phase:`Running`,conditions:[{type:`Ready`,status:`True`}]}:{phase:`Running`,containerStatuses:[{name:`zeebe`,image:`camunda/zeebe`,imageID:``,ready:!1,restartCount:4,state:{waiting:{reason:`CrashLoopBackOff`}}}]}}}function E(e,t,n){return{pod:e,cpu:{usedCores:t,limitCores:1},memory:{usedBytes:n,limitBytes:2*1024**3}}}function D(e,t){return{kind:`PluginRequest`,metadata:{name:e,namespace:`team-a`},status:t}}function O(e,t,n,r,i={}){return{id:`section:${e}`,section:e,label:t,order:n,selection:{section:e,registration:r},...i}}var k,A,j,M,N,P,F,I,L,R,z,B,V,H,U;function W(){return(W=e((()=>{n(),f(),w(),{fn:k}=__STORYBOOK_MODULE_TEST__,A=e=>({type:e,status:`True`,reason:`Healthy`}),j={operate:{mode:`Standalone`},tasklist:{mode:`Standalone`},admin:{mode:`Standalone`},connectors:{enabled:!0}},M=[T(`orders-zeebe-0`,`zeebe`),T(`orders-zeebe-1`,`zeebe`),T(`orders-zeebe-2`,`zeebe`),T(`orders-gateway-7c9f-a`,`gateway`),T(`orders-operate-5d8b-a`,`operate`),T(`orders-tasklist-6b4c-a`,`tasklist`),T(`orders-admin-8f2d-a`,`admin`),T(`orders-connectors-4a1e-a`,`connectors`),T(`orders-optimize-webapp-a`,`optimize-webapp`),T(`orders-optimize-importer-a`,`optimize-importer`)],N={kind:`CamundaOptimize`,metadata:{name:`orders-optimize`,namespace:`team-a`,uid:`uid-orders-optimize`},spec:{clusterRef:{name:`orders`}},status:{conditions:[A(`Ready`)]}},P=[{partition:1,leader:`orders-zeebe-0`,followers:[`orders-zeebe-1`,`orders-zeebe-2`],health:`unknown`},{partition:2,leader:`orders-zeebe-1`,followers:[`orders-zeebe-0`,`orders-zeebe-2`],health:`unknown`},{partition:3,leader:`orders-zeebe-2`,followers:[`orders-zeebe-0`,`orders-zeebe-1`],health:`unknown`}],F={title:`cluster/ComponentCards`,component:v,args:{onOpen:k()}},I=p({spec:j,pods:M,conditions:[`ZeebeReady`,`GatewayReady`,`OperateReady`,`TasklistReady`,`AdminReady`,`ConnectorsReady`].map(A),optimize:N,usage:M.map((e,t)=>E(e.metadata?.name??``,.1*(t+1),.8*1024**3)),partitions:P.map(e=>({...e,followers:[...e.followers]}))}),L={name:`All used and ready`,args:{cards:I,hrefOf:e=>`?tab=overview&component=${e}`}},R=(e,t)=>({kind:e,name:t,onOrphan:`Kept`}),z={name:`Section cards`,args:{cards:[...I,...d([O(`cloud-identity`,`Cloud identity`,1,`aws-iam-role`,{request:D(`orders-cloud-identity`,{summary:[{label:`Role`,value:`orders-camunda`}],resources:[R(`IAMRole`,`orders-camunda`)]}),condition:A(`Ready`)}),O(`backup-storage`,`Backup storage`,2,`aws-s3`,{request:D(`orders-backup-storage`,{summary:[{label:`Bucket`,value:`orders-backups-eu-west-1`}],resources:[R(`S3Bucket`,`orders-backups-eu-west-1`),R(`Grant`,`orders-backup-storage-read`)]}),condition:{status:`False`,reason:`GrantRejected`,message:`Grant orders-backup-storage-read has Ready=False`}}),O(`encryption-key`,`Encryption key`,3,`aws-kms-key`,{request:D(`orders-encryption-key`,{summary:[{label:`Key`,value:`Pending deletion`,severity:`Error`}],resources:[R(`KMSKey`,`orders-encryption-key`)]}),condition:{status:`False`,reason:`KeyPendingDeletion`,message:`The key is scheduled for deletion`}}),O(`access`,`Access`,4,`gateway-access`)])],hrefOf:e=>u(e)}},B={name:`Degraded brokers`,args:{cards:p({spec:{},pods:[T(`orders-zeebe-0`,`zeebe`),T(`orders-zeebe-1`,`zeebe`),T(`orders-zeebe-2`,`zeebe`,!1),T(`orders-gateway-7c9f-a`,`gateway`)],conditions:[{type:`ZeebeReady`,status:`False`,reason:`Degraded`,message:`2 of 3 pods are ready`},A(`GatewayReady`)],usage:[E(`orders-gateway-7c9f-a`,.35,1.1*1024**3)],partitions:[{partition:1,leader:`orders-zeebe-0`,followers:[`orders-zeebe-1`],health:`unknown`},{partition:2,leader:`orders-zeebe-1`,followers:[`orders-zeebe-0`],health:`unknown`},{partition:3,followers:[`orders-zeebe-0`],health:`unknown`}]})}},V={name:`Embedded apps, Connectors and Optimize not used`,args:{cards:p({spec:{},pods:[T(`orders-zeebe-0`,`zeebe`),T(`orders-gateway-7c9f-a`,`gateway`)],conditions:[A(`ZeebeReady`),A(`GatewayReady`),{type:`OperateReady`,status:`True`,reason:`Disabled`}],usage:[E(`orders-gateway-7c9f-a`,.2,.5*1024**3)],partitions:[{partition:1,leader:`orders-zeebe-0`,followers:[],health:`unknown`}]})}},H={name:`Usage not available`,args:{cards:p({spec:j,pods:M,conditions:[A(`ZeebeReady`),A(`GatewayReady`)],optimize:N,usage:`unavailable`,partitions:`unavailable`})}},U=[`AllUsedAndReady`,`SectionCards`,`DegradedBrokers`,`EmbeddedAppsConnectorsAndOptimizeNotUsed`,`UsageNotAvailable`],L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
  name: "All used and ready",
  args: {
    cards: allUsedAndReady,
    hrefOf: id => \`?tab=overview&component=\${id}\`
  }
}`,...L.parameters?.docs?.source}}},z.parameters={...z.parameters,docs:{...z.parameters?.docs,source:{originalSource:`{
  name: "Section cards",
  args: {
    cards: [...allUsedAndReady, ...sectionCards([section("cloud-identity", "Cloud identity", 1, "aws-iam-role", {
      request: request("orders-cloud-identity", {
        summary: [{
          label: "Role",
          value: "orders-camunda"
        }],
        resources: [resource("IAMRole", "orders-camunda")]
      }),
      condition: healthy("Ready")
    }), section("backup-storage", "Backup storage", 2, "aws-s3", {
      request: request("orders-backup-storage", {
        summary: [{
          label: "Bucket",
          value: "orders-backups-eu-west-1"
        }],
        resources: [resource("S3Bucket", "orders-backups-eu-west-1"), resource("Grant", "orders-backup-storage-read")]
      }),
      condition: {
        status: "False",
        reason: "GrantRejected",
        message: "Grant orders-backup-storage-read has Ready=False"
      }
    }), section("encryption-key", "Encryption key", 3, "aws-kms-key", {
      request: request("orders-encryption-key", {
        summary: [{
          label: "Key",
          value: "Pending deletion",
          severity: "Error"
        }],
        resources: [resource("KMSKey", "orders-encryption-key")]
      }),
      condition: {
        status: "False",
        reason: "KeyPendingDeletion",
        message: "The key is scheduled for deletion"
      }
    }), section("access", "Access", 4, "gateway-access")])],
    hrefOf: id => overviewHref(id as CardId)
  }
}`,...z.parameters?.docs?.source}}},B.parameters={...B.parameters,docs:{...B.parameters?.docs,source:{originalSource:`{
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
}`,...B.parameters?.docs?.source}}},V.parameters={...V.parameters,docs:{...V.parameters?.docs,source:{originalSource:`{
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
}`,...V.parameters?.docs?.source}}},H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
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
}`,...H.parameters?.docs?.source}}}})))()}W();export{L as AllUsedAndReady,B as DegradedBrokers,V as EmbeddedAppsConnectorsAndOptimizeNotUsed,z as SectionCards,H as UsageNotAvailable,U as __namedExportsOrder,F as default};