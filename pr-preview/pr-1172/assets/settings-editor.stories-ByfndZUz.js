import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,x as r}from"./labels-Be1Q9KU6.js";import{E as i,t as a}from"./merge-BODiO-dm.js";import{d as o}from"./schema-kVnlqPaB.js";import{t as s}from"./fields-DBtGs9ZT.js";import{c,i as l,n as u,o as d}from"./settings-editor-CIMbAIE7.js";import{o as f,r as p}from"./fixtures-Dr1JTW37.js";import{n as m,r as h}from"./settings-B5hZCPCm.js";function g({host:e,process:t,cluster:r,live:a,optimize:s}){let l={...r,...a},u=i({cluster:y},void 0,l),d=o({cluster:{kind:`CamundaCluster`,metadata:{name:`orders`,namespace:`team-a`,labels:{[n]:`orders`}},spec:l},merged:u,claim:{kind:`CamundaClusterClaim`,metadata:{name:`orders`,namespace:`team-a`},spec:{presetRef:`medium`,cluster:r,...s&&{attachments:{optimize:s}}}},sections:[],registrations:[],optimize:s&&{kind:`CamundaOptimize`,metadata:{name:`orders`,namespace:`team-a`},spec:{clusterRef:{name:`orders`},managementAuthRef:s.managementAuthConfigRef,importer:s.importer}}}),f=m(d,e,u.spec),p=f.find(e=>e.component===t)??f[0];return{group:p,settings:c({group:p,spec:t===`optimize`?s:u.spec})}}var _,v,y,b,x,S,C,w,T,E;function D(){return(D=e((()=>{_=t(),s(),r(),a(),d(),p(),l(),h(),{fn:v}=__STORYBOOK_MODULE_TEST__,y={zeebe:{replicas:3,partitions:3,storageSize:`32Gi`,resources:{requests:{cpu:`1`,memory:`4Gi`},limits:{memory:`4Gi`}}},gateway:{replicas:2}},b={title:`cluster/SettingsEditor`,component:u,args:{secrets:f,set:v(),reset:v()},render:e=>(0,_.jsx)(u,{...e},JSON.stringify([e.group,e.settings]))},x={name:`Brokers from the preset`,args:g({host:`zeebe`,process:`zeebe`,cluster:{zeebe:{replicas:5},extraEnv:[{name:`TZ`,value:`UTC`}]}})},S={name:`Embedded Tasklist`,args:g({host:`gateway`,process:`tasklist`,cluster:{gateway:{mode:`Standalone`},tasklist:{replicas:1}}})},C={name:`Locked fields`,args:g({host:`connectors`,process:`connectors`,cluster:{},live:{connectors:{version:`8.9.23`,serviceAccount:{name:`orders-connectors`}}}})},w={name:`Optimize`,args:g({host:`optimize`,process:`optimize`,cluster:{},optimize:{managementAuthConfigRef:`keycloak`,importer:{replicas:1}}})},T={name:`Collapsed`,args:{...x.args,collapsed:!0}},E=[`BrokersFromThePreset`,`EmbeddedTasklist`,`LockedFields`,`Optimize`,`Collapsed`],x.parameters={...x.parameters,docs:{...x.parameters?.docs,source:{originalSource:`{
  name: "Brokers from the preset",
  args: argsOf({
    host: "zeebe",
    process: "zeebe",
    cluster: {
      zeebe: {
        replicas: 5
      },
      extraEnv: [{
        name: "TZ",
        value: "UTC"
      }]
    }
  })
}`,...x.parameters?.docs?.source}}},S.parameters={...S.parameters,docs:{...S.parameters?.docs,source:{originalSource:`{
  name: "Embedded Tasklist",
  args: argsOf({
    host: "gateway",
    process: "tasklist",
    cluster: {
      gateway: {
        mode: "Standalone"
      },
      tasklist: {
        replicas: 1
      }
    }
  })
}`,...S.parameters?.docs?.source}}},C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
  name: "Locked fields",
  args: argsOf({
    host: "connectors",
    process: "connectors",
    cluster: {},
    live: {
      connectors: {
        version: "8.9.23",
        serviceAccount: {
          name: "orders-connectors"
        }
      }
    }
  })
}`,...C.parameters?.docs?.source}}},w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
  name: "Optimize",
  args: argsOf({
    host: "optimize",
    process: "optimize",
    cluster: {},
    optimize: {
      managementAuthConfigRef: "keycloak",
      importer: {
        replicas: 1
      }
    }
  })
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
  name: "Collapsed",
  args: {
    ...BrokersFromThePreset.args,
    collapsed: true
  }
}`,...T.parameters?.docs?.source}}}})))()}D();export{x as BrokersFromThePreset,T as Collapsed,S as EmbeddedTasklist,C as LockedFields,w as Optimize,E as __namedExportsOrder,b as default};