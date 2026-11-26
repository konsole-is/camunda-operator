import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{o as t,x as n}from"./labels-B2zybmgY.js";import{a as r,i,s as a,t as o}from"./cards-BzEtZEdg.js";import{r as s,t as c}from"./component-card-B88o45xV.js";function l(e,n,r=!0){return{kind:`Pod`,metadata:{name:e,labels:{[t]:n,...n.startsWith(`optimize-`)&&{"camunda.io/optimize-uid":`uid-orders-optimize`}}},status:r?{phase:`Running`,conditions:[{type:`Ready`,status:`True`}]}:{phase:`Running`,containerStatuses:[{name:`zeebe`,image:`camunda/zeebe`,imageID:``,ready:!1,restartCount:4,state:{waiting:{reason:`CrashLoopBackOff`}}}]}}}function u(e,t,n){return{pod:e,cpu:{usedCores:t,limitCores:1},memory:{usedBytes:n,limitBytes:2*1024**3}}}function d(e,t){return{kind:`PluginRequest`,metadata:{name:e,namespace:`team-a`},status:t}}function f(e,t,n,r,i={}){return{id:`section:${e}`,section:e,label:t,order:n,selection:{section:e,registration:r},...i}}var p,m,h,g,_,v,y,b,x,S,C,w,T,E,D,O,k;function A(){return(A=e((()=>{n(),i(),s(),{expect:p,fn:m}=__STORYBOOK_MODULE_TEST__,h=e=>({type:e,status:`True`,reason:`Healthy`}),g={operate:{mode:`Standalone`},tasklist:{mode:`Standalone`},admin:{mode:`Standalone`},connectors:{enabled:!0}},_=[l(`orders-zeebe-0`,`zeebe`),l(`orders-zeebe-1`,`zeebe`),l(`orders-zeebe-2`,`zeebe`),l(`orders-gateway-7c9f-a`,`gateway`),l(`orders-operate-5d8b-a`,`operate`),l(`orders-tasklist-6b4c-a`,`tasklist`),l(`orders-admin-8f2d-a`,`admin`),l(`orders-connectors-4a1e-a`,`connectors`),l(`orders-optimize-webapp-a`,`optimize-webapp`),l(`orders-optimize-importer-a`,`optimize-importer`)],v={kind:`CamundaOptimize`,metadata:{name:`orders-optimize`,namespace:`team-a`,uid:`uid-orders-optimize`},spec:{clusterRef:{name:`orders`}},status:{conditions:[h(`Ready`)]}},y=[{partition:1,leader:`orders-zeebe-0`,followers:[`orders-zeebe-1`,`orders-zeebe-2`],health:`unknown`},{partition:2,leader:`orders-zeebe-1`,followers:[`orders-zeebe-0`,`orders-zeebe-2`],health:`unknown`},{partition:3,leader:`orders-zeebe-2`,followers:[`orders-zeebe-0`,`orders-zeebe-1`],health:`unknown`}],b={title:`cluster/ComponentCards`,component:c,args:{onOpen:m()}},x=o({spec:g,pods:_,conditions:[`ZeebeReady`,`GatewayReady`,`OperateReady`,`TasklistReady`,`AdminReady`,`ConnectorsReady`].map(h),optimize:v,usage:_.map((e,t)=>u(e.metadata?.name??``,.1*(t+1),.8*1024**3)),partitions:y.map(e=>({...e,followers:[...e.followers]}))}),S={name:`All used and ready`,args:{cards:x,hrefOf:e=>`?tab=overview&component=${e}`}},C=(e,t)=>({kind:e,name:t,onOrphan:`Kept`}),w={name:`Section cards`,args:{cards:[...x,...a([f(`cloud-identity`,`Cloud identity`,1,`aws-iam-role`,{request:d(`orders-cloud-identity`,{summary:[{label:`Role`,value:`orders-camunda`}],resources:[C(`IAMRole`,`orders-camunda`)]}),condition:h(`Ready`)}),f(`backup-storage`,`Backup storage`,2,`aws-s3`,{request:d(`orders-backup-storage`,{summary:[{label:`Bucket`,value:`orders-backups-eu-west-1`}],resources:[C(`S3Bucket`,`orders-backups-eu-west-1`),C(`Grant`,`orders-backup-storage-read`)]}),condition:{status:`False`,reason:`GrantRejected`,message:`Grant orders-backup-storage-read has Ready=False`}}),f(`encryption-key`,`Encryption key`,3,`aws-kms-key`,{request:d(`orders-encryption-key`,{summary:[{label:`Key`,value:`Pending deletion`,severity:`Error`}],resources:[C(`KMSKey`,`orders-encryption-key`)]}),condition:{status:`False`,reason:`KeyPendingDeletion`,message:`The key is scheduled for deletion`}}),f(`access`,`Access`,4,`gateway-access`)])],hrefOf:e=>r(e)}},T={name:`Long summary label`,args:{cards:[...x,...a([f(`cloud-identity`,`Cloud identity`,1,`oidc-identity`,{request:d(`orders-cloud-identity`,{summary:[{label:`Issuer`,value:`https://login.example.com/realms/camunda`}],resources:[]}),condition:h(`Ready`)}),f(`secondary-storage`,`Secondary storage`,5,`existing-secondary-storage`,{request:d(`orders-secondary-storage`,{summary:[{label:`SecondaryStorageConfig`,value:`orders-storage, Ready`}],resources:[]}),condition:h(`Ready`)})])],hrefOf:e=>r(e)},play:async({canvasElement:e})=>{for(let t of e.querySelectorAll(`article dt, article dd`))await p(t.scrollWidth,`"${t.textContent}" is wider than its column`).toBeLessThanOrEqual(t.clientWidth)}},E={name:`Degraded brokers`,args:{cards:o({spec:{},pods:[l(`orders-zeebe-0`,`zeebe`),l(`orders-zeebe-1`,`zeebe`),l(`orders-zeebe-2`,`zeebe`,!1),l(`orders-gateway-7c9f-a`,`gateway`)],conditions:[{type:`ZeebeReady`,status:`False`,reason:`Degraded`,message:`2 of 3 pods are ready`},h(`GatewayReady`)],usage:[u(`orders-gateway-7c9f-a`,.35,1.1*1024**3)],partitions:[{partition:1,leader:`orders-zeebe-0`,followers:[`orders-zeebe-1`],health:`unknown`},{partition:2,leader:`orders-zeebe-1`,followers:[`orders-zeebe-0`],health:`unknown`},{partition:3,followers:[`orders-zeebe-0`],health:`unknown`}]})}},D={name:`Embedded apps, Connectors and Optimize not used`,args:{cards:o({spec:{},pods:[l(`orders-zeebe-0`,`zeebe`),l(`orders-gateway-7c9f-a`,`gateway`)],conditions:[h(`ZeebeReady`),h(`GatewayReady`),{type:`OperateReady`,status:`True`,reason:`Disabled`}],usage:[u(`orders-gateway-7c9f-a`,.2,.5*1024**3)],partitions:[{partition:1,leader:`orders-zeebe-0`,followers:[],health:`unknown`}]})}},O={name:`Usage not available`,args:{cards:o({spec:g,pods:_,conditions:[h(`ZeebeReady`),h(`GatewayReady`)],optimize:v,usage:`unavailable`,partitions:`unavailable`})}},k=[`AllUsedAndReady`,`SectionCards`,`LongSummaryLabel`,`DegradedBrokers`,`EmbeddedAppsConnectorsAndOptimizeNotUsed`,`UsageNotAvailable`],S.parameters={...S.parameters,docs:{...S.parameters?.docs,source:{originalSource:`{
  name: "All used and ready",
  args: {
    cards: allUsedAndReady,
    hrefOf: id => \`?tab=overview&component=\${id}\`
  }
}`,...S.parameters?.docs?.source}}},w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
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
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
  name: "Long summary label",
  args: {
    cards: [...allUsedAndReady, ...sectionCards([section("cloud-identity", "Cloud identity", 1, "oidc-identity", {
      request: request("orders-cloud-identity", {
        summary: [{
          label: "Issuer",
          value: "https://login.example.com/realms/camunda"
        }],
        resources: []
      }),
      condition: healthy("Ready")
    }), section("secondary-storage", "Secondary storage", 5, "existing-secondary-storage", {
      request: request("orders-secondary-storage", {
        summary: [{
          label: "SecondaryStorageConfig",
          value: "orders-storage, Ready"
        }],
        resources: []
      }),
      condition: healthy("Ready")
    })])],
    hrefOf: id => overviewHref(id as CardId)
  },
  play: async ({
    canvasElement
  }) => {
    for (const cell of canvasElement.querySelectorAll("article dt, article dd")) {
      await expect(cell.scrollWidth, \`"\${cell.textContent}" is wider than its column\`).toBeLessThanOrEqual(cell.clientWidth);
    }
  }
}`,...T.parameters?.docs?.source}}},E.parameters={...E.parameters,docs:{...E.parameters?.docs,source:{originalSource:`{
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
}`,...E.parameters?.docs?.source}}},D.parameters={...D.parameters,docs:{...D.parameters?.docs,source:{originalSource:`{
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
}`,...D.parameters?.docs?.source}}},O.parameters={...O.parameters,docs:{...O.parameters?.docs,source:{originalSource:`{
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
}`,...O.parameters?.docs?.source}}}})))()}A();export{S as AllUsedAndReady,E as DegradedBrokers,D as EmbeddedAppsConnectorsAndOptimizeNotUsed,T as LongSummaryLabel,w as SectionCards,O as UsageNotAvailable,k as __namedExportsOrder,b as default};