import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,o as r}from"./iframe-DSVb3gGx.js";import{a as i,c as a,i as o,n as s,o as c,s as l}from"./table-Bvpb3PCv.js";import{n as ee,t as te}from"./age-f6c5OgJq.js";import{n as ne,t as re}from"./condition-list-Dfi1hUEV.js";import{n as u,t as d}from"./condition-badge-CWMeJ3_v.js";import{a as f,t as ie}from"./deletion-choice-Cj2HSJgm.js";import{a as ae,i as p,r as m}from"./claim-tab-eG-ja0pj.js";import{n as oe,t as h}from"./detail-section-sL9jpk44.js";import{n as g,t as _}from"./not-read-B0beG25C.js";import{a as v,c as y,d as se,f as ce,i as b,l as le,m as ue,n as de,o as x,p as fe,r as pe,s as me,t as he,u as ge}from"./request-fixtures-f4dqiMrc.js";import{n as _e,t as ve}from"./status-chip-BWlm_38Q.js";import{n as S,r as C}from"./component-card-i1Sr64qC.js";function w({detail:e,unread:t,now:n}){let r=e=>t.includes(e),i=r(`PluginRegistration`),a=r(`Grant`),o=e.grants.length>0||a&&e.clusterGrants;return(0,L.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[e.summary.length>0&&(0,L.jsxs)(h,{title:`Summary`,region:!0,children:[(0,L.jsx)(T,{rows:e.summary}),e.summaryTime&&(0,L.jsxs)(`p`,{className:`text-xs text-muted`,children:[`Updated `,te(e.summaryTime,n),` ago`]})]}),e.connected&&(0,L.jsx)(h,{title:`Connected`,region:!0,children:(0,L.jsx)(D,{refs:e.connected,unread:t})}),e.connectors&&(0,L.jsx)(h,{title:`Principal`,region:!0,children:(0,L.jsxs)(p,{children:[(0,L.jsx)(m,{label:`Principal`,children:(0,L.jsx)(`span`,{className:`font-mono text-xs break-all`,children:e.connectors.principal??`Not written yet`})}),(0,L.jsx)(m,{label:`ServiceAccount`,children:(0,L.jsx)(`span`,{className:`font-mono text-xs break-all`,children:e.connectors.serviceAccount})})]})}),o&&(0,L.jsxs)(h,{title:`Grants`,note:e.clusterGrants?`Every grant of the cluster`:void 0,region:!0,children:[a&&(0,L.jsx)(_,{kinds:[`Grant`]}),(0,L.jsx)(O,{grants:e.grants,withRequest:e.clusterGrants,unread:a,now:n})]}),e.outputs.length>0&&(0,L.jsx)(h,{title:`Outputs`,region:!0,children:(0,L.jsx)(A,{detail:e,unread:r(`PluginSection`)})}),(e.inputs.length>0||i)&&(0,L.jsxs)(h,{title:`Inputs`,region:!0,children:[i&&(0,L.jsx)(_,{kinds:[`PluginRegistration`]}),e.inputs.length>0&&(0,L.jsx)(j,{detail:e})]}),(e.usedBy.length>0||i)&&(0,L.jsxs)(h,{title:`Used by`,region:!0,children:[i&&(0,L.jsx)(_,{kinds:[`PluginRegistration`]}),e.usedBy.length>0&&(0,L.jsx)(`ul`,{className:`flex flex-wrap gap-1.5`,children:e.usedBy.map(e=>(0,L.jsx)(`li`,{className:`rounded border border-border px-2 py-0.5 text-sm text-text`,children:e},e))})]}),e.links.length>0&&(0,L.jsx)(h,{title:`Links`,region:!0,children:(0,L.jsx)(`ul`,{className:`flex flex-col gap-2`,children:e.links.map((e,t)=>(0,L.jsx)(`li`,{className:`text-sm`,children:`error`in e?(0,L.jsx)(I,{row:e}):(0,L.jsx)(`a`,{href:e.url,target:`_blank`,rel:`noreferrer`,className:`relative font-medium text-text underline underline-offset-4 hit-area hover:text-accent`,children:e.name})},t))})}),e.resources.length>0&&(0,L.jsx)(h,{title:`Resources`,region:!0,children:(0,L.jsx)(M,{detail:e,claimUnread:r(`CamundaClusterClaim`)})})]})}function T({rows:e}){return(0,L.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-4 gap-y-2 text-sm`,children:e.map((e,t)=>`error`in e?(0,L.jsx)(`dd`,{className:`col-span-2`,children:(0,L.jsx)(I,{row:e})},t):(0,L.jsx)(E,{row:e},t))})}function E({row:e}){let t=e.severity===`Info`?void 0:e.severity,r=t===`Error`?`text-bad`:t===`Warning`?`text-warn`:``;return(0,L.jsxs)(L.Fragment,{children:[(0,L.jsx)(`dt`,{className:`text-muted`,children:e.label}),(0,L.jsxs)(`dd`,{className:n(`min-w-0 break-words`,r),children:[t&&(0,L.jsx)(S,{severity:t}),(0,L.jsx)(`span`,{className:n(`text-text`,r),children:e.value})]})]})}function D({refs:e,unread:t}){return(0,L.jsxs)(`table`,{className:`w-full text-sm`,children:[(0,L.jsx)(c,{children:(0,L.jsxs)(l,{children:[(0,L.jsx)(P,{children:`Kind`}),(0,L.jsx)(P,{children:`Name`}),(0,L.jsx)(P,{children:`Ready`})]})}),(0,L.jsx)(s,{children:e.map(e=>(0,L.jsxs)(l,{children:[(0,L.jsx)(N,{children:e.kind}),(0,L.jsx)(N,{mono:!0,children:e.name}),(0,L.jsx)(N,{children:t.includes(e.kind)?(0,L.jsx)(d,{type:`Ready`,unknown:!0}):e.found?(0,L.jsx)(ve,{condition:e.ready}):(0,L.jsx)(`span`,{className:`text-warn`,children:`Not in the namespace`})})]},`${e.kind}/${e.name}`))})]})}function O({grants:e,withRequest:t,unread:n,now:r}){return e.length===0?null:(0,L.jsx)(`ul`,{className:`flex flex-col divide-y divide-border`,children:e.map(e=>(0,L.jsxs)(`li`,{className:`flex min-w-0 flex-col gap-2 py-3 first:pt-0 last:pb-0`,children:[(0,L.jsx)(`span`,{className:`font-mono text-sm break-all text-text`,children:e.name}),(0,L.jsxs)(p,{children:[(0,L.jsx)(m,{label:`Template`,children:e.template||`-`}),t&&(0,L.jsx)(m,{label:`Request`,children:(0,L.jsx)(`span`,{className:`font-mono break-all`,children:e.request??`-`})})]}),!n&&(0,L.jsx)(k,{grant:e}),e.errors.map(e=>(0,L.jsx)(`p`,{className:`text-sm text-bad`,children:e},e)),e.statement!==void 0&&(0,L.jsx)(`pre`,{"aria-label":`Statement of ${e.name}`,tabIndex:0,className:`overflow-x-auto rounded-md bg-panel-2 p-3 text-xs outline-none focus-visible:ring-3 focus-visible:ring-ring/50`,children:(0,L.jsx)(`code`,{children:e.statement})}),n?(0,L.jsx)(`span`,{className:`flex flex-wrap gap-1.5`,children:R.map(e=>(0,L.jsx)(d,{type:e,unknown:!0},e))}):(0,L.jsx)(re,{label:`Conditions of ${e.name}`,conditions:e.conditions,now:r})]},e.name))})}function k({grant:e}){switch(e.state){case`oldTemplate`:return(0,L.jsx)(`p`,{className:`text-sm text-text-2`,children:`Applied with the old template. The controller of the option writes the Grant again from the new template.`});case`refused`:return(0,L.jsx)(`p`,{className:`text-sm text-warn`,children:`Refused. The conditions say why.`});case`waiting`:return(0,L.jsx)(`p`,{className:`text-sm text-muted`,children:`Not applied yet.`});default:return null}}function A({detail:e,unread:t}){return(0,L.jsxs)(`table`,{className:`w-full text-sm`,children:[(0,L.jsx)(c,{children:(0,L.jsxs)(l,{children:[(0,L.jsx)(P,{children:`Output`}),(0,L.jsx)(P,{children:`Value`}),(0,L.jsx)(P,{children:`Fills`})]})}),(0,L.jsx)(s,{children:e.outputs.map((e,n)=>`error`in e?(0,L.jsx)(F,{row:e,span:3},n):(0,L.jsxs)(l,{children:[(0,L.jsx)(N,{mono:!0,children:e.key}),(0,L.jsx)(N,{mono:!0,children:e.value}),(0,L.jsx)(N,{children:t?`Not known`:e.fills.length>0?e.fills.join(`, `):`No field`})]},e.key))})]})}function j({detail:e}){return(0,L.jsxs)(`table`,{className:`w-full text-sm`,children:[(0,L.jsx)(c,{children:(0,L.jsxs)(l,{children:[(0,L.jsx)(P,{children:`Input`}),(0,L.jsx)(P,{children:`From`}),(0,L.jsx)(P,{children:`Value`})]})}),(0,L.jsx)(s,{children:e.inputs.map(e=>(0,L.jsxs)(l,{children:[(0,L.jsx)(N,{mono:!0,children:e.name}),(0,L.jsx)(N,{children:`${e.sectionLabel} / ${e.output}`}),(0,L.jsx)(N,{mono:e.value!==void 0,children:e.value??`Waiting`})]},e.name))})]})}function M({detail:e,claimUnread:t}){let n=n=>t?`Not known`:e.controlled?n??`-`:`Not controlled by this claim`;return(0,L.jsxs)(L.Fragment,{children:[(0,L.jsx)(`p`,{className:`text-xs text-muted`,children:t?`Not known: the page could not read CamundaClusterClaim, so it cannot say what happens to each resource when the claim is deleted.`:e.controlled?`When the claim is deleted: ${ie[e.policy]}.`:`The claim does not control this request, so its deletion does not decide what happens to these resources.`}),(0,L.jsxs)(`table`,{className:`w-full text-sm`,children:[(0,L.jsx)(c,{children:(0,L.jsxs)(l,{children:[(0,L.jsx)(P,{children:`Kind`}),(0,L.jsx)(P,{children:`Name`}),(0,L.jsx)(P,{children:`ID`}),(0,L.jsx)(P,{children:`Location`}),(0,L.jsx)(P,{children:`Role`}),(0,L.jsx)(P,{children:`After the claim is deleted`})]})}),(0,L.jsx)(s,{children:e.resources.map((e,t)=>`error`in e?(0,L.jsx)(F,{row:e,span:6},t):(0,L.jsxs)(l,{children:[(0,L.jsx)(N,{children:e.kind}),(0,L.jsx)(N,{mono:!0,children:e.name}),(0,L.jsx)(N,{mono:!0,children:e.id??`-`}),(0,L.jsx)(N,{children:e.location??`-`}),(0,L.jsx)(N,{children:e.role??`-`}),(0,L.jsx)(N,{children:n(e.fate)})]},`${e.kind}/${e.name}`))})]})]})}function N({mono:e=!1,children:t}){return(0,L.jsx)(o,{className:n(`align-top whitespace-normal wrap-anywhere`,e&&`font-mono text-xs`),children:t})}function P({children:e}){return(0,L.jsx)(i,{className:`h-auto py-2 align-bottom whitespace-normal`,children:e})}function F({row:e,span:t}){return(0,L.jsx)(l,{children:(0,L.jsx)(o,{colSpan:t,className:`whitespace-normal`,children:(0,L.jsx)(I,{row:e})})})}function I({row:e}){return(0,L.jsx)(`span`,{className:`text-sm text-bad`,children:e.error})}var L,R;function z(){return(z=e((()=>{L=t(),ee(),ne(),f(),_e(),u(),a(),r(),ae(),C(),oe(),g(),R=[`Applied`,`MatchesTemplate`,`Ready`],w.__docgenInfo={description:`RequestOverview shows detail below the conditions of a request: one box
for each block that has something to show. unread holds the kinds that
the page could not read or that are out of date. A box that depends on
one of them names the kind, and shows no fate, no condition, and no
empty list from it: CamundaClusterClaim for the fates, Grant for the
grants, PluginRegistration for the inputs and the users, and the kind of
each connected contract for its Ready. now is the time that the ages
count to.`,methods:[],displayName:`RequestOverview`,props:{detail:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  summary: (SummaryRow | Invalid)[];
  summaryTime?: string;
  outputs: (OutputRow | Invalid)[];
  inputs: InputRow[];
  links: (LinkRow | Invalid)[];
  resources: (ResourceFateRow | Invalid)[];
  controlled: boolean;
  policy: DeletionPolicy;
  grants: GrantRow[];
  clusterGrants: boolean;
  connected?: ConnectedRef[];
  connectors?: { principal?: string; serviceAccount: string };
  usedBy: string[];
}`,signature:{properties:[{key:`summary`,value:{name:`Array`,elements:[{name:`unknown`}],raw:`(SummaryRow | Invalid)[]`,required:!0}},{key:`summaryTime`,value:{name:`string`,required:!1}},{key:`outputs`,value:{name:`Array`,elements:[{name:`unknown`}],raw:`(OutputRow | Invalid)[]`,required:!0}},{key:`inputs`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  section: string;
  sectionLabel: string;
  output: string;
  value?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`section`,value:{name:`string`,required:!0}},{key:`sectionLabel`,value:{name:`string`,required:!0}},{key:`output`,value:{name:`string`,required:!0}},{key:`value`,value:{name:`string`,required:!1}}]}}],raw:`InputRow[]`,required:!0}},{key:`links`,value:{name:`Array`,elements:[{name:`unknown`}],raw:`(LinkRow | Invalid)[]`,required:!0}},{key:`resources`,value:{name:`Array`,elements:[{name:`unknown`}],raw:`(ResourceFateRow | Invalid)[]`,required:!0}},{key:`controlled`,value:{name:`boolean`,required:!0}},{key:`policy`,value:{name:`DeletionPolicy`,required:!0}},{key:`grants`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  template: string;
  statement?: string;
  request?: string;
  conditions: ConditionRow[];
  ready?: ConditionRow;
  state: GrantState;
  errors: string[];
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`template`,value:{name:`string`,required:!0}},{key:`statement`,value:{name:`string`,required:!1}},{key:`request`,value:{name:`string`,required:!1}},{key:`conditions`,value:{name:`Array`,elements:[{name:`ConditionRow`}],raw:`ConditionRow[]`,required:!0}},{key:`ready`,value:{name:`ConditionRow`,required:!1}},{key:`state`,value:{name:`union`,raw:`"ready" | "oldTemplate" | "refused" | "waiting"`,elements:[{name:`literal`,value:`"ready"`},{name:`literal`,value:`"oldTemplate"`},{name:`literal`,value:`"refused"`},{name:`literal`,value:`"waiting"`}],required:!0}},{key:`errors`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}}]}}],raw:`GrantRow[]`,required:!0}},{key:`clusterGrants`,value:{name:`boolean`,required:!0}},{key:`connected`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  kind: ContractKind;
  name: string;
  found: boolean;
  ready?: ConditionRow;
}`,signature:{properties:[{key:`kind`,value:{name:`union`,raw:`"SecondaryStorageConfig" | "ObjectStorageConfig" | "DatabaseConfig"`,elements:[{name:`literal`,value:`"SecondaryStorageConfig"`},{name:`literal`,value:`"ObjectStorageConfig"`},{name:`literal`,value:`"DatabaseConfig"`}],required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`found`,value:{name:`boolean`,required:!0}},{key:`ready`,value:{name:`ConditionRow`,required:!1}}]}}],raw:`ConnectedRef[]`,required:!1}},{key:`connectors`,value:{name:`signature`,type:`object`,raw:`{ principal?: string; serviceAccount: string }`,signature:{properties:[{key:`principal`,value:{name:`string`,required:!1}},{key:`serviceAccount`,value:{name:`string`,required:!0}}]},required:!1}},{key:`usedBy`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}}]}},description:``},unread:{required:!0,tsType:{name:`unknown`},description:``},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}function B(e,t={}){let n=de(ge,t.deletionPolicy),r=le.map(e=>t.request&&e.spec.section===t.request.spec.section?t.request:e),i=pe(n,r);return ue({component:i.find(t=>t.section===e),claim:n,cluster:`orders`,sections:he,registrations:me,components:i,grants:t.grants??[],contracts:{SecondaryStorageConfig:[v(`orders-storage`)]}})}var V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{z(),se(),fe(),V=[b(`Applied`,`True`,`Healthy`),b(`MatchesTemplate`,`True`,`Healthy`),b(`Ready`,`True`,`Healthy`)],H={title:`cluster/RequestOverview`,component:w,args:{unread:[],now:ce}},U={name:`Cloud identity with the grants of the cluster`,args:{detail:B(`cloud-identity`,{grants:[x(`orders-read`,`backup-storage`,V),x(`orders-key`,`encryption-key`)]})}},W={name:`Backup storage with a rejected grant`,args:{detail:B(`backup-storage`,{request:y(`backup-storage`,{summary:[{label:`Bucket`,value:`orders-backups`}],conditions:[b(`OutputsReady`,`True`,`Healthy`),b(`Ready`,`False`,`GrantRejected`,`Grant orders-read has Ready=False`)]}),grants:[x(`orders-read`,`backup-storage`,[b(`Applied`,`False`,`OverCeiling`,`s3:DeleteObject is not in the ceiling of the identity`),b(`MatchesTemplate`,`True`,`Healthy`),b(`Ready`,`False`,`OverCeiling`,`s3:DeleteObject is not in the ceiling of the identity`)])]})}},G={name:`A grant applied with the old template`,args:{detail:B(`backup-storage`,{grants:[x(`orders-read`,`backup-storage`,[b(`Applied`,`True`,`Healthy`),b(`MatchesTemplate`,`False`,`TemplateChanged`,`The fragment no longer matches its template`),b(`Ready`,`False`,`TemplateChanged`,`The fragment no longer matches its template`)])]})}},K={name:`Encryption key used by two sections`,args:{detail:B(`encryption-key`)}},q={name:`Connectors permissions`,args:{detail:B(`connectors-permissions`)}},J={name:`Access waiting for endpoints`,args:{detail:B(`access`,{request:y(`access`,{summary:[{label:`Host`,value:`Waiting for the Gateway`,severity:`Warning`}],conditions:[b(`OutputsReady`,`False`,`WaitingForEndpoints`),b(`Ready`,`False`,`WaitingForEndpoints`)]})})}},Y={name:`Existing secondary storage, connected`,args:{detail:B(`secondary-storage`)}},X={name:`Resources kept and deleted`,args:{detail:B(`backup-storage`,{deletionPolicy:`Orphan`})}},Z={name:`Grants and the claim not read`,args:{detail:B(`backup-storage`,{grants:[x(`orders-read`,`backup-storage`,V)]}),unread:[`Grant`,`CamundaClusterClaim`]}},Q=[`CloudIdentityWithGrants`,`BackupStorageWithARejectedGrant`,`GrantWithAnOldTemplate`,`EncryptionKeyUsedByTwoSections`,`ConnectorsPermissions`,`AccessWaitingForEndpoints`,`ExistingSecondaryStorageConnected`,`ResourcesKeptAndDeleted`,`NotRead`],U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
  name: "Cloud identity with the grants of the cluster",
  args: {
    detail: detailOf("cloud-identity", {
      grants: [fixtureGrant("orders-read", "backup-storage", ready), fixtureGrant("orders-key", "encryption-key")]
    })
  }
}`,...U.parameters?.docs?.source}}},W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "Backup storage with a rejected grant",
  args: {
    detail: detailOf("backup-storage", {
      request: fixtureRequest("backup-storage", {
        summary: [{
          label: "Bucket",
          value: "orders-backups"
        }],
        conditions: [fixtureCondition("OutputsReady", "True", "Healthy"), fixtureCondition("Ready", "False", "GrantRejected", "Grant orders-read has Ready=False")]
      }),
      grants: [fixtureGrant("orders-read", "backup-storage", [fixtureCondition("Applied", "False", "OverCeiling", "s3:DeleteObject is not in the ceiling of the identity"), fixtureCondition("MatchesTemplate", "True", "Healthy"), fixtureCondition("Ready", "False", "OverCeiling", "s3:DeleteObject is not in the ceiling of the identity")])]
    })
  }
}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "A grant applied with the old template",
  args: {
    detail: detailOf("backup-storage", {
      grants: [fixtureGrant("orders-read", "backup-storage", [fixtureCondition("Applied", "True", "Healthy"), fixtureCondition("MatchesTemplate", "False", "TemplateChanged", "The fragment no longer matches its template"), fixtureCondition("Ready", "False", "TemplateChanged", "The fragment no longer matches its template")])]
    })
  }
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "Encryption key used by two sections",
  args: {
    detail: detailOf("encryption-key")
  }
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "Connectors permissions",
  args: {
    detail: detailOf("connectors-permissions")
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Access waiting for endpoints",
  args: {
    detail: detailOf("access", {
      request: fixtureRequest("access", {
        summary: [{
          label: "Host",
          value: "Waiting for the Gateway",
          severity: "Warning"
        }],
        conditions: [fixtureCondition("OutputsReady", "False", "WaitingForEndpoints"), fixtureCondition("Ready", "False", "WaitingForEndpoints")]
      })
    })
  }
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Existing secondary storage, connected",
  args: {
    detail: detailOf("secondary-storage")
  }
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Resources kept and deleted",
  args: {
    detail: detailOf("backup-storage", {
      deletionPolicy: "Orphan"
    })
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Grants and the claim not read",
  args: {
    detail: detailOf("backup-storage", {
      grants: [fixtureGrant("orders-read", "backup-storage", ready)]
    }),
    unread: ["Grant", "CamundaClusterClaim"]
  }
}`,...Z.parameters?.docs?.source}}}})))()}$();export{J as AccessWaitingForEndpoints,W as BackupStorageWithARejectedGrant,U as CloudIdentityWithGrants,q as ConnectorsPermissions,K as EncryptionKeyUsedByTwoSections,Y as ExistingSecondaryStorageConnected,G as GrantWithAnOldTemplate,Z as NotRead,X as ResourcesKeptAndDeleted,Q as __namedExportsOrder,H as default};