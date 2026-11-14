import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,f as r}from"./labels-lxF_8UjP.js";import{c as i,t as a}from"./merge-B877RDyT.js";import{g as o,t as s}from"./fields-Ds_W30BQ.js";import{o as c,r as l}from"./fixtures-Dju11n-g.js";import{n as u,r as d}from"./settings-D_JwDfaT.js";import{a as f,i as p,n as m,s as h}from"./settings-editor-BHagemgI.js";function g({host:e,process:t,cluster:r,live:a,optimize:s}){let c={...r,...a},l=i({cluster:y},void 0,c),d=o({cluster:{kind:`CamundaCluster`,metadata:{name:`orders`,namespace:`team-a`,labels:{[n]:`orders`}},spec:c},merged:l,claim:{kind:`CamundaClusterClaim`,metadata:{name:`orders`,namespace:`team-a`},spec:{presetRef:`medium`,cluster:r,...s&&{attachments:{optimize:s}}}},sections:[],registrations:[],optimize:s&&{kind:`CamundaOptimize`,metadata:{name:`orders`,namespace:`team-a`},spec:{clusterRef:{name:`orders`},managementAuthRef:s.managementAuthConfigRef,importer:s.importer}}}),f=u(d,e,l.spec),p=f.find(e=>e.component===t)??f[0];return{group:p,settings:h({group:p,spec:t===`optimize`?s:l.spec})}}var _,v,y,b,x,S,C,w,T;function E(){return(E=e((()=>{_=t(),s(),r(),a(),f(),l(),p(),d(),{fn:v}=__STORYBOOK_MODULE_TEST__,y={zeebe:{replicas:3,partitions:3,storageSize:`32Gi`,resources:{requests:{cpu:`1`,memory:`4Gi`},limits:{memory:`4Gi`}}},gateway:{replicas:2}},b={title:`cluster/SettingsEditor`,component:m,args:{secrets:c,set:v(),reset:v()},render:e=>(0,_.jsx)(m,{...e},JSON.stringify([e.group,e.settings]))},x={name:`Brokers from the preset`,args:g({host:`zeebe`,process:`zeebe`,cluster:{zeebe:{replicas:5},extraEnv:[{name:`TZ`,value:`UTC`}]}})},S={name:`Embedded Tasklist`,args:g({host:`gateway`,process:`tasklist`,cluster:{gateway:{mode:`Standalone`},tasklist:{replicas:1}}})},C={name:`Locked fields`,args:g({host:`connectors`,process:`connectors`,cluster:{},live:{connectors:{version:`8.9.23`,serviceAccount:{name:`orders-connectors`}}}})},w={name:`Optimize`,args:g({host:`optimize`,process:`optimize`,cluster:{},optimize:{managementAuthConfigRef:`keycloak`,importer:{replicas:1}}})},T=[`BrokersFromThePreset`,`EmbeddedTasklist`,`LockedFields`,`Optimize`],x.parameters={...x.parameters,docs:{...x.parameters?.docs,source:{originalSource:`{
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
}`,...w.parameters?.docs?.source}}}})))()}E();export{x as BrokersFromThePreset,S as EmbeddedTasklist,C as LockedFields,w as Optimize,T as __namedExportsOrder,b as default};