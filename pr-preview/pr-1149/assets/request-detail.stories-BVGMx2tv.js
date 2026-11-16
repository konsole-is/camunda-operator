import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,o as r}from"./iframe-DzDBGYPJ.js";import{a as i,c as a,i as o,n as s,o as c,s as l}from"./table-80aNasV8.js";import{n as ee,t as u}from"./not-read-TId2yrib.js";import{n as te,t as d}from"./age-f6c5OgJq.js";import{n as ne,t as re}from"./condition-list-C8AN9rpl.js";import{n as f,t as ie}from"./condition-badge-DA-xByDE.js";import{a as ae,t as oe}from"./deletion-choice-BrEzZity.js";import{a as se,i as p,r as m}from"./claim-tab-B83l5xqn.js";import{n as h,t as g}from"./detail-section-HonrIC3a.js";import{n as _,t as ce}from"./values-BluJAOkz.js";import{c as le,l as ue,n as de}from"./grants-Xp6_9WbH.js";import{a as fe,c as v,d as pe,f as me,i as y,l as he,m as ge,n as _e,o as b,p as ve,r as ye,s as be,t as x,u as S}from"./request-fixtures-aMIaX_-K.js";import{n as C,t as w}from"./status-chip-Dlop9RuY.js";import{n as T,r as E}from"./component-card-Wprwd0a9.js";import{r as D,t as O}from"./grants-tab-FPOwcazY.js";function k({detail:e,unread:t,now:n}){let r=e=>t.includes(e),i=r(`PluginRegistration`),a=r(`Grant`),o=r(`PluginSection`),s=e.grants.length>0||a||o;return(0,R.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[e.summary.length>0&&(0,R.jsxs)(g,{title:`Summary`,region:!0,children:[(0,R.jsx)(A,{rows:e.summary}),e.summaryTime&&(0,R.jsxs)(`p`,{className:`text-xs text-muted`,children:[`Updated `,d(e.summaryTime,n),` ago`]})]}),e.connected&&(0,R.jsx)(g,{title:`Connected`,region:!0,children:(0,R.jsx)(M,{refs:e.connected,unread:t})}),e.connectors&&(0,R.jsx)(g,{title:`Principal`,region:!0,children:(0,R.jsxs)(p,{children:[(0,R.jsx)(m,{label:`Principal`,children:(0,R.jsx)(`span`,{className:`font-mono text-xs break-all`,children:e.connectors.principal??`Not written yet`})}),(0,R.jsx)(m,{label:`ServiceAccount`,children:(0,R.jsx)(`span`,{className:`font-mono text-xs break-all`,children:e.connectors.serviceAccount??`Not set`})})]})}),s&&(0,R.jsxs)(g,{title:`Grants`,note:e.clusterGrants?`Every grant of the cluster`:void 0,region:!0,children:[(a||o)&&(0,R.jsx)(u,{kinds:[...a?[`Grant`]:[],...o?[`PluginSection`]:[]]}),i&&e.grants.length>0&&(0,R.jsx)(u,{kinds:[`PluginRegistration`]}),(0,R.jsx)(N,{grants:e.grants.filter(e=>i||!e.templateGone),withRequest:e.clusterGrants,unread:a,templatesUnread:i,now:n}),!i&&e.grants.some(e=>e.templateGone)&&(0,R.jsxs)(`div`,{className:`flex flex-col gap-2 border-t border-border pt-3`,children:[(0,R.jsx)(`p`,{className:`text-sm font-medium text-text`,children:`Grants of a template the registration no longer has`}),(0,R.jsx)(`p`,{className:`text-sm text-text-2`,children:`The role keeps the statement of each of these Grants until the controller of this registration deletes that Grant.`}),(0,R.jsx)(N,{grants:e.grants.filter(e=>e.templateGone),withRequest:e.clusterGrants,unread:a,templatesUnread:!1,now:n})]})]}),e.outputs.length>0&&(0,R.jsx)(g,{title:`Outputs`,region:!0,children:(0,R.jsx)(Se,{detail:e,unread:o||r(`CamundaClusterClaim`)})}),(e.inputs.length>0||i)&&(0,R.jsxs)(g,{title:`Inputs`,region:!0,children:[i&&(0,R.jsx)(u,{kinds:[`PluginRegistration`]}),e.inputs.length>0&&(0,R.jsx)(Ce,{detail:e})]}),(e.usedBy.length>0||i)&&(0,R.jsxs)(g,{title:`Used by`,region:!0,children:[i&&(0,R.jsx)(u,{kinds:[`PluginRegistration`]}),e.usedBy.length>0&&(0,R.jsx)(`ul`,{className:`flex flex-wrap gap-1.5`,children:e.usedBy.map(e=>(0,R.jsx)(`li`,{className:`rounded border border-border px-2 py-0.5 text-sm text-text`,children:e},e))})]}),e.links.length>0&&(0,R.jsx)(g,{title:`Links`,region:!0,children:(0,R.jsx)(`ul`,{className:`flex flex-col gap-2`,children:e.links.map((e,t)=>(0,R.jsx)(`li`,{className:`text-sm`,children:`error`in e?(0,R.jsx)(L,{row:e}):(0,R.jsx)(`a`,{href:e.url,target:`_blank`,rel:`noreferrer`,className:`relative font-medium text-text underline underline-offset-4 hit-area hover:text-accent`,children:e.name})},t))})}),e.resources.length>0&&(0,R.jsx)(g,{title:`Resources`,region:!0,children:(0,R.jsx)(we,{detail:e,claimUnread:r(`CamundaClusterClaim`)})})]})}function A({rows:e}){let t=e.filter(e=>!_(e));return(0,R.jsxs)(R.Fragment,{children:[e.filter(_).map((e,t)=>(0,R.jsx)(`p`,{className:`text-sm text-bad`,children:e.error},t)),t.length>0&&(0,R.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-4 gap-y-2 text-sm`,children:t.map((e,t)=>(0,R.jsx)(j,{row:e},t))})]})}function j({row:e}){let t=e.severity===`Info`?void 0:e.severity,r=t===`Error`?`text-bad`:t===`Warning`?`text-warn`:``;return(0,R.jsxs)(R.Fragment,{children:[(0,R.jsx)(`dt`,{className:`text-muted`,children:e.label}),(0,R.jsxs)(`dd`,{className:n(`min-w-0 break-words`,r),children:[t&&(0,R.jsx)(T,{severity:t}),(0,R.jsx)(`span`,{className:n(`text-text`,r),children:e.value})]})]})}function M({refs:e,unread:t}){return(0,R.jsxs)(`table`,{className:`w-full text-sm`,children:[(0,R.jsx)(c,{children:(0,R.jsxs)(l,{children:[(0,R.jsx)(F,{children:`Kind`}),(0,R.jsx)(F,{children:`Name`}),(0,R.jsx)(F,{children:`Ready`})]})}),(0,R.jsx)(s,{children:e.map(e=>(0,R.jsxs)(l,{children:[(0,R.jsx)(P,{children:e.kind}),(0,R.jsx)(P,{mono:!0,children:e.name}),(0,R.jsx)(P,{children:t.includes(e.kind)?(0,R.jsx)(ie,{type:`Ready`,unknown:!0}):e.found?(0,R.jsx)(w,{condition:e.ready}):(0,R.jsx)(`span`,{className:`text-warn`,children:`Not in the namespace`})})]},`${e.kind}/${e.name}`))})]})}function N({grants:e,withRequest:t,unread:n,templatesUnread:r,now:i}){return e.length===0?null:(0,R.jsx)(`ul`,{className:`flex flex-col divide-y divide-border`,children:e.map(e=>(0,R.jsxs)(`li`,{className:`flex min-w-0 flex-col gap-2 py-3 first:pt-0 last:pb-0`,children:[(0,R.jsx)(`span`,{className:`font-mono text-sm break-all text-text`,children:e.name}),(0,R.jsxs)(p,{children:[(0,R.jsx)(m,{label:`Template`,children:e.template||`-`}),t&&(0,R.jsx)(m,{label:`Request`,children:(0,R.jsx)(`span`,{className:`font-mono break-all`,children:e.request??`-`})})]}),!n&&!e.error&&(0,R.jsx)(xe,{grant:e,templatesUnread:r}),e.errors.map(e=>(0,R.jsx)(`p`,{className:`text-sm text-bad`,children:e},e)),e.statement!==void 0&&(0,R.jsx)(`pre`,{"aria-label":`Statement of ${e.name}`,tabIndex:0,className:`overflow-x-auto rounded-md bg-panel-2 p-3 text-xs outline-none focus-visible:ring-3 focus-visible:ring-ring/50`,children:(0,R.jsx)(`code`,{children:e.statement})}),n?(0,R.jsx)(O,{}):!e.error&&(0,R.jsx)(re,{label:`Conditions of ${e.name}`,conditions:e.conditions,now:i})]},e.name))})}function xe({grant:e,templatesUnread:t}){switch(e.state){case`oldTemplate`:return e.templateGone||t?null:(0,R.jsx)(`p`,{className:`text-sm text-text-2`,children:de.TemplateChanged});case`refused`:{let t=ue(e);return t?(0,R.jsx)(`p`,{className:`text-sm text-warn`,children:t}):null}case`waiting`:return(0,R.jsx)(`p`,{className:`text-sm text-muted`,children:`Not applied yet.`});default:return null}}function Se({detail:e,unread:t}){return(0,R.jsxs)(`table`,{className:`w-full text-sm`,children:[(0,R.jsx)(c,{children:(0,R.jsxs)(l,{children:[(0,R.jsx)(F,{children:`Output`}),(0,R.jsx)(F,{children:`Value`}),(0,R.jsx)(F,{children:`Fills`})]})}),(0,R.jsx)(s,{children:e.outputs.map((e,n)=>`error`in e?(0,R.jsx)(I,{row:e,span:3},n):(0,R.jsxs)(l,{children:[(0,R.jsx)(P,{mono:!0,children:e.key}),(0,R.jsx)(P,{mono:!0,children:e.value}),(0,R.jsx)(P,{children:t?`Not known`:e.fills.length>0?e.fills.join(`, `):`No field`})]},e.key))})]})}function Ce({detail:e}){return(0,R.jsxs)(`table`,{className:`w-full text-sm`,children:[(0,R.jsx)(c,{children:(0,R.jsxs)(l,{children:[(0,R.jsx)(F,{children:`Input`}),(0,R.jsx)(F,{children:`From`}),(0,R.jsx)(F,{children:`Value`})]})}),(0,R.jsx)(s,{children:e.inputs.map(e=>(0,R.jsxs)(l,{children:[(0,R.jsx)(P,{mono:!0,children:e.name}),(0,R.jsx)(P,{children:`${e.sectionLabel} / ${e.output}`}),(0,R.jsx)(P,{mono:e.value!==void 0,children:e.selected?e.value??`Waiting`:`The claim selects no option for ${e.sectionLabel}.`})]},e.name))})]})}function we({detail:e,claimUnread:t}){let n=t&&!e.deleting,r=t=>n?`Not known`:e.controlled?t??`-`:`Not controlled by this claim`;return(0,R.jsxs)(R.Fragment,{children:[(0,R.jsx)(`p`,{className:`text-xs text-muted`,children:n?`Not known: the page has no current list of CamundaClusterClaim, so it cannot say what happens to each resource when the claim is deleted.`:e.controlled?`When the claim is deleted: ${oe[e.policy]}.`:`The claim does not control this request, so its deletion does not decide what happens to these resources.`}),(0,R.jsxs)(`table`,{className:`w-full text-sm`,children:[(0,R.jsx)(c,{children:(0,R.jsxs)(l,{children:[(0,R.jsx)(F,{children:`Kind`}),(0,R.jsx)(F,{children:`Name`}),(0,R.jsx)(F,{children:`ID`}),(0,R.jsx)(F,{children:`Location`}),(0,R.jsx)(F,{children:`Role`}),(0,R.jsx)(F,{children:`After the claim is deleted`})]})}),(0,R.jsx)(s,{children:e.resources.map((e,t)=>`error`in e?(0,R.jsx)(I,{row:e,span:6},t):(0,R.jsxs)(l,{children:[(0,R.jsx)(P,{children:e.kind}),(0,R.jsx)(P,{mono:!0,children:e.name}),(0,R.jsx)(P,{mono:!0,children:e.id??`-`}),(0,R.jsx)(P,{children:e.location??`-`}),(0,R.jsx)(P,{children:e.role??`-`}),(0,R.jsx)(P,{children:r(e.fate)})]},`${e.kind}/${e.name}`))})]})]})}function P({mono:e=!1,children:t}){return(0,R.jsx)(o,{className:n(`align-top whitespace-normal wrap-anywhere`,e&&`font-mono text-xs`),children:t})}function F({children:e}){return(0,R.jsx)(i,{className:`h-auto py-2 align-bottom whitespace-normal`,children:e})}function I({row:e,span:t}){return(0,R.jsx)(l,{children:(0,R.jsx)(o,{colSpan:t,className:`whitespace-normal`,children:(0,R.jsx)(L,{row:e})})})}function L({row:e}){return(0,R.jsx)(`span`,{className:`text-sm text-bad`,children:e.error})}var R;function z(){return(z=e((()=>{R=t(),te(),ne(),ae(),C(),f(),le(),D(),ce(),a(),r(),se(),E(),h(),ee(),k.__docgenInfo={description:`RequestOverview shows detail below the conditions of a request: one box
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
}`,signature:{properties:[{key:`kind`,value:{name:`union`,raw:`"SecondaryStorageConfig" | "ObjectStorageConfig" | "DatabaseConfig"`,elements:[{name:`literal`,value:`"SecondaryStorageConfig"`},{name:`literal`,value:`"ObjectStorageConfig"`},{name:`literal`,value:`"DatabaseConfig"`}],required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`found`,value:{name:`boolean`,required:!0}},{key:`ready`,value:{name:`ConditionRow`,required:!1}}]}}],raw:`ConnectedRef[]`,required:!1}},{key:`connectors`,value:{name:`signature`,type:`object`,raw:`{ principal?: string; serviceAccount?: string }`,signature:{properties:[{key:`principal`,value:{name:`string`,required:!1}},{key:`serviceAccount`,value:{name:`string`,required:!1}}]},required:!1}},{key:`usedBy`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}}]}},description:``},unread:{required:!0,tsType:{name:`unknown`},description:``},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}function B(e,t={}){let n=_e(S,t.deletionPolicy),r=he.map(e=>t.request&&e.spec.section===t.request.spec.section?t.request:e),i=ye(n,r);return ge({component:i.find(t=>t.section===e),claim:n,cluster:`orders`,sections:x,registrations:be,components:i,grants:t.grants??[],contracts:{SecondaryStorageConfig:[fe(`orders-storage`)]}})}var V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{z(),pe(),ve(),V=[y(`Applied`,`True`,`Healthy`),y(`MatchesTemplate`,`True`,`Healthy`),y(`Ready`,`True`,`Healthy`)],H={title:`cluster/RequestOverview`,component:k,args:{unread:[],now:me}},U={name:`Cloud identity with the grants of the cluster`,args:{detail:B(`cloud-identity`,{grants:[b(`orders-read`,`backup-storage`,V),b(`orders-key`,`encryption-key`)]})}},W={name:`Backup storage with a rejected grant`,args:{detail:B(`backup-storage`,{request:v(`backup-storage`,{summary:[{label:`Bucket`,value:`orders-backups`}],conditions:[y(`OutputsReady`,`True`,`Healthy`),y(`Ready`,`False`,`GrantRejected`,`Grant orders-read has Ready=False`)]}),grants:[b(`orders-read`,`backup-storage`,[y(`Applied`,`False`,`OverCeiling`,`s3:DeleteObject is not in the ceiling of the identity`),y(`MatchesTemplate`,`True`,`Healthy`),y(`Ready`,`False`,`OverCeiling`,`s3:DeleteObject is not in the ceiling of the identity`)])]})}},G={name:`A grant applied with the old template`,args:{detail:B(`backup-storage`,{grants:[b(`orders-read`,`backup-storage`,[y(`Applied`,`True`,`Healthy`),y(`MatchesTemplate`,`False`,`TemplateChanged`,`The fragment no longer matches its template`),y(`Ready`,`False`,`TemplateChanged`,`The fragment no longer matches its template`)])]})}},K={name:`Encryption key used by two sections`,args:{detail:B(`encryption-key`)}},q={name:`Connectors permissions`,args:{detail:B(`connectors-permissions`)}},J={name:`Access waiting for endpoints`,args:{detail:B(`access`,{request:v(`access`,{summary:[{label:`Host`,value:`Waiting for the Gateway`,severity:`Warning`}],conditions:[y(`OutputsReady`,`False`,`WaitingForEndpoints`),y(`Ready`,`False`,`WaitingForEndpoints`)]})})}},Y={name:`Existing secondary storage, connected`,args:{detail:B(`secondary-storage`)}},X={name:`Resources kept and deleted`,args:{detail:B(`backup-storage`,{deletionPolicy:`Orphan`})}},Z={name:`Grants and the claim not read`,args:{detail:B(`backup-storage`,{grants:[b(`orders-read`,`backup-storage`,V)]}),unread:[`Grant`,`CamundaClusterClaim`]}},Q=[`CloudIdentityWithGrants`,`BackupStorageWithARejectedGrant`,`GrantWithAnOldTemplate`,`EncryptionKeyUsedByTwoSections`,`ConnectorsPermissions`,`AccessWaitingForEndpoints`,`ExistingSecondaryStorageConnected`,`ResourcesKeptAndDeleted`,`NotRead`],U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
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