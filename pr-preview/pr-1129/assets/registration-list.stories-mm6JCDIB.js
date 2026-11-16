import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{t as r}from"./hooks-BZtkPEQF.js";import{n as i,t as a}from"./data-table-BJFRKeLh.js";import{D as o,E as s,c,h as l,m as u,s as d}from"./policy-D5M8iHpu.js";import{a as f,n as p}from"./claim-tab-CHluR9sU.js";import{n as m,t as h}from"./age-f6c5OgJq.js";import{i as g,n as ee,r as te,t as _}from"./catalog-panel-C_zrvK4m.js";import{i as v,n as ne,r as y,t as b}from"./fixtures-BSCp3F5s.js";import{n as x,t as S}from"./condition-badge-CJ-M-6OW.js";function re(e){let{slot:t,appliesTo:n}=e.spec;return t?s.filter(e=>e.slot===t&&n.includes(e.claimKind)):[]}function C(e,t){let{spec:n}=e,r=E(t),i=r.includes(`PluginRegistration`),a=i?void 0:D(e,`Accepted`),o=i?void 0:D(e,`Ready`);return{name:e.metadata.name??``,label:n.label,...n.controller&&{controller:n.controller},mode:n.mode,...i&&{healthUnknown:i},...a&&{accepted:a},...o&&{ready:o},...o?.status===`True`&&{healthySince:String(o.lastTransitionTime)},...!r.includes(`PluginRequest`)&&{requests:ie(e,t,r)},...n.operatorVersions&&{operatorVersions:n.operatorVersions},orphan:n.deletionPolicies.includes(`Orphan`)}}function w(e){let t=E(e),n=[...e.sections].sort((e,t)=>(e.spec.order??0)-(t.spec.order??0)||k(e.metadata.name??``,t.metadata.name??``)),r=new Set(n.map(e=>e.metadata.name)),i=t=>t.map(t=>C(t,e)).sort((e,t)=>k(e.label,t.label)||k(e.name,t.name)),a=n.map(n=>({section:{name:n.metadata.name??``,label:n.spec.label,selection:n.spec.selection,...n.spec.emptyOptionLabel&&{emptyOptionLabel:n.spec.emptyOptionLabel},fields:re(n)},...t.includes(`PluginRegistration`)&&{unread:!0},rows:i(e.registrations.filter(e=>e.spec.sections.includes(n.metadata.name??``)))})),o=e.registrations.filter(e=>!e.spec.sections.some(e=>r.has(e)));return o.length===0?a:[...a,{rows:i(o),...t.includes(`PluginSection`)&&{unread:!0}}]}function T(e){let t=e.filter(e=>d(e,`Accepted`)&&d(e,`Ready`)).length;return{healthy:t,notHealthy:e.length-t}}function E(e){let t=[...(e.errors??[]).map(e=>e.kind),...e.stale??[]];return[...new Set(t)]}function ie(e,t,n){let r=t.requests.filter(t=>t.spec.registration===e.metadata.name);if(n.includes(`Namespace`)||n.includes(`Environment`))return{total:r.length};let i=new Map;for(let e of r){let n=O(e.metadata.namespace??``,t),r=n?.metadata.name??``,a=n?n.spec.displayName||r:A;i.set(r,{label:a,count:(i.get(r)?.count??0)+1})}let a=[...i],o=e=>a.filter(([,t])=>t.label===e).length>1;return{total:r.length,byEnvironment:a.map(([e,{label:t,count:n}])=>({environment:e&&o(t)?`${t} (${e})`:t,count:n})).sort((e,t)=>k(e.environment,t.environment))}}function D(e,t){return e.status?.conditions?.find(e=>e.type===t)}function O(e,t){let n=t.namespaces.find(t=>t.metadata?.name===e);if(n)return u(n,t.environments).environment}function k(e,t){return e===t?0:e<t?-1:1}var A;function j(){return(j=e((()=>{o(),l(),c(),A=`No environment`})))()}function M(){return(M=e((()=>{})))()}function N(){return(N=e((()=>{r(),M()})))()}function P({groups:e,summary:t,charts:n,catalogError:r,sections:i,errors:a,stale:o=[],now:s}){let c=E({errors:a,stale:o}),l=c.includes(`PluginRegistration`);return(0,I.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,I.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,I.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em]`,children:`Plugins`}),(0,I.jsx)(`p`,{className:`text-sm text-muted`,children:l?`Health not known`:`${t.healthy} healthy, ${t.notHealthy} not healthy`})]}),a.map(e=>(0,I.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind)),c.length>0&&(0,I.jsx)(`p`,{className:`text-sm text-muted`,children:`Not read: ${c.join(`, `)}. What needs them shows as not known.`}),e.map(e=>(0,I.jsx)(ae,{group:e,now:s},e.section?.name??``)),(0,I.jsx)(_,{charts:n,sections:i,error:r,installedUnknown:l})]})}function ae({group:e,now:t}){let n=e.section?.label??(e.unread?`Section not known`:`Not in a section`);return(0,I.jsxs)(p,{label:n,children:[e.section&&(0,I.jsx)(oe,{section:e.section}),e.rows.length===0?(0,I.jsx)(`p`,{className:`text-sm text-muted`,children:e.unread?`Options not known`:`No option is installed for this section.`}):(0,I.jsx)(a,{label:`Options of ${n}`,toolbar:!1,columns:R,rows:e.rows.map(e=>({id:e.name,label:e.label,cells:F(e,t)}))})]})}function oe({section:e}){return(0,I.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[e.fields.length===0?(0,I.jsx)(`p`,{children:`Fills no field`}):(0,I.jsx)(`ul`,{children:e.fields.map(e=>(0,I.jsx)(`li`,{children:se(e)},`${e.claimKind} ${e.kind} ${e.path}`))}),(0,I.jsx)(`p`,{className:`text-muted`,children:e.selection===`One`?`Pick one`:`Pick many`}),(0,I.jsx)(`p`,{className:`text-muted`,children:e.emptyOptionLabel?`Empty option: ${e.emptyOptionLabel}`:`No empty option`})]})}function se(e){let t=e.claimKind===`CamundaManagementClusterClaim`?` for a management plane`:``;return`Fills ${e.kind} ${e.path}${t}`}function F(e,t){return{option:(0,I.jsxs)(I.Fragment,{children:[(0,I.jsx)(`span`,{className:`font-medium`,children:e.label}),(0,I.jsx)(`span`,{className:`${L} font-mono`,children:e.name})]}),controller:e.controller?[e.controller.name,e.controller.version].filter(Boolean).join(` `):`None`,mode:e.mode,accepted:(0,I.jsx)(S,{type:`Accepted`,condition:e.accepted,unknown:e.healthUnknown}),ready:(0,I.jsxs)(I.Fragment,{children:[(0,I.jsx)(S,{type:`Ready`,condition:e.ready,unknown:e.healthUnknown}),e.healthySince&&(0,I.jsx)(`time`,{dateTime:e.healthySince,className:`${L} mt-1`,children:`Healthy since ${h(e.healthySince,t)} ago`})]}),requests:e.requests?(0,I.jsxs)(I.Fragment,{children:[(0,I.jsx)(`span`,{children:e.requests.total}),e.requests.byEnvironment?e.requests.byEnvironment.map(e=>(0,I.jsx)(`span`,{className:L,children:`${e.environment} ${e.count}`},e.environment)):e.requests.total>0&&(0,I.jsx)(`span`,{className:L,children:`Environments not known`})]}):`Not known`,operatorVersions:e.operatorVersions??`Any`,orphan:e.orphan?`Supported`:`Not supported`}}var I,L,R;function z(){return(z=e((()=>{I=n(),t(),f(),m(),i(),ee(),x(),j(),N(),L=`block text-xs text-muted`,R=[{key:`option`,label:`Option`},{key:`controller`,label:`Controller`},{key:`mode`,label:`Mode`},{key:`accepted`,label:`Accepted`},{key:`ready`,label:`Ready`},{key:`requests`,label:`Requests`},{key:`operatorVersions`,label:`Operator versions`},{key:`orphan`,label:`Orphan`}],P.__docgenInfo={description:`RegistrationList is the Plugins page: the registrations in the groups of
their sections, the summary line, and the panel of the charts. While
PluginRegistration is read, charts are the charts that are not installed.
While it is not read, charts must not be empty when the catalog has a
chart. catalogError is the error of the read of the catalog. Each of
errors shows above the groups, and the groups stay. stale holds the kinds
whose watch has no snapshot on its current connection, as
useRegistrationSources gives them. While PluginRegistration is not read,
the page shows no health count and no chart as not installed. now is the
time that "Healthy since" counts to.`,methods:[],displayName:`RegistrationList`,props:{groups:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  section?: SectionHeader;
  rows: RegistrationRow[];
  unread?: boolean;
}`,signature:{properties:[{key:`section`,value:{name:`signature`,type:`object`,raw:`{
  name: string;
  label: string;
  selection: "One" | "Many";
  emptyOptionLabel?: string;
  fields: readonly SlotField[];
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`selection`,value:{name:`union`,raw:`"One" | "Many"`,elements:[{name:`literal`,value:`"One"`},{name:`literal`,value:`"Many"`}],required:!0}},{key:`emptyOptionLabel`,value:{name:`string`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}}]},required:!1}},{key:`rows`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  label: string;
  controller?: { name: string; version?: string };
  mode: "Default" | "Optional";
  accepted?: ConditionRow;
  ready?: ConditionRow;
  /**
   * healthUnknown is true while PluginRegistration is not read. accepted,
   * ready, and healthySince are then unset.
   */
  healthUnknown?: boolean;
  /** healthySince is lastTransitionTime of Ready while Ready is True. */
  healthySince?: string;
  /**
   * requests is unset while PluginRequest is not read, and byEnvironment
   * while Namespace or Environment is not read.
   */
  requests?: {
    total: number;
    byEnvironment?: { environment: string; count: number }[];
  };
  operatorVersions?: string;
  orphan: boolean;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`controller`,value:{name:`signature`,type:`object`,raw:`{ name: string; version?: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`version`,value:{name:`string`,required:!1}}]},required:!1}},{key:`mode`,value:{name:`union`,raw:`"Default" | "Optional"`,elements:[{name:`literal`,value:`"Default"`},{name:`literal`,value:`"Optional"`}],required:!0}},{key:`accepted`,value:{name:`ConditionRow`,required:!1}},{key:`ready`,value:{name:`ConditionRow`,required:!1}},{key:`healthUnknown`,value:{name:`boolean`,required:!1},description:`healthUnknown is true while PluginRegistration is not read. accepted,
ready, and healthySince are then unset.`},{key:`healthySince`,value:{name:`string`,required:!1},description:`healthySince is lastTransitionTime of Ready while Ready is True.`},{key:`requests`,value:{name:`signature`,type:`object`,raw:`{
  total: number;
  byEnvironment?: { environment: string; count: number }[];
}`,signature:{properties:[{key:`total`,value:{name:`number`,required:!0}},{key:`byEnvironment`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ environment: string; count: number }`,signature:{properties:[{key:`environment`,value:{name:`string`,required:!0}},{key:`count`,value:{name:`number`,required:!0}}]}}],raw:`{ environment: string; count: number }[]`,required:!1}}]},required:!1},description:`requests is unset while PluginRequest is not read, and byEnvironment
while Namespace or Environment is not read.`},{key:`operatorVersions`,value:{name:`string`,required:!1}},{key:`orphan`,value:{name:`boolean`,required:!0}}]}}],raw:`RegistrationRow[]`,required:!0}},{key:`unread`,value:{name:`boolean`,required:!1}}]}}],raw:`RegistrationGroup[]`},description:``},summary:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{ healthy: number; notHealthy: number }`,signature:{properties:[{key:`healthy`,value:{name:`number`,required:!0}},{key:`notHealthy`,value:{name:`number`,required:!0}}]}},description:``},charts:{required:!0,tsType:{name:`unknown`},description:``},catalogError:{required:!1,tsType:{name:`string`},description:``},sections:{required:!0,tsType:{name:`unknown`},description:``},errors:{required:!0,tsType:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`},description:``},stale:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}var B,V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{te(),ne(),j(),z(),B=e=>({groups:w(e),summary:T(e.registrations),sections:e.sections}),V=(...e)=>{let t=v();return B({...t,registrations:[...t.registrations,...e]})},H={title:`setup/plugins/RegistrationList`,component:P,args:{...B(v()),charts:[],errors:[],now:`2026-11-13T09:00:00Z`}},U={name:`Built-in registrations`},W={name:`A registration that is not accepted and one that is not ready`,args:V(y(`rustfs`,{label:`RustFS bucket`,sections:[`backup-storage`],mode:`Default`,controller:{name:`rustfs`,version:`0.1.0`},operatorVersions:`>=0.3.0 <1.0.0`},[b(`Accepted`,`False`,`DefaultConflict`,`In section backup-storage, these registrations also have mode Default: s3-bucket`),b(`Ready`,`True`,`Healthy`)]),y(`aws-kms`,{label:`AWS KMS key`,sections:[`encryption-key`],controller:{name:`aws-kms`,version:`0.1.0`},deletionPolicies:[`Delete`]},[b(`Accepted`,`True`,`Healthy`),b(`Ready`,`False`,`CredentialsInvalid`,`The role cannot be assumed`)]))},G={name:`A registration in two sections`,args:V(y(`aws-identity`,{label:`AWS IAM role`,sections:[`cloud-identity`,`connectors-permissions`],controller:{name:`aws-identity`,version:`0.1.0`}},[b(`Accepted`,`True`,`Healthy`),b(`Ready`,`True`,`Healthy`)]))},K={name:`A registration whose section is gone`,args:V(y(`vault-transit`,{label:`Vault transit key`,sections:[`secret-store`],controller:{name:`vault-transit`,version:`0.2.0`}},[b(`Accepted`,`False`,`SectionMissing`,`These sections do not exist: secret-store`)]))},q={name:`With the catalog panel`,args:{charts:g.charts}},J={name:`The registrations cannot be read`,args:{...B({...v(),registrations:[],errors:[{kind:`PluginRegistration`,message:`forbidden`}]}),errors:[{kind:`PluginRegistration`,message:`forbidden`}],charts:g.charts}},Y={name:`The registrations watch failed after the page loaded`,args:{...B({...v(),errors:[{kind:`PluginRegistration`,message:`watch closed`}]}),errors:[{kind:`PluginRegistration`,message:`watch closed`}],charts:g.charts}},X={name:`Sections, requests, and environments not read yet`,args:{...B({...v(),sections:[],stale:[`PluginSection`,`PluginRequest`,`Environment`]}),sections:[],stale:[`PluginSection`,`PluginRequest`,`Environment`]}},Z={name:`The catalog cannot be read`,args:{catalogError:`reading catalog /app/catalog/official-plugins.json: ENOENT: no such file or directory`}},Q=[`BuiltInRegistrations`,`NotAcceptedAndNotReady`,`InTwoSections`,`SectionGone`,`WithTheCatalogPanel`,`RegistrationsNotRead`,`RegistrationsWatchFailed`,`ListsNotAnswered`,`CatalogNotRead`],U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
  name: "Built-in registrations"
}`,...U.parameters?.docs?.source}}},W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "A registration that is not accepted and one that is not ready",
  args: withRegistrations(pluginRegistration("rustfs", {
    label: "RustFS bucket",
    sections: ["backup-storage"],
    mode: "Default",
    controller: {
      name: "rustfs",
      version: "0.1.0"
    },
    operatorVersions: ">=0.3.0 <1.0.0"
  }, [condition("Accepted", "False", "DefaultConflict", "In section backup-storage, these registrations also have mode Default: s3-bucket"), condition("Ready", "True", "Healthy")]), pluginRegistration("aws-kms", {
    label: "AWS KMS key",
    sections: ["encryption-key"],
    controller: {
      name: "aws-kms",
      version: "0.1.0"
    },
    deletionPolicies: ["Delete"]
  }, [condition("Accepted", "True", "Healthy"), condition("Ready", "False", "CredentialsInvalid", "The role cannot be assumed")]))
}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "A registration in two sections",
  args: withRegistrations(pluginRegistration("aws-identity", {
    label: "AWS IAM role",
    sections: ["cloud-identity", "connectors-permissions"],
    controller: {
      name: "aws-identity",
      version: "0.1.0"
    }
  }, [condition("Accepted", "True", "Healthy"), condition("Ready", "True", "Healthy")]))
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "A registration whose section is gone",
  args: withRegistrations(pluginRegistration("vault-transit", {
    label: "Vault transit key",
    sections: ["secret-store"],
    controller: {
      name: "vault-transit",
      version: "0.2.0"
    }
  }, [condition("Accepted", "False", "SectionMissing", "These sections do not exist: secret-store")]))
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "With the catalog panel",
  args: {
    charts: catalog.charts as CatalogChart[]
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "The registrations cannot be read",
  args: {
    ...listOf({
      ...registrationSources(),
      registrations: [],
      errors: [{
        kind: "PluginRegistration",
        message: "forbidden"
      }]
    }),
    errors: [{
      kind: "PluginRegistration",
      message: "forbidden"
    }],
    charts: catalog.charts as CatalogChart[]
  }
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "The registrations watch failed after the page loaded",
  args: {
    ...listOf({
      ...registrationSources(),
      errors: [{
        kind: "PluginRegistration",
        message: "watch closed"
      }]
    }),
    errors: [{
      kind: "PluginRegistration",
      message: "watch closed"
    }],
    charts: catalog.charts as CatalogChart[]
  }
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Sections, requests, and environments not read yet",
  args: {
    ...listOf({
      ...registrationSources(),
      sections: [],
      stale: ["PluginSection", "PluginRequest", "Environment"]
    }),
    sections: [],
    stale: ["PluginSection", "PluginRequest", "Environment"]
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "The catalog cannot be read",
  args: {
    catalogError: "reading catalog /app/catalog/official-plugins.json: ENOENT: no such file or directory"
  }
}`,...Z.parameters?.docs?.source}}}})))()}$();export{U as BuiltInRegistrations,Z as CatalogNotRead,G as InTwoSections,X as ListsNotAnswered,W as NotAcceptedAndNotReady,J as RegistrationsNotRead,Y as RegistrationsWatchFailed,K as SectionGone,q as WithTheCatalogPanel,Q as __namedExportsOrder,H as default};