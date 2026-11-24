import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./data-table-CFvZKU0M.js";import{n as i,t as a}from"./age-f6c5OgJq.js";import{n as o,t as s}from"./condition-badge-ClTbZwVe.js";import{n as c,o as l}from"./claim-tab-DjgrHrcR.js";import{o as u}from"./unread-Do34-4tt.js";import{t as d}from"./use-clock-DsKrgeKH.js";import{i as f,l as p,o as m,t as h}from"./fixtures-kgATjJaf.js";import{i as g,n as _,r as v,t as y}from"./catalog-panel-DXAYPRPd.js";import{a as b,i as x,n as S,r as C}from"./groups-CUbu843n.js";import{t as w}from"./sources-pgcXN-2p.js";function T({groups:e,summary:t,charts:n,catalogError:r,sections:i,errors:a,stale:o=[],now:s}){let c=u({errors:a,stale:o}),l=c.includes(`PluginRegistration`);return(0,k.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,k.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,k.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em]`,children:`Plugins`}),(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:l?`Health not known`:`${t.healthy} healthy, ${t.notHealthy} not healthy`})]}),a.map(e=>(0,k.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind)),c.length>0&&(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:`Not read: ${c.join(`, `)}. What needs them shows as not known.`}),e.map(e=>(0,k.jsx)(E,{group:e,now:s},e.section?.name??``)),(0,k.jsx)(y,{charts:n,sections:i,error:r,installedUnknown:l})]})}function E({group:e,now:t}){let n=e.section?.label??(e.unread?`Section not known`:`Not in a section`);return(0,k.jsxs)(c,{label:n,children:[e.section&&(0,k.jsx)(D,{section:e.section}),e.rows.length===0?(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:e.unread?`Options not known`:`No option is installed for this section.`}):(0,k.jsx)(r,{label:`Options of ${n}`,toolbar:!1,columns:j,rows:e.rows.map(e=>({id:e.name,label:e.label,href:`/setup/plugins/${encodeURIComponent(e.name)}`,cells:O(e,t)}))})]})}function D({section:e}){return(0,k.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[e.fields.length===0?(0,k.jsx)(`p`,{children:`Fills no field`}):(0,k.jsx)(`ul`,{children:e.fields.map(e=>(0,k.jsx)(`li`,{children:S(e)},`${e.claimKind} ${e.kind} ${e.path}`))}),(0,k.jsx)(`p`,{className:`text-muted`,children:e.selection===`One`?`Pick one`:`Pick many`}),(0,k.jsx)(`p`,{className:`text-muted`,children:e.emptyOptionLabel?`Empty option: ${e.emptyOptionLabel}`:`No empty option`})]})}function O(e,t){return{option:(0,k.jsxs)(`span`,{className:`flex min-w-0 flex-col`,children:[(0,k.jsx)(`span`,{className:`font-medium`,children:e.label}),(0,k.jsx)(`span`,{className:`font-mono text-xs text-muted`,children:e.name})]}),controller:e.controller?[e.controller.name,e.controller.version].filter(Boolean).join(` `):`None`,mode:e.mode,accepted:(0,k.jsx)(s,{type:`Accepted`,condition:e.accepted,unknown:e.healthUnknown}),ready:(0,k.jsxs)(k.Fragment,{children:[(0,k.jsx)(s,{type:`Ready`,condition:e.ready,unknown:e.healthUnknown}),e.healthySince&&(0,k.jsx)(`time`,{dateTime:e.healthySince,className:`${A} mt-1`,children:`Healthy since ${a(e.healthySince,t)} ago`})]}),requests:e.requests?(0,k.jsxs)(k.Fragment,{children:[(0,k.jsx)(`span`,{children:e.requests.total}),e.requests.byEnvironment?e.requests.byEnvironment.map(e=>(0,k.jsx)(`span`,{className:A,children:`${e.environment} ${e.count}`},e.environment)):e.requests.total>0&&(0,k.jsx)(`span`,{className:A,children:`Environments not known`})]}):`Not known`,operatorVersions:e.operatorVersions??`Any`,orphan:e.orphan?`Supported`:`Not supported`}}var k,A,j;function M(){return(M=e((()=>{k=t(),l(),i(),n(),d(),_(),o(),C(),w(),A=`block text-xs text-muted`,j=[{key:`option`,label:`Option`},{key:`controller`,label:`Controller`},{key:`mode`,label:`Mode`},{key:`accepted`,label:`Accepted`},{key:`ready`,label:`Ready`},{key:`requests`,label:`Requests`},{key:`operatorVersions`,label:`Operator versions`},{key:`orphan`,label:`Orphan`}],T.__docgenInfo={description:`RegistrationList is the Plugins page: the registrations in the groups of
their sections, the summary line, and the panel of the charts. While
PluginRegistration is read, charts are the charts that are not installed.
While it is not read, charts must not be empty when the catalog has a
chart. catalogError is the error of the read of the catalog. Each of
errors shows above the groups, and the groups stay. stale holds the kinds
whose watch is not fresh (freshKey), as
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
while Namespace or Environment is not read.`},{key:`operatorVersions`,value:{name:`string`,required:!1}},{key:`orphan`,value:{name:`boolean`,required:!0}}]}}],raw:`RegistrationRow[]`,required:!0}},{key:`unread`,value:{name:`boolean`,required:!1}}]}}],raw:`RegistrationGroup[]`},description:``},summary:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{ healthy: number; notHealthy: number }`,signature:{properties:[{key:`healthy`,value:{name:`number`,required:!0}},{key:`notHealthy`,value:{name:`number`,required:!0}}]}},description:``},charts:{required:!0,tsType:{name:`unknown`},description:``},catalogError:{required:!1,tsType:{name:`string`},description:``},sections:{required:!0,tsType:{name:`unknown`},description:``},errors:{required:!0,tsType:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`},description:``},stale:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}var N,P,F,I,L,R,z,B,V,H,U,W,G;function K(){return(K=e((()=>{v(),f(),C(),M(),N=e=>({groups:x(e),summary:b(e.registrations),sections:e.sections}),P=(...e)=>{let t=p();return N({...t,registrations:[...t.registrations,...e]})},F={title:`setup/plugins/RegistrationList`,component:T,args:{...N(p()),charts:[],errors:[],now:`2026-11-13T09:00:00Z`}},I={name:`Built-in registrations`},L={name:`A registration that is not accepted and one that is not ready`,args:P(m(`rustfs`,{label:`RustFS bucket`,sections:[`backup-storage`],mode:`Default`,controller:{name:`rustfs`,version:`0.1.0`},operatorVersions:`>=0.3.0 <1.0.0`},[h(`Accepted`,`False`,`DefaultConflict`,`In section backup-storage, these registrations also have mode Default: s3-bucket`),h(`Ready`,`True`,`Healthy`)]),m(`aws-kms`,{label:`AWS KMS key`,sections:[`encryption-key`],controller:{name:`aws-kms`,version:`0.1.0`},deletionPolicies:[`Delete`]},[h(`Accepted`,`True`,`Healthy`),h(`Ready`,`False`,`CredentialsInvalid`,`The role cannot be assumed`)]))},R={name:`A registration in two sections`,args:P(m(`aws-identity`,{label:`AWS IAM role`,sections:[`cloud-identity`,`connectors-permissions`],controller:{name:`aws-identity`,version:`0.1.0`}},[h(`Accepted`,`True`,`Healthy`),h(`Ready`,`True`,`Healthy`)]))},z={name:`A registration whose section is gone`,args:P(m(`vault-transit`,{label:`Vault transit key`,sections:[`secret-store`],controller:{name:`vault-transit`,version:`0.2.0`}},[h(`Accepted`,`False`,`SectionMissing`,`These sections do not exist: secret-store`)]))},B={name:`With the catalog panel`,args:{charts:g.charts}},V={name:`The registrations cannot be read`,args:{...N({...p(),registrations:[],errors:[{kind:`PluginRegistration`,message:`forbidden`}]}),errors:[{kind:`PluginRegistration`,message:`forbidden`}],charts:g.charts}},H={name:`The registrations watch failed after the page loaded`,args:{...N({...p(),errors:[{kind:`PluginRegistration`,message:`watch closed`}]}),errors:[{kind:`PluginRegistration`,message:`watch closed`}],charts:g.charts}},U={name:`Sections, requests, and environments not read yet`,args:{...N({...p(),sections:[],stale:[`PluginSection`,`PluginRequest`,`Environment`]}),sections:[],stale:[`PluginSection`,`PluginRequest`,`Environment`]}},W={name:`The catalog cannot be read`,args:{catalogError:`reading catalog /app/catalog/official-plugins.json: ENOENT: no such file or directory`}},G=[`BuiltInRegistrations`,`NotAcceptedAndNotReady`,`InTwoSections`,`SectionGone`,`WithTheCatalogPanel`,`RegistrationsNotRead`,`RegistrationsWatchFailed`,`ListsNotAnswered`,`CatalogNotRead`],I.parameters={...I.parameters,docs:{...I.parameters?.docs,source:{originalSource:`{
  name: "Built-in registrations"
}`,...I.parameters?.docs?.source}}},L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
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
}`,...L.parameters?.docs?.source}}},R.parameters={...R.parameters,docs:{...R.parameters?.docs,source:{originalSource:`{
  name: "A registration in two sections",
  args: withRegistrations(pluginRegistration("aws-identity", {
    label: "AWS IAM role",
    sections: ["cloud-identity", "connectors-permissions"],
    controller: {
      name: "aws-identity",
      version: "0.1.0"
    }
  }, [condition("Accepted", "True", "Healthy"), condition("Ready", "True", "Healthy")]))
}`,...R.parameters?.docs?.source}}},z.parameters={...z.parameters,docs:{...z.parameters?.docs,source:{originalSource:`{
  name: "A registration whose section is gone",
  args: withRegistrations(pluginRegistration("vault-transit", {
    label: "Vault transit key",
    sections: ["secret-store"],
    controller: {
      name: "vault-transit",
      version: "0.2.0"
    }
  }, [condition("Accepted", "False", "SectionMissing", "These sections do not exist: secret-store")]))
}`,...z.parameters?.docs?.source}}},B.parameters={...B.parameters,docs:{...B.parameters?.docs,source:{originalSource:`{
  name: "With the catalog panel",
  args: {
    charts: catalog.charts as CatalogChart[]
  }
}`,...B.parameters?.docs?.source}}},V.parameters={...V.parameters,docs:{...V.parameters?.docs,source:{originalSource:`{
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
}`,...V.parameters?.docs?.source}}},H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
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
}`,...H.parameters?.docs?.source}}},U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
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
}`,...U.parameters?.docs?.source}}},W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "The catalog cannot be read",
  args: {
    catalogError: "reading catalog /app/catalog/official-plugins.json: ENOENT: no such file or directory"
  }
}`,...W.parameters?.docs?.source}}}})))()}K();export{I as BuiltInRegistrations,W as CatalogNotRead,R as InTwoSections,U as ListsNotAnswered,L as NotAcceptedAndNotReady,V as RegistrationsNotRead,H as RegistrationsWatchFailed,z as SectionGone,B as WithTheCatalogPanel,G as __namedExportsOrder,F as default};