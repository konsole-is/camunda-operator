import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{r as n,t as r}from"./button-DLE7EUqh.js";import{n as i,t as a}from"./status-chip-BMhu0Pt1.js";function o({name:e,namespace:t,ready:n,readyUnknown:r=!1,mode:i,platformConfig:o,serves:c,actions:l}){let u=[[`Identity provider`,i??`-`],[`Namespace`,t],[`Platform config`,o||`-`],[`Serves`,c?`environments`in c?c.environments.join(`, `):c.noClusterSelector?`No cluster, because it has no cluster selector`:c.selector:`-`]];return(0,s.jsxs)(`header`,{className:`flex flex-col gap-4 border-b border-divider pb-5`,children:[(0,s.jsxs)(`div`,{className:`flex flex-wrap items-start justify-between gap-3`,children:[(0,s.jsxs)(`div`,{className:`flex min-w-0 flex-wrap items-center gap-3`,children:[(0,s.jsx)(`h1`,{className:`min-w-0 font-heading text-3xl font-bold tracking-[-0.02em] break-all`,children:e}),(0,s.jsx)(a,{condition:n,readyReason:!0,unknown:r})]}),l&&(0,s.jsx)(`div`,{className:`flex flex-wrap items-center gap-2`,children:l})]}),(0,s.jsx)(`dl`,{className:`grid grid-cols-2 gap-x-8 gap-y-3 text-sm sm:flex sm:flex-wrap`,children:u.map(([e,t])=>(0,s.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-0.5`,children:[(0,s.jsx)(`dt`,{className:`text-xs text-muted`,children:e}),(0,s.jsx)(`dd`,{className:`truncate text-text-2`,children:t})]},e))})]})}var s;function c(){return(c=e((()=>{s=t(),i(),o.__docgenInfo={description:`PlaneHeader names the plane and shows its Ready state, its identity mode,
its namespace, its platform config, and what it serves. actions holds the
buttons of a managed plane. readyUnknown shows the Ready state as not
known, for a Ready condition that the page could not read.`,methods:[],displayName:`PlaneHeader`,props:{name:{required:!0,tsType:{name:`string`},description:``},namespace:{required:!0,tsType:{name:`string`},description:``},ready:{required:!1,tsType:{name:`ConditionLike`},description:``},readyUnknown:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`false`,computed:!1}},mode:{required:!1,tsType:{name:`union`,raw:`"Keycloak" | "External Keycloak" | "OIDC"`,elements:[{name:`literal`,value:`"Keycloak"`},{name:`literal`,value:`"External Keycloak"`},{name:`literal`,value:`"OIDC"`}]},description:``},platformConfig:{required:!1,tsType:{name:`string`},description:``},serves:{required:!1,tsType:{name:`union`,raw:`Plane["serves"]`},description:``},actions:{required:!1,tsType:{name:`ReactNode`},description:``}}}})))()}var l,u,d,f,p,m,h;function g(){return(g=e((()=>{l=t(),n(),c(),u={title:`planes/PlaneHeader`,component:o},d={name:`OIDC ready`,args:{name:`plane-a`,namespace:`platform`,ready:{status:`True`,reason:`Healthy`},mode:`OIDC`,platformConfig:`oidc-platform`,serves:{environments:[`platform`,`staging`]}}},f={name:`Keycloak not ready`,args:{name:`plane-b`,namespace:`platform`,ready:{status:`False`,reason:`KeycloakNotReady`,message:`0 of 1 Keycloak pods are ready`},mode:`Keycloak`,platformConfig:`default-platform`,serves:{environments:[`production`]},actions:(0,l.jsx)(r,{variant:`outline`,size:`sm`,children:`Suspend`})}},p={name:`Suspended`,args:{name:`plane-c`,namespace:`platform`,ready:{status:`True`,reason:`Suspended`},mode:`External Keycloak`,platformConfig:`default-platform`,serves:{environments:[`platform`]}}},m={name:`Unmanaged with a selector`,args:{name:`legacy-plane`,namespace:`camunda-system`,ready:{status:`True`,reason:`Healthy`},mode:`Keycloak`,serves:{selector:`team in (orders,payments), !legacy`}}},h=[`OidcReady`,`KeycloakNotReady`,`Suspended`,`UnmanagedWithSelector`],d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "OIDC ready",
  args: {
    name: "plane-a",
    namespace: "platform",
    ready: {
      status: "True",
      reason: "Healthy"
    },
    mode: "OIDC",
    platformConfig: "oidc-platform",
    serves: {
      environments: ["platform", "staging"]
    }
  }
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Keycloak not ready",
  args: {
    name: "plane-b",
    namespace: "platform",
    ready: {
      status: "False",
      reason: "KeycloakNotReady",
      message: "0 of 1 Keycloak pods are ready"
    },
    mode: "Keycloak",
    platformConfig: "default-platform",
    serves: {
      environments: ["production"]
    },
    actions: <Button variant="outline" size="sm">
        Suspend
      </Button>
  }
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Suspended",
  args: {
    name: "plane-c",
    namespace: "platform",
    ready: {
      status: "True",
      reason: "Suspended"
    },
    mode: "External Keycloak",
    platformConfig: "default-platform",
    serves: {
      environments: ["platform"]
    }
  }
}`,...p.parameters?.docs?.source}}},m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "Unmanaged with a selector",
  args: {
    name: "legacy-plane",
    namespace: "camunda-system",
    ready: {
      status: "True",
      reason: "Healthy"
    },
    mode: "Keycloak",
    serves: {
      selector: "team in (orders,payments), !legacy"
    }
  }
}`,...m.parameters?.docs?.source}}}})))()}g();export{f as KeycloakNotReady,d as OidcReady,p as Suspended,m as UnmanagedWithSelector,h as __namedExportsOrder,u as default};