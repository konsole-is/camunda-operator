import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{t as r}from"./hooks-DwXdO6lu.js";import{x as i}from"./labels-Be1Q9KU6.js";import{t as a}from"./policy-BJNT8nX8.js";import{n as o,t as s}from"./link-DOLMGIR2.js";import{n as c,t as l}from"./precheck-line-jRJb9apc.js";import{n as u,t as d}from"./progress-rows-Coe0ymu9.js";function f(e,t,n,r){let i=e?.status;return[...(e?.spec.plugins??[]).map(({section:a,registration:o})=>{let s=i?.requests?.find(e=>e.section===a&&e.registration===o),c=n.find(e=>e.metadata.name===o)?.spec.label||o,l=r.find(t=>t.metadata.namespace===e?.metadata.namespace&&t.metadata.name===s?.name)?.status?.summary?.[0];return{id:`section:${a}/${o}`,label:t.find(e=>e.metadata.name===a)?.spec.label||a,detail:s?`${c}, ${s.name}`:c,...l&&{summary:`${l.label}: ${l.value}`},checks:[{label:`Outputs`,condition:p(s?.outputsReady)},{label:`Ready`,condition:p(s?.ready)}]}}),...m.map(({id:e,label:t,type:n})=>{let r=i?.conditions?.find(e=>e.type===n);return{id:e,label:t,...r?.message&&{summary:r.message},checks:[{label:`Ready`,condition:r&&{status:r.status,reason:r.reason,message:r.message}}]}})]}function p(e){return e===`True`||e===`False`||e===`Unknown`?{status:e}:void 0}var m;function h(){return(h=e((()=>{a(),m=[{id:`requests`,label:`Requests`,type:`RequestsReady`},{id:`cluster`,label:`Cluster`,type:`ClusterReady`},{id:`attachments`,label:`Attachments`,type:`AttachmentsReady`}]})))()}function g({name:e,namespace:t,claim:n,written:r=!1,sections:i,registrations:a,requests:s}){let c=(0,v.useId)(),u=n?.status?.conditions??[],p=u.find(e=>e.type===y),m=u.find(e=>e.type===b)?.status===`True`,h=`/fleet/clusters/${encodeURIComponent(t)}/${encodeURIComponent(e)}`;return(0,_.jsxs)(`section`,{"aria-labelledby":c,className:`flex max-w-3xl min-w-0 flex-col gap-4`,children:[(0,_.jsx)(`h2`,{id:c,className:`font-heading text-base font-semibold`,children:`Creating the cluster ${e}`}),(0,_.jsx)(d,{ready:p,rows:f(n,i,a,s),done:n&&(0,_.jsxs)(`div`,{className:`flex flex-wrap gap-4`,children:[m&&(0,_.jsx)(o,{href:h,className:x,children:`Open the cluster page`}),!r&&(0,_.jsx)(o,{href:`${h}?tab=claim`,className:x,children:`Open the Claim tab`})]})}),n&&(0,_.jsx)(l,{claim:n})]})}var _,v,y,b,x;function S(){return(S=e((()=>{_=n(),s(),v=t(),c(),u(),h(),r(),i(),y=`Ready`,b=`ClusterReady`,x=`relative self-start text-sm text-info underline-offset-4 hit-area hover:underline`,g.__docgenInfo={description:`ClusterProgressView shows the progress of the cluster claim named name in
namespace: its Ready condition, clusterProgressRows, and the precheck
refusal of the claim controller. A link opens the Claim tab of the
cluster page, unless written says that claim is not live yet. Once
ClusterReady is True, a link opens the cluster page.`,methods:[],displayName:`ClusterProgressView`,props:{name:{required:!0,tsType:{name:`string`},description:``},namespace:{required:!0,tsType:{name:`string`},description:``},claim:{required:!1,tsType:{name:`CamundaClusterClaim`},description:``},written:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`false`,computed:!1}},sections:{required:!0,tsType:{name:`unknown`},description:``},registrations:{required:!0,tsType:{name:`unknown`},description:``},requests:{required:!0,tsType:{name:`unknown`},description:``}}}})))()}var C,w,T,E,D,O,k,A,j,M,N,P,F,I,L,R;function z(){return(z=e((()=>{S(),C=[{metadata:{name:`access`},spec:{label:`Access`,selection:`One`}},{metadata:{name:`secondary-storage`},spec:{label:`Secondary storage`,selection:`One`}},{metadata:{name:`encryption-key`},spec:{label:`Encryption key`,selection:`One`}}],w=[{metadata:{name:`gateway`},spec:{label:`Gateway`}},{metadata:{name:`elasticsearch`},spec:{label:`New Elasticsearch cluster`}},{metadata:{name:`kms`},spec:{label:`KMS key`}}],T=[{section:`access`,registration:`gateway`},{section:`secondary-storage`,registration:`elasticsearch`},{section:`encryption-key`,registration:`kms`}],E=(e,t,n,r)=>({type:e,status:t,reason:n,message:r,observedGeneration:1,lastTransitionTime:`2026-11-16T00:00:00Z`}),D=(e,t,n,r)=>({section:e,registration:t,name:`orders-${e}`,outputsReady:n,ready:r}),O=e=>({apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{namespace:`team-a`,name:`orders`,generation:1},spec:{presetRef:`small`,plugins:T},...e&&{status:e}}),k=[{metadata:{namespace:`team-a`,name:`orders-access`},status:{summary:[{label:`Host`,value:`orders.team-a.example.com`}]}},{metadata:{namespace:`team-a`,name:`orders-secondary-storage`},status:{summary:[{label:`Nodes`,value:`3 of 3 ready`}]}}],A={title:`create/ClusterProgressView`,component:g,args:{name:`orders`,namespace:`team-a`,sections:C,registrations:w,requests:[]}},j={name:`Every row pending`,args:{claim:O()}},M=`waiting for the outputs of encryption-key`,N={name:`Waiting for outputs`,args:{claim:O({conditions:[E(`Ready`,`False`,`PrerequisiteNotMet`,M),E(`RequestsReady`,`False`,`Blocked`,M)],requests:[D(`access`,`gateway`,`True`,`True`),D(`secondary-storage`,`elasticsearch`,`True`,`True`),D(`encryption-key`,`kms`,`False`,`False`)]}),requests:k}},P={name:`Refused by the precheck`,args:{claim:O({conditions:[E(`Ready`,`False`,`NoEnvironment`,`the namespace team-a has no fleet.camunda.io/environment label`)]})}},F=`StorageAlreadyAttached: CamundaCluster "fleet-smoke/camundacluster-sample" already holds the backend`,I={name:`Cluster held at zero`,args:{claim:O({cluster:{name:`orders`},conditions:[E(`Ready`,`False`,`ClusterNotReady`,F),E(`RequestsReady`,`True`,`Ready`,`All requests are ready`),E(`ClusterReady`,`False`,`StorageAlreadyAttached`,F),E(`AttachmentsReady`,`True`,`Ready`,`No attachments`)],requests:[D(`access`,`gateway`,`True`,`True`),D(`secondary-storage`,`elasticsearch`,`True`,`True`),D(`encryption-key`,`kms`,`True`,`True`)]}),requests:k}},L={name:`Ready with the cluster link`,args:{claim:O({cluster:{name:`orders`},conditions:[E(`Ready`,`True`,`Ready`,`The cluster is ready`),E(`RequestsReady`,`True`,`Ready`,`All requests are ready`),E(`ClusterReady`,`True`,`Ready`,`Ready: The cluster is ready`),E(`AttachmentsReady`,`True`,`Ready`,`No attachments`)],requests:[D(`access`,`gateway`,`True`,`True`),D(`secondary-storage`,`elasticsearch`,`True`,`True`),D(`encryption-key`,`kms`,`True`,`True`)]}),requests:k}},R=[`EveryRowPending`,`WaitingForOutputs`,`RefusedByThePrecheck`,`ClusterHeldAtZero`,`ReadyWithTheClusterLink`],j.parameters={...j.parameters,docs:{...j.parameters?.docs,source:{originalSource:`{
  name: "Every row pending",
  args: {
    claim: claim()
  }
}`,...j.parameters?.docs?.source}}},N.parameters={...N.parameters,docs:{...N.parameters?.docs,source:{originalSource:`{
  name: "Waiting for outputs",
  args: {
    claim: claim({
      conditions: [condition("Ready", "False", "PrerequisiteNotMet", waiting), condition("RequestsReady", "False", "Blocked", waiting)],
      requests: [entry("access", "gateway", "True", "True"), entry("secondary-storage", "elasticsearch", "True", "True"), entry("encryption-key", "kms", "False", "False")]
    }),
    requests
  }
}`,...N.parameters?.docs?.source}}},P.parameters={...P.parameters,docs:{...P.parameters?.docs,source:{originalSource:`{
  name: "Refused by the precheck",
  args: {
    claim: claim({
      conditions: [condition("Ready", "False", "NoEnvironment", "the namespace team-a has no fleet.camunda.io/environment label")]
    })
  }
}`,...P.parameters?.docs?.source}}},I.parameters={...I.parameters,docs:{...I.parameters?.docs,source:{originalSource:`{
  name: "Cluster held at zero",
  args: {
    claim: claim({
      cluster: {
        name: "orders"
      },
      conditions: [condition("Ready", "False", "ClusterNotReady", held), condition("RequestsReady", "True", "Ready", "All requests are ready"), condition("ClusterReady", "False", "StorageAlreadyAttached", held), condition("AttachmentsReady", "True", "Ready", "No attachments")],
      requests: [entry("access", "gateway", "True", "True"), entry("secondary-storage", "elasticsearch", "True", "True"), entry("encryption-key", "kms", "True", "True")]
    }),
    requests
  }
}`,...I.parameters?.docs?.source}}},L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
  name: "Ready with the cluster link",
  args: {
    claim: claim({
      cluster: {
        name: "orders"
      },
      conditions: [condition("Ready", "True", "Ready", "The cluster is ready"), condition("RequestsReady", "True", "Ready", "All requests are ready"), condition("ClusterReady", "True", "Ready", "Ready: The cluster is ready"), condition("AttachmentsReady", "True", "Ready", "No attachments")],
      requests: [entry("access", "gateway", "True", "True"), entry("secondary-storage", "elasticsearch", "True", "True"), entry("encryption-key", "kms", "True", "True")]
    }),
    requests
  }
}`,...L.parameters?.docs?.source}}}})))()}z();export{I as ClusterHeldAtZero,j as EveryRowPending,L as ReadyWithTheClusterLink,P as RefusedByThePrecheck,N as WaitingForOutputs,R as __namedExportsOrder,A as default};