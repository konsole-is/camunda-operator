import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{r as t,t as n}from"./cluster-header-ClH6NPCM.js";function r(e,t){if(e){let t=e.replace(/\/+$/,``);return i.map(([e,n])=>({label:e,href:`${t}/${n}`}))}return i.flatMap(([e,n])=>{let r=t?.find(e=>e.name===n);return r?[{label:e,href:r.url}]:[]})}var i;function a(){return(a=e((()=>{i=[[`Operate`,`operate`],[`Tasklist`,`tasklist`],[`Admin`,`admin`]]})))()}var o,s,c,l;function u(){return(u=e((()=>{t(),a(),o={title:`cluster/ClusterHeader`,component:n},s={name:`Ready with launch links`,args:{name:`orders`,namespace:`team-a`,ready:{status:`True`,reason:`Healthy`},environment:`Production`,version:`8.9.23`,preset:`medium`,storage:`elasticsearch`,links:r(`https://orders.example.com`,void 0)}},c={name:`Not ready without external URL`,args:{name:`payments`,namespace:`team-b`,ready:{status:`False`,reason:`ZeebeNotReady`,message:`1 of 3 Zeebe brokers are ready`},storage:`rdbms`,links:[]}},l=[`ReadyWithLaunchLinks`,`NotReadyWithoutExternalUrl`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Ready with launch links",
  args: {
    name: "orders",
    namespace: "team-a",
    ready: {
      status: "True",
      reason: "Healthy"
    },
    environment: "Production",
    version: "8.9.23",
    preset: "medium",
    storage: "elasticsearch",
    links: launchLinks("https://orders.example.com", undefined)
  }
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Not ready without external URL",
  args: {
    name: "payments",
    namespace: "team-b",
    ready: {
      status: "False",
      reason: "ZeebeNotReady",
      message: "1 of 3 Zeebe brokers are ready"
    },
    storage: "rdbms",
    links: []
  }
}`,...c.parameters?.docs?.source}}}})))()}u();export{c as NotReadyWithoutExternalUrl,s as ReadyWithLaunchLinks,l as __namedExportsOrder,o as default};