import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,r}from"./button-lmY3Hw21.js";import{n as i,t as a}from"./status-chip-DAXEX2Rs.js";function o({name:e,namespace:t,ready:r,environment:i,version:o,preset:l,storage:u,links:d,actions:f}){let p=[[`Environment`,i??`No environment`],[`Namespace`,t],[`Version`,o??`-`],[`Preset`,l??`-`],[`Secondary storage`,u?c[u]:`-`]];return(0,s.jsxs)(`header`,{className:`flex flex-col gap-4 border-b border-divider pb-5`,children:[(0,s.jsxs)(`div`,{className:`flex flex-wrap items-start justify-between gap-3`,children:[(0,s.jsxs)(`div`,{className:`flex min-w-0 flex-wrap items-center gap-3`,children:[(0,s.jsx)(`h1`,{className:`min-w-0 font-heading text-3xl font-bold tracking-[-0.02em] break-all`,children:e}),(0,s.jsx)(a,{condition:r,readyReason:!0})]}),(0,s.jsxs)(`div`,{className:`flex flex-wrap items-center gap-2`,children:[d.map(e=>(0,s.jsx)(`a`,{href:e.href,target:`_blank`,rel:`noreferrer`,className:n({variant:`outline`,size:`sm`}),children:e.label},e.label)),f]})]}),(0,s.jsx)(`dl`,{className:`grid grid-cols-2 gap-x-8 gap-y-3 text-sm sm:flex sm:flex-wrap`,children:p.map(([e,t])=>(0,s.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-0.5`,children:[(0,s.jsx)(`dt`,{className:`text-xs text-muted`,children:e}),(0,s.jsx)(`dd`,{className:`truncate text-text-2`,children:t})]},e))})]})}var s,c;function l(){return(l=e((()=>{s=t(),i(),r(),c={elasticsearch:`Elasticsearch`,rdbms:`Relational database`},o.__docgenInfo={description:`ClusterHeader names the cluster, shows its Ready state and facts, and links to its web apps.`,methods:[],displayName:`ClusterHeader`,props:{name:{required:!0,tsType:{name:`string`},description:``},namespace:{required:!0,tsType:{name:`string`},description:``},ready:{required:!1,tsType:{name:`ConditionLike`},description:``},environment:{required:!1,tsType:{name:`string`},description:``},version:{required:!1,tsType:{name:`string`},description:``},preset:{required:!1,tsType:{name:`string`},description:``},storage:{required:!1,tsType:{name:`union`,raw:`"elasticsearch" | "rdbms"`,elements:[{name:`literal`,value:`"elasticsearch"`},{name:`literal`,value:`"rdbms"`}]},description:``},links:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  label: "Operate" | "Tasklist" | "Admin";
  href: string;
}`,signature:{properties:[{key:`label`,value:{name:`union`,raw:`"Operate" | "Tasklist" | "Admin"`,elements:[{name:`literal`,value:`"Operate"`},{name:`literal`,value:`"Tasklist"`},{name:`literal`,value:`"Admin"`}],required:!0}},{key:`href`,value:{name:`string`,required:!0}}]}}],raw:`LaunchLink[]`},description:``},actions:{required:!1,tsType:{name:`ReactNode`},description:``}}}})))()}function u(e,t){if(e){let t=e.replace(/\/+$/,``);return d.map(([e,n])=>({label:e,href:`${t}/${n}`}))}return d.flatMap(([e,n])=>{let r=t?.find(e=>e.name===n);return r?[{label:e,href:r.url}]:[]})}var d;function f(){return(f=e((()=>{d=[[`Operate`,`operate`],[`Tasklist`,`tasklist`],[`Admin`,`admin`]]})))()}var p,m,h,g;function _(){return(_=e((()=>{l(),f(),p={title:`cluster/ClusterHeader`,component:o},m={name:`Ready with launch links`,args:{name:`orders`,namespace:`team-a`,ready:{status:`True`,reason:`Healthy`},environment:`Production`,version:`8.9.23`,preset:`medium`,storage:`elasticsearch`,links:u(`https://orders.example.com`,void 0)}},h={name:`Not ready without external URL`,args:{name:`payments`,namespace:`team-b`,ready:{status:`False`,reason:`ZeebeNotReady`,message:`1 of 3 Zeebe brokers are ready`},storage:`rdbms`,links:[]}},g=[`ReadyWithLaunchLinks`,`NotReadyWithoutExternalUrl`],m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
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
}`,...m.parameters?.docs?.source}}},h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
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
}`,...h.parameters?.docs?.source}}}})))()}_();export{h as NotReadyWithoutExternalUrl,m as ReadyWithLaunchLinks,g as __namedExportsOrder,p as default};