import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{o as t,x as n}from"./labels-Be1Q9KU6.js";import{a as r,i,s as a,t as o}from"./cards-DZWgh7c8.js";import{r as s,t as c}from"./component-card-C2pPAnhd.js";function l(e,n,r=!0){return{kind:`Pod`,metadata:{name:e,labels:{[t]:n,...n.startsWith(`optimize-`)&&{"camunda.io/optimize-uid":`uid-orders-optimize`}}},status:r?{phase:`Running`,conditions:[{type:`Ready`,status:`True`}]}:{phase:`Running`,containerStatuses:[{name:`zeebe`,image:`camunda/zeebe`,imageID:``,ready:!1,restartCount:4,state:{waiting:{reason:`CrashLoopBackOff`}}}]}}}function u(e,t,n){return{pod:e,cpu:{usedCores:t,limitCores:1},memory:{usedBytes:n,limitBytes:2*1024**3}}}function d(e,t){return{kind:`PluginRequest`,metadata:{name:e,namespace:`team-a`},status:t}}function f(e,t,n,r,i={}){return{id:`section:${e}`,section:e,label:t,order:n,selection:{section:e,registration:r},...i}}var p,m,h,g,_,v,y,b,x,S,C,w,T,E,D;function O(){return(O=e((()=>{n(),i(),s(),{fn:p}=__STORYBOOK_MODULE_TEST__,m=e=>({type:e,status:`True`,reason:`Healthy`}),h={operate:{mode:`Standalone`},tasklist:{mode:`Standalone`},admin:{mode:`Standalone`},connectors:{enabled:!0}},g=[l(`orders-zeebe-0`,`zeebe`),l(`orders-zeebe-1`,`zeebe`),l(`orders-zeebe-2`,`zeebe`),l(`orders-gateway-7c9f-a`,`gateway`),l(`orders-operate-5d8b-a`,`operate`),l(`orders-tasklist-6b4c-a`,`tasklist`),l(`orders-admin-8f2d-a`,`admin`),l(`orders-connectors-4a1e-a`,`connectors`),l(`orders-optimize-webapp-a`,`optimize-webapp`),l(`orders-optimize-importer-a`,`optimize-importer`)],_={kind:`CamundaOptimize`,metadata:{name:`orders-optimize`,namespace:`team-a`,uid:`uid-orders-optimize`},spec:{clusterRef:{name:`orders`}},status:{conditions:[m(`Ready`)]}},v=[{partition:1,leader:`orders-zeebe-0`,followers:[`orders-zeebe-1`,`orders-zeebe-2`],health:`unknown`},{partition:2,leader:`orders-zeebe-1`,followers:[`orders-zeebe-0`,`orders-zeebe-2`],health:`unknown`},{partition:3,leader:`orders-zeebe-2`,followers:[`orders-zeebe-0`,`orders-zeebe-1`],health:`unknown`}],y={title:`cluster/ComponentCards`,component:c,args:{onOpen:p()}},b=o({spec:h,pods:g,conditions:[`ZeebeReady`,`GatewayReady`,`OperateReady`,`TasklistReady`,`AdminReady`,`ConnectorsReady`].map(m),optimize:_,usage:g.map((e,t)=>u(e.metadata?.name??``,.1*(t+1),.8*1024**3)),partitions:v.map(e=>({...e,followers:[...e.followers]}))}),x={name:`All used and ready`,args:{cards:b,hrefOf:e=>`?tab=overview&component=${e}`}},S=(e,t)=>({kind:e,name:t,onOrphan:`Kept`}),C={name:`Section cards`,args:{cards:[...b,...a([f(`cloud-identity`,`Cloud identity`,1,`aws-iam-role`,{request:d(`orders-cloud-identity`,{summary:[{label:`Role`,value:`orders-camunda`}],resources:[S(`IAMRole`,`orders-camunda`)]}),condition:m(`Ready`)}),f(`backup-storage`,`Backup storage`,2,`aws-s3`,{request:d(`orders-backup-storage`,{summary:[{label:`Bucket`,value:`orders-backups-eu-west-1`}],resources:[S(`S3Bucket`,`orders-backups-eu-west-1`),S(`Grant`,`orders-backup-storage-read`)]}),condition:{status:`False`,reason:`GrantRejected`,message:`Grant orders-backup-storage-read has Ready=False`}}),f(`encryption-key`,`Encryption key`,3,`aws-kms-key`,{request:d(`orders-encryption-key`,{summary:[{label:`Key`,value:`Pending deletion`,severity:`Error`}],resources:[S(`KMSKey`,`orders-encryption-key`)]}),condition:{status:`False`,reason:`KeyPendingDeletion`,message:`The key is scheduled for deletion`}}),f(`access`,`Access`,4,`gateway-access`)])],hrefOf:e=>r(e)}},w={name:`Degraded brokers`,args:{cards:o({spec:{},pods:[l(`orders-zeebe-0`,`zeebe`),l(`orders-zeebe-1`,`zeebe`),l(`orders-zeebe-2`,`zeebe`,!1),l(`orders-gateway-7c9f-a`,`gateway`)],conditions:[{type:`ZeebeReady`,status:`False`,reason:`Degraded`,message:`2 of 3 pods are ready`},m(`GatewayReady`)],usage:[u(`orders-gateway-7c9f-a`,.35,1.1*1024**3)],partitions:[{partition:1,leader:`orders-zeebe-0`,followers:[`orders-zeebe-1`],health:`unknown`},{partition:2,leader:`orders-zeebe-1`,followers:[`orders-zeebe-0`],health:`unknown`},{partition:3,followers:[`orders-zeebe-0`],health:`unknown`}]})}},T={name:`Embedded apps, Connectors and Optimize not used`,args:{cards:o({spec:{},pods:[l(`orders-zeebe-0`,`zeebe`),l(`orders-gateway-7c9f-a`,`gateway`)],conditions:[m(`ZeebeReady`),m(`GatewayReady`),{type:`OperateReady`,status:`True`,reason:`Disabled`}],usage:[u(`orders-gateway-7c9f-a`,.2,.5*1024**3)],partitions:[{partition:1,leader:`orders-zeebe-0`,followers:[],health:`unknown`}]})}},E={name:`Usage not available`,args:{cards:o({spec:h,pods:g,conditions:[m(`ZeebeReady`),m(`GatewayReady`)],optimize:_,usage:`unavailable`,partitions:`unavailable`})}},D=[`AllUsedAndReady`,`SectionCards`,`DegradedBrokers`,`EmbeddedAppsConnectorsAndOptimizeNotUsed`,`UsageNotAvailable`],x.parameters={...x.parameters,docs:{...x.parameters?.docs,source:{originalSource:`{
  name: "All used and ready",
  args: {
    cards: allUsedAndReady,
    hrefOf: id => \`?tab=overview&component=\${id}\`
  }
}`,...x.parameters?.docs?.source}}},C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
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
}`,...C.parameters?.docs?.source}}},w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
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
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
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
}`,...T.parameters?.docs?.source}}},E.parameters={...E.parameters,docs:{...E.parameters?.docs,source:{originalSource:`{
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
}`,...E.parameters?.docs?.source}}}})))()}O();export{x as AllUsedAndReady,w as DegradedBrokers,T as EmbeddedAppsConnectorsAndOptimizeNotUsed,C as SectionCards,E as UsageNotAvailable,D as __namedExportsOrder,y as default};