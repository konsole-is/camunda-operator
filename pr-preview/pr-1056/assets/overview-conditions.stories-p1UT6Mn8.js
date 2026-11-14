import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{i as n,n as r,r as i,t as a}from"./camundaoptimize-BTOwQKfp.js";import{n as o,t as s}from"./camundaclusterclaim-DV7HS06u.js";import{r as c,t as l}from"./placement-BPmEgwCN.js";import{n as u,t as d}from"./conditions-table-LsPXyfdj.js";function f({cluster:e,optimize:t,claim:n,now:r}){let o=[{label:`Optimize`,conditions:l(a,t,`resource`)},{label:`Claim`,conditions:(n??[]).filter(e=>[`claim`,`resource`].includes(c(s,e.type)))}].filter(e=>e.conditions.length>0);return(0,p.jsxs)(`section`,{"aria-label":`Conditions`,className:`flex flex-col gap-3 rounded-lg border border-border bg-panel p-4`,children:[(0,p.jsx)(`h2`,{className:`font-heading text-base font-semibold`,children:`Conditions`}),(0,p.jsx)(d,{conditions:l(i,e,`resource`),groups:o,now:r,label:`Conditions outside the components`})]})}var p;function m(){return(m=e((()=>{p=t(),u(),n(),r(),o(),f.__docgenInfo={description:`OverviewConditions lists the conditions that belong to no component: those
of the cluster and of its Optimize that the maps place at resource, and
the claim group, which also holds the claim conditions at resource. A group
shows only when it has rows. now is the time that the ages count to.`,methods:[],displayName:`OverviewConditions`,props:{cluster:{required:!0,tsType:{name:`union`,raw:`ConditionRow[] | undefined`,elements:[{name:`Array`,elements:[{name:`ConditionRow`}],raw:`ConditionRow[]`},{name:`undefined`}]},description:``},optimize:{required:!1,tsType:{name:`Array`,elements:[{name:`ConditionRow`}],raw:`ConditionRow[]`},description:``},claim:{required:!1,tsType:{name:`Array`,elements:[{name:`ConditionRow`}],raw:`ConditionRow[]`},description:``},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}var h,g,_,v,y;function b(){return(b=e((()=>{m(),h=`2026-10-18T12:00:00Z`,g=(e,t=h)=>({type:e,status:`True`,reason:`Healthy`,lastTransitionTime:t}),_={title:`cluster/OverviewConditions`,component:f},v={name:`With Optimize and claim`,args:{cluster:[g(`Ready`),{type:`AdminSecretReady`,status:`False`,reason:`MissingSecret`,message:`Secret team-a/orders-admin does not exist`,lastTransitionTime:`2026-10-18T11:40:00Z`},g(`MirroredSecretsReady`,`2026-10-17T09:00:00Z`)],optimize:[g(`MirroredSecretsReady`)],claim:[{type:`RequestsReady`,status:`False`,reason:`Blocked`,message:`The access request waits for its plugin`,lastTransitionTime:`2026-10-18T11:55:00Z`},g(`ClusterReady`),g(`AttachmentsReady`)],now:h}},y=[`WithOptimizeAndClaim`],v.parameters={...v.parameters,docs:{...v.parameters?.docs,source:{originalSource:`{
  name: "With Optimize and claim",
  args: {
    cluster: [healthy("Ready"), {
      type: "AdminSecretReady",
      status: "False",
      reason: "MissingSecret",
      message: "Secret team-a/orders-admin does not exist",
      lastTransitionTime: "2026-10-18T11:40:00Z"
    }, healthy("MirroredSecretsReady", "2026-10-17T09:00:00Z")],
    optimize: [healthy("MirroredSecretsReady")],
    claim: [{
      type: "RequestsReady",
      status: "False",
      reason: "Blocked",
      message: "The access request waits for its plugin",
      lastTransitionTime: "2026-10-18T11:55:00Z"
    }, healthy("ClusterReady"), healthy("AttachmentsReady")],
    now
  }
}`,...v.parameters?.docs?.source}}}})))()}b();export{v as WithOptimizeAndClaim,y as __namedExportsOrder,_ as default};