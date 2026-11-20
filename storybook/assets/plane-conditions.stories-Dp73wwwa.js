import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./conditions-table--NnsD3Y8.js";import{n as i,t as a}from"./camundamanagementcluster-B7akbDjz.js";import{r as o,t as s}from"./placement-BPmEgwCN.js";var c;function l(){return(l=e((()=>{c={Ready:`header`,RequestsReady:`claim`,ManagementClusterReady:`claim`}})))()}function u({plane:e,claim:t,now:n}){let i=[{label:`Claim`,conditions:(t??[]).filter(e=>[`claim`,`resource`].includes(o(c,e.type)))}].filter(e=>e.conditions.length>0);return(0,d.jsxs)(`section`,{"aria-label":`Conditions`,className:`flex flex-col gap-3 rounded-lg border border-border bg-panel p-4`,children:[(0,d.jsx)(`h2`,{className:`font-heading text-base font-semibold`,children:`Conditions`}),(0,d.jsx)(r,{conditions:s(a,e,`resource`),groups:i,now:n,label:`Conditions outside the components`})]})}var d;function f(){return(f=e((()=>{d=t(),n(),i(),l(),u.__docgenInfo={description:`PlaneConditions is the Conditions section of a plane: the conditions of
the CamundaManagementCluster that the map places at resource, then the
group "Claim" with the claim conditions at claim or resource. A group
shows only when it has rows. now is the time that the ages count to.`,methods:[],displayName:`PlaneConditions`,props:{plane:{required:!1,tsType:{name:`Array`,elements:[{name:`ConditionRow`}],raw:`ConditionRow[]`},description:``},claim:{required:!1,tsType:{name:`Array`,elements:[{name:`ConditionRow`}],raw:`ConditionRow[]`},description:``},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}var p,m,h,g,_,v,y,b,x;function S(){return(S=e((()=>{f(),p=`2026-11-15T12:00:00Z`,m=(e,t=p)=>({type:e,status:`True`,reason:`Healthy`,lastTransitionTime:t}),h=(e,t)=>({type:e,status:`True`,reason:`Disabled`,message:t,lastTransitionTime:p}),g=[m(`Ready`),m(`RequestsReady`),m(`ManagementClusterReady`)],_={title:`planes/PlaneConditions`,component:u},v={name:`OIDC ready`,args:{plane:[m(`Ready`),h(`SecretsReady`,`Component is disabled.`),m(`MirroredSecretsReady`,`2026-11-14T09:00:00Z`),m(`ServiceAccountsReady`),m(`ManagementAuthReady`),h(`OptimizeCallbacksReady`,`The identity provider of the platform config holds the callback URLs of Optimize`),m(`IdentityReady`)],claim:g,now:p}},y={name:`Failing ManagementAuthReady`,args:{plane:[m(`SecretsReady`),m(`MirroredSecretsReady`),m(`ServiceAccountsReady`),{type:`ManagementAuthReady`,status:`False`,reason:`InvalidReference`,message:`CamundaPlatformConfig "oidc-platform" sets no spec.auth.oidc.jwksUrl`,lastTransitionTime:`2026-11-15T11:50:00Z`},m(`OptimizeCallbacksReady`)],claim:[m(`RequestsReady`),{type:`ManagementClusterReady`,status:`False`,reason:`Creating`,message:`InvalidReference: the platform config sets no JWKS URL`,lastTransitionTime:`2026-11-15T11:50:00Z`}],now:p}},b={name:`Claim only`,args:{claim:[{type:`RequestsReady`,status:`False`,reason:`Pending`,message:`The management-database request waits for its plugin`,lastTransitionTime:p},{type:`ManagementClusterReady`,status:`Unknown`,reason:`Creating`,message:`The claim controller has not created the management plane yet`,lastTransitionTime:p}],now:p}},x=[`OidcReady`,`FailingManagementAuth`,`ClaimOnly`],v.parameters={...v.parameters,docs:{...v.parameters?.docs,source:{originalSource:`{
  name: "OIDC ready",
  args: {
    plane: [healthy("Ready"), disabled("SecretsReady", "Component is disabled."), healthy("MirroredSecretsReady", "2026-11-14T09:00:00Z"), healthy("ServiceAccountsReady"), healthy("ManagementAuthReady"), disabled("OptimizeCallbacksReady", "The identity provider of the platform config holds the callback URLs of Optimize"), healthy("IdentityReady")],
    claim,
    now
  }
}`,...v.parameters?.docs?.source}}},y.parameters={...y.parameters,docs:{...y.parameters?.docs,source:{originalSource:`{
  name: "Failing ManagementAuthReady",
  args: {
    plane: [healthy("SecretsReady"), healthy("MirroredSecretsReady"), healthy("ServiceAccountsReady"), {
      type: "ManagementAuthReady",
      status: "False",
      reason: "InvalidReference",
      message: 'CamundaPlatformConfig "oidc-platform" sets no spec.auth.oidc.jwksUrl',
      lastTransitionTime: "2026-11-15T11:50:00Z"
    }, healthy("OptimizeCallbacksReady")],
    claim: [healthy("RequestsReady"), {
      type: "ManagementClusterReady",
      status: "False",
      reason: "Creating",
      message: "InvalidReference: the platform config sets no JWKS URL",
      lastTransitionTime: "2026-11-15T11:50:00Z"
    }],
    now
  }
}`,...y.parameters?.docs?.source}}},b.parameters={...b.parameters,docs:{...b.parameters?.docs,source:{originalSource:`{
  name: "Claim only",
  args: {
    claim: [{
      type: "RequestsReady",
      status: "False",
      reason: "Pending",
      message: "The management-database request waits for its plugin",
      lastTransitionTime: now
    }, {
      type: "ManagementClusterReady",
      status: "Unknown",
      reason: "Creating",
      message: "The claim controller has not created the management plane yet",
      lastTransitionTime: now
    }],
    now
  }
}`,...b.parameters?.docs?.source}}}})))()}S();export{b as ClaimOnly,y as FailingManagementAuth,v as OidcReady,x as __namedExportsOrder,_ as default};