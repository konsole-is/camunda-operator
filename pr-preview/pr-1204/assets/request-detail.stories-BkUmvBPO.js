import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,o as r}from"./iframe-CPFD3Ulq.js";import{n as i,t as a}from"./not-read-RuqQXk-B.js";import{a as o,c as s,i as c,n as l,o as u,s as d}from"./table-DsmZWsJ3.js";import{n as ee,t as f}from"./age-f6c5OgJq.js";import{n as te,t as ne}from"./condition-list-DsE7u4-S.js";import{n as re,t as ie}from"./condition-badge-pLLzOr80.js";import{a as ae,t as oe}from"./deletion-choice-JZIwd-SH.js";import{a as p,i as m,o as h}from"./claim-tab-CJqKDpJR.js";import{n as g,t as _}from"./detail-section-uryUP_s7.js";import{n as v,t as se}from"./values-ImnVLqKX.js";import{c as ce,l as le,n as ue}from"./grants-XyVqxCuQ.js";import{a as de,c as y,d as fe,f as pe,i as b,l as me,m as he,n as ge,o as x,p as _e,r as ve,s as ye,t as be,u as S}from"./request-fixtures-BC3FoqA5.js";import{n as C,t as w}from"./status-chip-lMnIRT6n.js";import{n as T,r as E}from"./component-card-DBeLYWRV.js";import{r as D,t as O}from"./grants-tab-CazJIdDr.js";function k({detail:e,unread:t,now:n}){let r=e=>t.includes(e),i=r(`PluginRegistration`),o=r(`Grant`),s=r(`PluginSection`),c=e.grants.length>0||o||s;return(0,R.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[e.summary.length>0&&(0,R.jsxs)(_,{title:`Summary`,region:!0,children:[(0,R.jsx)(A,{rows:e.summary}),e.summaryTime&&(0,R.jsxs)(`p`,{className:`text-xs text-muted`,children:[`Updated `,f(e.summaryTime,n),` ago`]})]}),e.connected&&(0,R.jsx)(_,{title:`Connected`,region:!0,children:(0,R.jsx)(M,{refs:e.connected,unread:t})}),e.connectors&&(0,R.jsx)(_,{title:`Principal`,region:!0,children:(0,R.jsxs)(p,{children:[(0,R.jsx)(m,{label:`Principal`,children:(0,R.jsx)(`span`,{className:`font-mono text-xs break-all`,children:e.connectors.principal??`Not written yet`})}),(0,R.jsx)(m,{label:`ServiceAccount`,children:(0,R.jsx)(`span`,{className:`font-mono text-xs break-all`,children:e.connectors.serviceAccount??`Not set`})})]})}),c&&(0,R.jsxs)(_,{title:`Grants`,note:e.clusterGrants?`Every grant of the cluster`:void 0,region:!0,children:[(o||s)&&(0,R.jsx)(a,{kinds:[...o?[`Grant`]:[],...s?[`PluginSection`]:[]]}),i&&e.grants.length>0&&(0,R.jsx)(a,{kinds:[`PluginRegistration`]}),(0,R.jsx)(N,{grants:e.grants.filter(e=>i||!e.templateGone),withRequest:e.clusterGrants,unread:o,templatesUnread:i,now:n}),!i&&e.grants.some(e=>e.templateGone)&&(0,R.jsxs)(`div`,{className:`flex flex-col gap-2 border-t border-border pt-3`,children:[(0,R.jsx)(`p`,{className:`text-sm font-medium text-text`,children:`Grants of a template the registration no longer has`}),(0,R.jsx)(`p`,{className:`text-sm text-text-2`,children:`The role keeps the statement of each of these Grants until the controller of this registration deletes that Grant.`}),(0,R.jsx)(N,{grants:e.grants.filter(e=>e.templateGone),withRequest:e.clusterGrants,unread:o,templatesUnread:!1,now:n})]})]}),e.outputs.length>0&&(0,R.jsx)(_,{title:`Outputs`,region:!0,children:(0,R.jsx)(Se,{detail:e,unread:s||r(`CamundaClusterClaim`)})}),(e.inputs.length>0||i)&&(0,R.jsxs)(_,{title:`Inputs`,region:!0,children:[i&&(0,R.jsx)(a,{kinds:[`PluginRegistration`]}),e.inputs.length>0&&(0,R.jsx)(Ce,{detail:e})]}),(e.usedBy.length>0||i)&&(0,R.jsxs)(_,{title:`Used by`,region:!0,children:[i&&(0,R.jsx)(a,{kinds:[`PluginRegistration`]}),e.usedBy.length>0&&(0,R.jsx)(`ul`,{className:`flex flex-wrap gap-1.5`,children:e.usedBy.map(e=>(0,R.jsx)(`li`,{className:`rounded border border-border px-2 py-0.5 text-sm text-text`,children:e},e))})]}),e.links.length>0&&(0,R.jsx)(_,{title:`Links`,region:!0,children:(0,R.jsx)(`ul`,{className:`flex flex-col gap-2`,children:e.links.map((e,t)=>(0,R.jsx)(`li`,{className:`text-sm`,children:`error`in e?(0,R.jsx)(L,{row:e}):(0,R.jsx)(`a`,{href:e.url,target:`_blank`,rel:`noreferrer`,className:`relative font-medium text-text underline underline-offset-4 hit-area hover:text-accent`,children:e.name})},t))})}),e.resources.length>0&&(0,R.jsx)(_,{title:`Resources`,region:!0,children:(0,R.jsx)(we,{detail:e,claimUnread:r(`CamundaClusterClaim`)})})]})}function A({rows:e}){let t=e.filter(e=>!v(e));return(0,R.jsxs)(R.Fragment,{children:[e.filter(v).map((e,t)=>(0,R.jsx)(`p`,{className:`text-sm text-bad`,children:e.error},t)),t.length>0&&(0,R.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-4 gap-y-2 text-sm`,children:t.map((e,t)=>(0,R.jsx)(j,{row:e},t))})]})}function j({row:e}){let t=e.severity===`Info`?void 0:e.severity,r=t===`Error`?`text-bad`:t===`Warning`?`text-warn`:``;return(0,R.jsxs)(R.Fragment,{children:[(0,R.jsx)(`dt`,{className:`text-muted`,children:e.label}),(0,R.jsxs)(`dd`,{className:n(`min-w-0 break-words`,r),children:[t&&(0,R.jsx)(T,{severity:t}),(0,R.jsx)(`span`,{className:n(`text-text`,r),children:e.value})]})]})}function M({refs:e,unread:t}){return(0,R.jsxs)(`table`,{className:`w-full text-sm`,children:[(0,R.jsx)(u,{children:(0,R.jsxs)(d,{children:[(0,R.jsx)(F,{children:`Kind`}),(0,R.jsx)(F,{children:`Name`}),(0,R.jsx)(F,{children:`Ready`})]})}),(0,R.jsx)(l,{children:e.map(e=>(0,R.jsxs)(d,{children:[(0,R.jsx)(P,{children:e.kind}),(0,R.jsx)(P,{mono:!0,children:e.name}),(0,R.jsx)(P,{children:t.includes(e.kind)?(0,R.jsx)(ie,{type:`Ready`,unknown:!0}):e.found?(0,R.jsx)(w,{condition:e.ready}):(0,R.jsx)(`span`,{className:`text-warn`,children:`Not in the namespace`})})]},`${e.kind}/${e.name}`))})]})}function N({grants:e,withRequest:t,unread:n,templatesUnread:r,now:i}){return e.length===0?null:(0,R.jsx)(`ul`,{className:`flex flex-col divide-y divide-border`,children:e.map(e=>(0,R.jsxs)(`li`,{className:`flex min-w-0 flex-col gap-2 py-3 first:pt-0 last:pb-0`,children:[(0,R.jsx)(`span`,{className:`font-mono text-sm break-all text-text`,children:e.name}),(0,R.jsxs)(p,{children:[(0,R.jsx)(m,{label:`Template`,children:e.template||`-`}),t&&(0,R.jsx)(m,{label:`Request`,children:(0,R.jsx)(`span`,{className:`font-mono break-all`,children:e.request??`-`})})]}),!n&&!e.error&&(0,R.jsx)(xe,{grant:e,templatesUnread:r}),e.errors.map(e=>(0,R.jsx)(`p`,{className:`text-sm text-bad`,children:e},e)),e.statement!==void 0&&(0,R.jsx)(`pre`,{"aria-label":`Statement of ${e.name}`,tabIndex:0,className:`overflow-x-auto rounded-md bg-panel-2 p-3 text-xs outline-none focus-visible:ring-3 focus-visible:ring-ring/50`,children:(0,R.jsx)(`code`,{children:e.statement})}),n?(0,R.jsx)(O,{}):!e.error&&(0,R.jsx)(ne,{label:`Conditions of ${e.name}`,conditions:e.conditions,now:i})]},e.name))})}function xe({grant:e,templatesUnread:t}){switch(e.state){case`oldTemplate`:return e.templateGone||t?null:(0,R.jsx)(`p`,{className:`text-sm text-text-2`,children:ue.TemplateChanged});case`refused`:{let t=le(e);return t?(0,R.jsx)(`p`,{className:`text-sm text-warn`,children:t}):null}case`waiting`:return(0,R.jsx)(`p`,{className:`text-sm text-muted`,children:`Not applied yet.`});default:return null}}function Se({detail:e,unread:t}){return(0,R.jsxs)(`table`,{className:`w-full text-sm`,children:[(0,R.jsx)(u,{children:(0,R.jsxs)(d,{children:[(0,R.jsx)(F,{children:`Output`}),(0,R.jsx)(F,{children:`Value`}),(0,R.jsx)(F,{children:`Fills`})]})}),(0,R.jsx)(l,{children:e.outputs.map((e,n)=>`error`in e?(0,R.jsx)(I,{row:e,span:3},n):(0,R.jsxs)(d,{children:[(0,R.jsx)(P,{mono:!0,children:e.key}),(0,R.jsx)(P,{mono:!0,children:e.value}),(0,R.jsx)(P,{children:t?`Not known`:e.fills.length>0?e.fills.join(`, `):`No field`})]},e.key))})]})}function Ce({detail:e}){return(0,R.jsxs)(`table`,{className:`w-full text-sm`,children:[(0,R.jsx)(u,{children:(0,R.jsxs)(d,{children:[(0,R.jsx)(F,{children:`Input`}),(0,R.jsx)(F,{children:`From`}),(0,R.jsx)(F,{children:`Value`})]})}),(0,R.jsx)(l,{children:e.inputs.map(e=>(0,R.jsxs)(d,{children:[(0,R.jsx)(P,{mono:!0,children:e.name}),(0,R.jsx)(P,{children:`${e.sectionLabel} / ${e.output}`}),(0,R.jsx)(P,{mono:e.value!==void 0,children:e.selected?e.value??`Waiting`:`The claim selects no option for ${e.sectionLabel}.`})]},e.name))})]})}function we({detail:e,claimUnread:t}){let n=t&&!e.deleting,r=t=>n?`Not known`:e.controlled?t??`-`:`Not controlled by this claim`;return(0,R.jsxs)(R.Fragment,{children:[(0,R.jsx)(`p`,{className:`text-xs text-muted`,children:n?`Not known: the fleet manager has no current list of CamundaClusterClaim, so it cannot say what happens to each resource when the claim is deleted.`:e.controlled?`When the claim is deleted: ${oe[e.policy]}.`:`The claim does not control this request, so its deletion does not decide what happens to these resources.`}),(0,R.jsxs)(`table`,{className:`w-full text-sm`,children:[(0,R.jsx)(u,{children:(0,R.jsxs)(d,{children:[(0,R.jsx)(F,{children:`Kind`}),(0,R.jsx)(F,{children:`Name`}),(0,R.jsx)(F,{children:`ID`}),(0,R.jsx)(F,{children:`Location`}),(0,R.jsx)(F,{children:`Role`}),(0,R.jsx)(F,{children:`After the claim is deleted`})]})}),(0,R.jsx)(l,{children:e.resources.map((e,t)=>`error`in e?(0,R.jsx)(I,{row:e,span:6},t):(0,R.jsxs)(d,{children:[(0,R.jsx)(P,{children:e.kind}),(0,R.jsx)(P,{mono:!0,children:e.name}),(0,R.jsx)(P,{mono:!0,children:e.id??`-`}),(0,R.jsx)(P,{children:e.location??`-`}),(0,R.jsx)(P,{children:e.role??`-`}),(0,R.jsx)(P,{children:r(e.fate)})]},`${e.kind}/${e.name}`))})]})]})}function P({mono:e=!1,children:t}){return(0,R.jsx)(c,{className:n(`align-top whitespace-normal wrap-anywhere`,e&&`font-mono text-xs`),children:t})}function F({children:e}){return(0,R.jsx)(o,{className:`h-auto py-2 align-bottom whitespace-normal`,children:e})}function I({row:e,span:t}){return(0,R.jsx)(d,{children:(0,R.jsx)(c,{colSpan:t,className:`whitespace-normal`,children:(0,R.jsx)(L,{row:e})})})}function L({row:e}){return(0,R.jsx)(`span`,{className:`text-sm text-bad`,children:e.error})}var R;function z(){return(z=e((()=>{R=t(),ee(),te(),ae(),i(),C(),re(),ce(),D(),se(),s(),r(),h(),E(),g(),k.__docgenInfo={description:`RequestOverview shows detail below the conditions of a request: one box
for each block that has something to show. unread holds the kinds that
the page could not read or that are out of date. A box that depends on
one of them names the kind, and shows no fate, no condition, and no
empty list from it: CamundaClusterClaim for the fates, Grant and
PluginSection for the grants, PluginRegistration for the inputs and the
users, and the kind of each connected contract for its Ready. While
CamundaClusterClaim or PluginSection is in unread, the fields that
outputs fill show as Not known. now is the time that the ages
count to.`,methods:[],displayName:`RequestOverview`,props:{detail:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  summary: (SummaryRow | Invalid)[];
  summaryTime?: string;
  outputs: (OutputRow | Invalid)[];
  inputs: InputRow[];
  links: (LinkRow | Invalid)[];
  resources: (ResourceFateRow | Invalid)[];
  controlled: boolean;
  deleting: boolean;
  policy: DeletionPolicy;
  grants: GrantRow[];
  clusterGrants: boolean;
  connected?: ConnectedRef[];
  connectors?: { principal?: string; serviceAccount?: string };
  usedBy: string[];
}`,signature:{properties:[{key:`summary`,value:{name:`Array`,elements:[{name:`unknown`}],raw:`(SummaryRow | Invalid)[]`,required:!0}},{key:`summaryTime`,value:{name:`string`,required:!1}},{key:`outputs`,value:{name:`Array`,elements:[{name:`unknown`}],raw:`(OutputRow | Invalid)[]`,required:!0}},{key:`inputs`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  section: string;
  sectionLabel: string;
  output: string;
  selected: boolean;
  value?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`section`,value:{name:`string`,required:!0}},{key:`sectionLabel`,value:{name:`string`,required:!0}},{key:`output`,value:{name:`string`,required:!0}},{key:`selected`,value:{name:`boolean`,required:!0}},{key:`value`,value:{name:`string`,required:!1}}]}}],raw:`InputRow[]`,required:!0}},{key:`links`,value:{name:`Array`,elements:[{name:`unknown`}],raw:`(LinkRow | Invalid)[]`,required:!0}},{key:`resources`,value:{name:`Array`,elements:[{name:`unknown`}],raw:`(ResourceFateRow | Invalid)[]`,required:!0}},{key:`controlled`,value:{name:`boolean`,required:!0}},{key:`deleting`,value:{name:`boolean`,required:!0}},{key:`policy`,value:{name:`DeletionPolicy`,required:!0}},{key:`grants`,value:{name:`Array`,elements:[{name:`intersection`,raw:`GrantView & {
  template: string;
  statement?: string;
  conditions: ConditionRow[];
  templateGone: boolean;
  errors: string[];
}`,elements:[{name:`GrantView`},{name:`signature`,type:`object`,raw:`{
  template: string;
  statement?: string;
  conditions: ConditionRow[];
  templateGone: boolean;
  errors: string[];
}`,signature:{properties:[{key:`template`,value:{name:`string`,required:!0}},{key:`statement`,value:{name:`string`,required:!1}},{key:`conditions`,value:{name:`Array`,elements:[{name:`ConditionRow`}],raw:`ConditionRow[]`,required:!0}},{key:`templateGone`,value:{name:`boolean`,required:!0}},{key:`errors`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}}]}}]}],raw:`GrantRow[]`,required:!0}},{key:`clusterGrants`,value:{name:`boolean`,required:!0}},{key:`connected`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  kind: ContractKind;
  name: string;
  found: boolean;
  ready?: ConditionRow;
}`,signature:{properties:[{key:`kind`,value:{name:`union`,raw:`"SecondaryStorageConfig" | "ObjectStorageConfig" | "DatabaseConfig"`,elements:[{name:`literal`,value:`"SecondaryStorageConfig"`},{name:`literal`,value:`"ObjectStorageConfig"`},{name:`literal`,value:`"DatabaseConfig"`}],required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`found`,value:{name:`boolean`,required:!0}},{key:`ready`,value:{name:`ConditionRow`,required:!1}}]}}],raw:`ConnectedRef[]`,required:!1}},{key:`connectors`,value:{name:`signature`,type:`object`,raw:`{ principal?: string; serviceAccount?: string }`,signature:{properties:[{key:`principal`,value:{name:`string`,required:!1}},{key:`serviceAccount`,value:{name:`string`,required:!1}}]},required:!1}},{key:`usedBy`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}}]}},description:``},unread:{required:!0,tsType:{name:`unknown`},description:``},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}function B(e,t={}){let n=ge(S,t.deletionPolicy),r=me.map(e=>t.request&&e.spec.section===t.request.spec.section?t.request:e),i=ve(n,r);return he({component:i.find(t=>t.section===e),claim:n,cluster:`orders`,sections:be,registrations:ye,components:i,grants:t.grants??[],contracts:{SecondaryStorageConfig:[de(`orders-storage`)]}})}var V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{z(),fe(),_e(),V=[b(`Applied`,`True`,`Healthy`),b(`MatchesTemplate`,`True`,`Healthy`),b(`Ready`,`True`,`Healthy`)],H={title:`cluster/RequestOverview`,component:k,args:{unread:[],now:pe}},U={name:`Cloud identity with the grants of the cluster`,args:{detail:B(`cloud-identity`,{grants:[x(`orders-read`,`backup-storage`,V),x(`orders-key`,`encryption-key`)]})}},W={name:`Backup storage with a rejected grant`,args:{detail:B(`backup-storage`,{request:y(`backup-storage`,{summary:[{label:`Bucket`,value:`orders-backups`}],conditions:[b(`OutputsReady`,`True`,`Healthy`),b(`Ready`,`False`,`GrantRejected`,`Grant orders-read has Ready=False`)]}),grants:[x(`orders-read`,`backup-storage`,[b(`Applied`,`False`,`OverCeiling`,`s3:DeleteObject is not in the ceiling of the identity`),b(`MatchesTemplate`,`True`,`Healthy`),b(`Ready`,`False`,`OverCeiling`,`s3:DeleteObject is not in the ceiling of the identity`)])]})}},G={name:`A grant applied with the old template`,args:{detail:B(`backup-storage`,{grants:[x(`orders-read`,`backup-storage`,[b(`Applied`,`True`,`Healthy`),b(`MatchesTemplate`,`False`,`TemplateChanged`,`The fragment no longer matches its template`),b(`Ready`,`False`,`TemplateChanged`,`The fragment no longer matches its template`)])]})}},K={name:`Encryption key used by two sections`,args:{detail:B(`encryption-key`)}},q={name:`Connectors permissions`,args:{detail:B(`connectors-permissions`)}},J={name:`Access waiting for endpoints`,args:{detail:B(`access`,{request:y(`access`,{summary:[{label:`Host`,value:`Waiting for the Gateway`,severity:`Warning`}],conditions:[b(`OutputsReady`,`False`,`WaitingForEndpoints`),b(`Ready`,`False`,`WaitingForEndpoints`)]})})}},Y={name:`Existing secondary storage, connected`,args:{detail:B(`secondary-storage`)}},X={name:`Resources kept and deleted`,args:{detail:B(`backup-storage`,{deletionPolicy:`Orphan`})}},Z={name:`Grants and the claim not read`,args:{detail:B(`backup-storage`,{grants:[x(`orders-read`,`backup-storage`,V)]}),unread:[`Grant`,`CamundaClusterClaim`]}},Q=[`CloudIdentityWithGrants`,`BackupStorageWithARejectedGrant`,`GrantWithAnOldTemplate`,`EncryptionKeyUsedByTwoSections`,`ConnectorsPermissions`,`AccessWaitingForEndpoints`,`ExistingSecondaryStorageConnected`,`ResourcesKeptAndDeleted`,`NotRead`],U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
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