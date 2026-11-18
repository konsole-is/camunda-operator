import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{t as r}from"./hooks-DwXdO6lu.js";import{x as i}from"./labels-Be1Q9KU6.js";import{n as a,t as o}from"./link-DOLMGIR2.js";import{n as s,t as c}from"./precheck-line-CAHPxO7l.js";import{n as l,t as u}from"./progress-rows-BEU3Rsi0.js";function d(e,t,n,r){let i=e?.status;return[...(e?.spec.plugins??[]).map(({section:a,registration:o})=>{let s=i?.requests?.find(e=>e.section===a&&e.registration===o),c=n.find(e=>e.metadata.name===o)?.spec.label||o,l=r.find(t=>t.metadata.namespace===e?.metadata.namespace&&t.metadata.name===s?.name)?.status?.summary?.[0];return{id:`section:${a}/${o}`,label:t.find(e=>e.metadata.name===a)?.spec.label||a,detail:s?`${c}, ${s.name}`:c,...l&&{summary:`${l.label}: ${l.value}`},checks:[{label:`Outputs`,condition:f(s?.outputsReady)},{label:`Ready`,condition:f(s?.ready)}]}}),...p.map(({id:e,label:t,type:n})=>{let r=i?.conditions?.find(e=>e.type===n);return{id:e,label:t,...r?.message&&{summary:r.message},checks:[{label:`Ready`,condition:r&&{status:r.status,reason:r.reason,message:r.message}}]}})]}function f(e){return e===`True`||e===`False`||e===`Unknown`?{status:e}:void 0}var p;function m(){return(m=e((()=>{p=[{id:`requests`,label:`Requests`,type:`RequestsReady`},{id:`cluster`,label:`Cluster`,type:`ClusterReady`},{id:`attachments`,label:`Attachments`,type:`AttachmentsReady`}]})))()}function h({name:e,namespace:t,claim:n,sections:r,registrations:i,requests:o}){let s=(0,_.useId)(),l=n?.status?.conditions??[],f=l.find(e=>e.type===v),p=l.find(e=>e.type===y)?.status===`True`,m=`/fleet/clusters/${encodeURIComponent(t)}/${encodeURIComponent(e)}`;return(0,g.jsxs)(`section`,{"aria-labelledby":s,className:`flex max-w-3xl min-w-0 flex-col gap-4`,children:[(0,g.jsx)(`h2`,{id:s,className:`font-heading text-base font-semibold`,children:`Creating the cluster ${e}`}),(0,g.jsx)(u,{ready:f,rows:d(n,r,i,o),done:p&&(0,g.jsx)(a,{href:m,className:`relative self-start text-sm text-info underline-offset-4 hit-area hover:underline`,children:`Open the cluster page`})}),n&&(0,g.jsx)(c,{claim:n,href:n.status?.cluster?`${m}?tab=claim`:void 0})]})}var g,_,v,y;function b(){return(b=e((()=>{g=n(),o(),_=t(),s(),l(),m(),r(),i(),v=`Ready`,y=`ClusterReady`,h.__docgenInfo={description:`ClusterProgressView shows the progress of the cluster claim named name in
namespace: its Ready condition, clusterProgressRows, and the precheck
refusal of the claim controller. The refusal links the Claim tab only
once status.cluster names the cluster, because the cluster page needs
the cluster. Once ClusterReady is True, a link opens the cluster page.`,methods:[],displayName:`ClusterProgressView`,props:{name:{required:!0,tsType:{name:`string`},description:``},namespace:{required:!0,tsType:{name:`string`},description:``},claim:{required:!1,tsType:{name:`CamundaClusterClaim`},description:``},sections:{required:!0,tsType:{name:`unknown`},description:``},registrations:{required:!0,tsType:{name:`unknown`},description:``},requests:{required:!0,tsType:{name:`unknown`},description:``}}}})))()}var x,S,C,w,T,E,D,O,k,A,j,M,N,P,F,I,L;function R(){return(R=e((()=>{b(),x=[{metadata:{name:`access`},spec:{label:`Access`,selection:`One`}},{metadata:{name:`secondary-storage`},spec:{label:`Secondary storage`,selection:`One`}},{metadata:{name:`encryption-key`},spec:{label:`Encryption key`,selection:`One`}}],S=[{metadata:{name:`gateway`},spec:{label:`Gateway`}},{metadata:{name:`elasticsearch`},spec:{label:`New Elasticsearch cluster`}},{metadata:{name:`kms`},spec:{label:`KMS key`}}],C=[{section:`access`,registration:`gateway`},{section:`secondary-storage`,registration:`elasticsearch`},{section:`encryption-key`,registration:`kms`}],w=(e,t,n,r)=>({type:e,status:t,reason:n,message:r,observedGeneration:1,lastTransitionTime:`2026-11-16T00:00:00Z`}),T=(e,t,n,r)=>({section:e,registration:t,name:`orders-${e}`,outputsReady:n,ready:r}),E=e=>({apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{namespace:`team-a`,name:`orders`,generation:1},spec:{presetRef:`small`,plugins:C},...e&&{status:e}}),D=[{metadata:{namespace:`team-a`,name:`orders-access`},status:{summary:[{label:`Host`,value:`orders.team-a.example.com`}]}},{metadata:{namespace:`team-a`,name:`orders-secondary-storage`},status:{summary:[{label:`Nodes`,value:`3 of 3 ready`}]}}],O={title:`create/ClusterProgressView`,component:h,args:{name:`orders`,namespace:`team-a`,sections:x,registrations:S,requests:[]}},k={name:`Every row pending`,args:{claim:E()}},A=`waiting for the outputs of encryption-key`,j={name:`Waiting for outputs`,args:{claim:E({conditions:[w(`Ready`,`False`,`PrerequisiteNotMet`,A),w(`RequestsReady`,`False`,`Blocked`,A)],requests:[T(`access`,`gateway`,`True`,`True`),T(`secondary-storage`,`elasticsearch`,`True`,`True`),T(`encryption-key`,`kms`,`False`,`False`)]}),requests:D}},M={name:`Refused by the precheck`,args:{claim:E({conditions:[w(`Ready`,`False`,`NoEnvironment`,`the namespace team-a has no fleet.camunda.io/environment label`)]})}},N={name:`Refused by the precheck after the cluster exists`,args:{claim:E({cluster:{name:`orders`},conditions:[w(`Ready`,`False`,`WiringCycle`,`the inputs of the sections access and encryption-key wait for each other`)]})}},P=`StorageAlreadyAttached: CamundaCluster "fleet-smoke/camundacluster-sample" already holds the backend`,F={name:`Cluster held at zero`,args:{claim:E({cluster:{name:`orders`},conditions:[w(`Ready`,`False`,`ClusterNotReady`,P),w(`RequestsReady`,`True`,`Ready`,`All requests are ready`),w(`ClusterReady`,`False`,`StorageAlreadyAttached`,P),w(`AttachmentsReady`,`True`,`Ready`,`No attachments`)],requests:[T(`access`,`gateway`,`True`,`True`),T(`secondary-storage`,`elasticsearch`,`True`,`True`),T(`encryption-key`,`kms`,`True`,`True`)]}),requests:D}},I={name:`Ready with the cluster link`,args:{claim:E({cluster:{name:`orders`},conditions:[w(`Ready`,`True`,`Ready`,`The cluster is ready`),w(`RequestsReady`,`True`,`Ready`,`All requests are ready`),w(`ClusterReady`,`True`,`Ready`,`Ready: The cluster is ready`),w(`AttachmentsReady`,`True`,`Ready`,`No attachments`)],requests:[T(`access`,`gateway`,`True`,`True`),T(`secondary-storage`,`elasticsearch`,`True`,`True`),T(`encryption-key`,`kms`,`True`,`True`)]}),requests:D}},L=[`EveryRowPending`,`WaitingForOutputs`,`RefusedByThePrecheck`,`RefusedAfterTheClusterExists`,`ClusterHeldAtZero`,`ReadyWithTheClusterLink`],k.parameters={...k.parameters,docs:{...k.parameters?.docs,source:{originalSource:`{
  name: "Every row pending",
  args: {
    claim: claim()
  }
}`,...k.parameters?.docs?.source}}},j.parameters={...j.parameters,docs:{...j.parameters?.docs,source:{originalSource:`{
  name: "Waiting for outputs",
  args: {
    claim: claim({
      conditions: [condition("Ready", "False", "PrerequisiteNotMet", waiting), condition("RequestsReady", "False", "Blocked", waiting)],
      requests: [entry("access", "gateway", "True", "True"), entry("secondary-storage", "elasticsearch", "True", "True"), entry("encryption-key", "kms", "False", "False")]
    }),
    requests
  }
}`,...j.parameters?.docs?.source}}},M.parameters={...M.parameters,docs:{...M.parameters?.docs,source:{originalSource:`{
  name: "Refused by the precheck",
  args: {
    claim: claim({
      conditions: [condition("Ready", "False", "NoEnvironment", "the namespace team-a has no fleet.camunda.io/environment label")]
    })
  }
}`,...M.parameters?.docs?.source}}},N.parameters={...N.parameters,docs:{...N.parameters?.docs,source:{originalSource:`{
  name: "Refused by the precheck after the cluster exists",
  args: {
    claim: claim({
      cluster: {
        name: "orders"
      },
      conditions: [condition("Ready", "False", "WiringCycle", "the inputs of the sections access and encryption-key wait for each other")]
    })
  }
}`,...N.parameters?.docs?.source}}},F.parameters={...F.parameters,docs:{...F.parameters?.docs,source:{originalSource:`{
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
}`,...F.parameters?.docs?.source}}},I.parameters={...I.parameters,docs:{...I.parameters?.docs,source:{originalSource:`{
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
}`,...I.parameters?.docs?.source}}}})))()}R();export{F as ClusterHeldAtZero,k as EveryRowPending,I as ReadyWithTheClusterLink,N as RefusedAfterTheClusterExists,M as RefusedByThePrecheck,j as WaitingForOutputs,L as __namedExportsOrder,O as default};