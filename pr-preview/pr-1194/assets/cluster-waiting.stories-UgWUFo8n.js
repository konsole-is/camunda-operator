import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./not-read-RuqQXk-B.js";function i({waits:e,deleting:t=!1,unread:n}){let i=t?`The cluster no longer exists`:`The cluster does not exist yet`;return(0,a.jsxs)(`section`,{"aria-label":i,className:`flex flex-col gap-3 rounded-lg border border-border bg-panel p-4`,children:[(0,a.jsx)(`h2`,{className:`font-heading text-base font-semibold`,children:i}),t?(0,a.jsx)(`p`,{className:`text-sm text-text-2`,children:`The claim is being deleted.`}):n.length>0?(0,a.jsx)(r,{kinds:n}):e&&e.length>0&&(0,a.jsxs)(a.Fragment,{children:[(0,a.jsx)(`p`,{className:`text-sm text-text-2`,children:`The claim waits for the outputs of these requests:`}),(0,a.jsx)(`ul`,{"aria-label":`Requests the claim waits for`,className:`flex flex-col gap-1 text-sm`,children:e.map((e,t)=>(0,a.jsxs)(`li`,{className:`text-text`,children:[e.label,`:`,` `,e.request?(0,a.jsx)(`span`,{className:`font-mono text-text-2`,children:e.request}):(0,a.jsx)(`span`,{className:`text-muted`,children:`Not made yet`})]},`${e.section}/${t}`))})]})]})}var a;function o(){return(o=e((()=>{a=t(),n(),i.__docgenInfo={description:`ClusterWaiting is the box of a cluster page whose claim has no cluster.
It says that the cluster does not exist yet and names each of waits, or,
while unread is not empty, the kinds that it could not read. With
deleting, it says only that the claim is being deleted.`,methods:[],displayName:`ClusterWaiting`,props:{waits:{required:!1,tsType:{name:`unknown`},description:``},deleting:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`false`,computed:!1}},unread:{required:!0,tsType:{name:`unknown`},description:``}}}})))()}var s,c,l,u,d,f;function p(){return(p=e((()=>{o(),s={title:`cluster/ClusterWaiting`,component:i,args:{unread:[]}},c={name:`Waiting for two requests`,args:{waits:[{section:`secondary-storage`,label:`Secondary storage`,request:`orders-secondary-storage`},{section:`access`,label:`Access`}]}},l={name:`Waiting for nothing`,args:{waits:[]}},u={name:`Requests not known`,args:{waits:[],unread:[`PluginRequest`]}},d={name:`Claim being deleted`,args:{waits:[],deleting:!0}},f=[`WaitingForTwoRequests`,`WaitingForNothing`,`RequestsNotKnown`,`ClaimBeingDeleted`],c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Waiting for two requests",
  args: {
    waits: [{
      section: "secondary-storage",
      label: "Secondary storage",
      request: "orders-secondary-storage"
    }, {
      section: "access",
      label: "Access"
    }]
  }
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Waiting for nothing",
  args: {
    waits: []
  }
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Requests not known",
  args: {
    waits: [],
    unread: ["PluginRequest"]
  }
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Claim being deleted",
  args: {
    waits: [],
    deleting: true
  }
}`,...d.parameters?.docs?.source}}}})))()}p();export{d as ClaimBeingDeleted,u as RequestsNotKnown,l as WaitingForNothing,c as WaitingForTwoRequests,f as __namedExportsOrder,s as default};