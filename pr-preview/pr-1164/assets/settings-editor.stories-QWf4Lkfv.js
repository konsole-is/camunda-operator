import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,x as r}from"./labels-Be1Q9KU6.js";import{E as i,t as a}from"./merge-BODiO-dm.js";import{d as o}from"./schema-CkYx6TI9.js";import{t as s}from"./fields-CnMyiCBC.js";import{c,i as l,n as u,o as d}from"./settings-editor-DUnvjvIZ.js";import{o as f,r as p}from"./fixtures-Dr1JTW37.js";import{n as m,r as h}from"./settings-B5hZCPCm.js";function g({host:e,process:t,cluster:r,live:a,optimize:s}){let l={...r,...a},u=i({cluster:S},void 0,l),d=o({cluster:{kind:`CamundaCluster`,metadata:{name:`orders`,namespace:`team-a`,labels:{[n]:`orders`}},spec:l},merged:u,claim:{kind:`CamundaClusterClaim`,metadata:{name:`orders`,namespace:`team-a`},spec:{presetRef:`medium`,cluster:r,...s&&{attachments:{optimize:s}}}},sections:[],registrations:[],optimize:s&&{kind:`CamundaOptimize`,metadata:{name:`orders`,namespace:`team-a`},spec:{clusterRef:{name:`orders`},managementAuthRef:s.managementAuthConfigRef,importer:s.importer}}}),f=m(d,e,u.spec),p=f.find(e=>e.component===t)??f[0];return{group:p,settings:c({group:p,spec:t===`optimize`?s:u.spec})}}var _,v,y,b,x,S,C,w,T,E,D,O,k;function A(){return(A=e((()=>{_=t(),s(),r(),a(),d(),p(),l(),h(),{expect:v,fn:y,userEvent:b,within:x}=__STORYBOOK_MODULE_TEST__,S={zeebe:{replicas:3,partitions:3,storageSize:`32Gi`,resources:{requests:{cpu:`1`,memory:`4Gi`},limits:{memory:`4Gi`}}},gateway:{replicas:2}},C={title:`cluster/SettingsEditor`,component:u,args:{secrets:f,set:y(),reset:y()},render:e=>(0,_.jsx)(u,{...e},JSON.stringify([e.group,e.settings]))},w={name:`Brokers from the preset`,args:g({host:`zeebe`,process:`zeebe`,cluster:{zeebe:{replicas:5},extraEnv:[{name:`TZ`,value:`UTC`}]}})},T={name:`Embedded Tasklist`,args:g({host:`gateway`,process:`tasklist`,cluster:{gateway:{mode:`Standalone`},tasklist:{replicas:1}}})},E={name:`Locked fields`,args:g({host:`connectors`,process:`connectors`,cluster:{},live:{connectors:{version:`8.9.23`,serviceAccount:{name:`orders-connectors`}}}})},D={name:`Optimize`,args:g({host:`optimize`,process:`optimize`,cluster:{},optimize:{managementAuthConfigRef:`keycloak`,importer:{replicas:1}}})},O={name:`Collapsed`,args:{...w.args,collapsed:!0},play:async({canvasElement:e})=>{let t=x(x(e).getByRole(`heading`,{level:3,name:`Brokers`})).getByRole(`button`);await v(t).toHaveAttribute(`aria-expanded`,`false`),await v(t).toHaveAccessibleName(`Brokers 1 changed in the claim`),await b.click(t),await v(x(e).getByText(/^\d+ settings: .*1 changed in the claim$/)).toBeVisible()}},k=[`BrokersFromThePreset`,`EmbeddedTasklist`,`LockedFields`,`Optimize`,`Collapsed`],w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
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
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
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
}`,...T.parameters?.docs?.source}}},E.parameters={...E.parameters,docs:{...E.parameters?.docs,source:{originalSource:`{
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
}`,...E.parameters?.docs?.source}}},D.parameters={...D.parameters,docs:{...D.parameters?.docs,source:{originalSource:`{
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
}`,...D.parameters?.docs?.source}}},O.parameters={...O.parameters,docs:{...O.parameters?.docs,source:{originalSource:`{
  name: "Collapsed",
  args: {
    ...BrokersFromThePreset.args,
    collapsed: true
  },
  play: async ({
    canvasElement
  }) => {
    const row = within(within(canvasElement).getByRole("heading", {
      level: 3,
      name: "Brokers"
    })).getByRole("button");
    await expect(row).toHaveAttribute("aria-expanded", "false");
    await expect(row).toHaveAccessibleName("Brokers 1 changed in the claim");
    await userEvent.click(row);
    await expect(within(canvasElement).getByText(/^\\d+ settings: .*1 changed in the claim$/)).toBeVisible();
  }
}`,...O.parameters?.docs?.source}}}})))()}A();export{w as BrokersFromThePreset,O as Collapsed,T as EmbeddedTasklist,E as LockedFields,D as Optimize,k as __namedExportsOrder,C as default};