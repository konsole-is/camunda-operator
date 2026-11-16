import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{b as n}from"./labels-CoLYcJ9C.js";import{C as r,b as i}from"./release-DN3G17Sp.js";import{h as a,m as o}from"./policy-DG5CgYLS.js";import{n as s,t as c}from"./link-DOLMGIR2.js";import{a as l,i as u,n as d,r as f}from"./claim-tab-DyENXb9O.js";import{a as p,c as m,d as h,i as g,l as _,n as v,o as y,t as b,u as x}from"./values-9f-8xfft.js";import{n as S,t as C}from"./condition-badge-B9hj4lr7.js";import{r as w,s as T,t as E}from"./groups-tgzbe4Oz.js";import{i as D,r as O}from"./grants-D7KpJfci.js";function k(e,t,n){let r=T(t),a=r.includes(`PluginRequest`),o=r.includes(`Grant`),s=r.includes(`Namespace`)||r.includes(`Environment`);return t.requests.filter(t=>t.spec.registration===e).sort((e,t)=>i(e.metadata.namespace??``,t.metadata.namespace??``)||i(e.metadata.name??``,t.metadata.name??``)).map(e=>{let r=e.metadata.namespace??``,i=e.metadata.name??``,c=b(e.status?.conditions),l=`conditions`in c?c.conditions:[],u=l.find(e=>e.type===`Ready`),d=l.find(e=>e.type===`OutputsReady`),f=n.filter(e=>e.metadata.namespace===r&&e.metadata.labels?.[`plugins.camunda.io/request`]===i).map(O),[p]=y(e.status?.summary,j,`Summary item`,`summary items`);return{namespace:r,name:i,claim:e.spec.claimRef,target:{kind:e.spec.target.kind,name:e.spec.target.name},...!s&&{environment:A(r,t)},...a&&{healthUnknown:!0},...!a&&d&&{outputsReady:d},...!a&&u&&{ready:u},...`error`in c&&{error:c.error},...!o&&{refusedGrants:u?.reason===`GrantRejected`?f.filter(e=>e.state===`refused`).map(e=>e.name):[],grants:f},...p&&{headline:p},resources:y(e.status?.resources,M,`Resource`,`resources`),deletionPolicy:e.spec.deletionPolicy}})}function A(e,t){let n=t.namespaces.find(t=>t.metadata?.name===e),r=n?o(n,t.environments).environment:void 0;return r?r.spec.displayName||(r.metadata.name??``):E}function j(e){return g(e)&&p(e.label)&&p(e.value)&&N.includes(e.severity)}function M(e){return g(e)&&p(e.kind)&&p(e.name)&&(e.onOrphan===`Kept`||e.onOrphan===`Deleted`)&&p(e.id,!0)&&p(e.location,!0)&&p(e.role,!0)}var N;function P(){return(P=e((()=>{n(),r(),a(),D(),w(),v(),N=[void 0,`Info`,`Warning`,`Error`]})))()}function F({rows:e,unread:t}){let n=t.includes(`PluginRequest`);if(e.length===0)return n?(0,R.jsx)(_,{kinds:[`PluginRequest`]}):(0,R.jsx)(`p`,{className:`text-sm text-muted`,children:`No claim uses this option.`});let r=[`Namespace`,`Environment`].filter(e=>t.includes(e));return(0,R.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[n&&(0,R.jsx)(`p`,{className:`text-sm text-muted`,children:`The list can miss requests: PluginRequest is not read.`}),(0,R.jsx)(`div`,{className:`grid grid-cols-1 gap-4 xl:grid-cols-2`,children:e.map(e=>(0,R.jsx)(d,{label:`${e.namespace}/${e.name}`,children:(0,R.jsxs)(u,{children:[(0,R.jsx)(I,{row:e}),(0,R.jsx)(f,{label:`Environment`,children:e.environment??(0,R.jsx)(_,{kinds:r})}),e.error?(0,R.jsx)(f,{label:`Conditions`,children:(0,R.jsx)(x,{text:e.error})}):(0,R.jsxs)(R.Fragment,{children:[(0,R.jsx)(f,{label:`Outputs`,children:(0,R.jsx)(L,{type:`OutputsReady`,condition:e.outputsReady,unknown:e.healthUnknown})}),(0,R.jsxs)(f,{label:`Ready`,children:[(0,R.jsx)(L,{type:`Ready`,condition:e.ready,unknown:e.healthUnknown}),e.ready?.reason===`GrantRejected`&&(e.refusedGrants?e.refusedGrants.length>0&&(0,R.jsx)(`span`,{className:`block text-xs text-warn`,children:`Refused Grants: ${e.refusedGrants.join(`, `)}`}):(0,R.jsx)(_,{kinds:[`Grant`]}))]})]}),(0,R.jsx)(f,{label:`Headline`,children:e.headline?`error`in e.headline?(0,R.jsx)(x,{text:e.headline.error}):`${e.headline.label}: ${e.headline.value}`:(0,R.jsx)(`span`,{className:`text-muted`,children:`None`})}),(0,R.jsx)(f,{label:`Resources`,children:e.resources.length===0?(0,R.jsx)(`span`,{className:`text-muted`,children:`None reported`}):(0,R.jsx)(`ul`,{className:`flex flex-col gap-1.5`,children:e.resources.map((e,t)=>`error`in e?(0,R.jsx)(`li`,{children:(0,R.jsx)(x,{text:e.error})},t):(0,R.jsxs)(`li`,{children:[(0,R.jsx)(`span`,{className:`block font-mono break-all`,children:`${e.kind} ${e.name}`}),(e.role||e.location)&&(0,R.jsx)(`span`,{className:z,children:[e.role,e.location].filter(Boolean).join(`, `)}),(0,R.jsx)(`span`,{className:z,children:`On Orphan: ${e.onOrphan}`})]},`${e.kind} ${e.name} ${t}`))})}),(0,R.jsx)(f,{label:`Grants`,children:e.grants?e.grants.length===0?(0,R.jsx)(`span`,{className:`text-muted`,children:`None`}):(0,R.jsx)(`ul`,{className:`flex flex-col gap-1.5`,children:e.grants.map(e=>(0,R.jsxs)(`li`,{className:`flex flex-col gap-0.5`,children:[(0,R.jsx)(`span`,{className:`font-mono break-all`,children:e.name}),e.error?(0,R.jsx)(x,{text:e.error}):(0,R.jsx)(C,{type:`Ready`,condition:e.ready})]},e.name))}):(0,R.jsx)(_,{kinds:[`Grant`]})}),(0,R.jsx)(f,{label:`Deletion policy`,children:e.deletionPolicy})]})},`${e.namespace}/${e.name}`))})]})}function I({row:e}){let t=e.claim.kind===`CamundaManagementClusterClaim`,n=`/fleet/${t?`planes`:`clusters`}/${encodeURIComponent(e.namespace)}/${encodeURIComponent(e.target.name)}`;return(0,R.jsx)(f,{label:t?`Management plane`:`Cluster`,children:(0,R.jsx)(s,{href:n,className:`${m} font-mono`,children:e.claim.name})})}function L({type:e,condition:t,unknown:n}){return(0,R.jsxs)(R.Fragment,{children:[(0,R.jsx)(C,{type:e,condition:t,unknown:n}),!n&&t?.status!==`True`&&t?.message&&(0,R.jsx)(`span`,{className:`mt-1 block text-xs break-words text-text-2`,children:t.message})]})}var R,z;function B(){return(B=e((()=>{R=t(),c(),l(),S(),h(),z=`block text-xs text-muted`,F.__docgenInfo={description:`UsedByTab shows each request of rows in its own panel: its claim, its
environment, OutputsReady and Ready, its headline, its resources, its
Grants, and its deletion policy. unread holds the kinds that are not
read: with PluginRequest in it, the tab never says that no claim uses the
option.`,methods:[],displayName:`UsedByTab`,props:{rows:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  namespace: string;
  name: string;
  claim: {
    kind: "CamundaClusterClaim" | "CamundaManagementClusterClaim";
    name: string;
  };
  target: { kind: string; name: string };
  /**
   * environment is the display name, else the name, of the Environment of
   * the namespace, or "No environment". It is unset while Namespace or
   * Environment is not read.
   */
  environment?: string;
  /** healthUnknown is true while PluginRequest is not read. outputsReady and ready are then unset. */
  healthUnknown?: true;
  outputsReady?: ConditionRow;
  ready?: ConditionRow;
  /** error says why the conditions cannot be read. */
  error?: string;
  /**
   * refusedGrants names the refused Grants of the request while Ready has
   * the reason GrantRejected. It is unset while Grant is not read.
   */
  refusedGrants?: string[];
  /** headline is the first item of status.summary. */
  headline?: SummaryItem | Invalid;
  resources: (ResourceRef | Invalid)[];
  /** grants is unset while Grant is not read. */
  grants?: GrantView[];
  deletionPolicy: "Delete" | "Orphan";
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`claim`,value:{name:`signature`,type:`object`,raw:`{
  kind: "CamundaClusterClaim" | "CamundaManagementClusterClaim";
  name: string;
}`,signature:{properties:[{key:`kind`,value:{name:`union`,raw:`"CamundaClusterClaim" | "CamundaManagementClusterClaim"`,elements:[{name:`literal`,value:`"CamundaClusterClaim"`},{name:`literal`,value:`"CamundaManagementClusterClaim"`}],required:!0}},{key:`name`,value:{name:`string`,required:!0}}]},required:!0}},{key:`target`,value:{name:`signature`,type:`object`,raw:`{ kind: string; name: string }`,signature:{properties:[{key:`kind`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]},required:!0}},{key:`environment`,value:{name:`string`,required:!1},description:`environment is the display name, else the name, of the Environment of
the namespace, or "No environment". It is unset while Namespace or
Environment is not read.`},{key:`healthUnknown`,value:{name:`literal`,value:`true`,required:!1},description:`healthUnknown is true while PluginRequest is not read. outputsReady and ready are then unset.`},{key:`outputsReady`,value:{name:`ConditionRow`,required:!1}},{key:`ready`,value:{name:`ConditionRow`,required:!1}},{key:`error`,value:{name:`string`,required:!1},description:`error says why the conditions cannot be read.`},{key:`refusedGrants`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!1},description:`refusedGrants names the refused Grants of the request while Ready has
the reason GrantRejected. It is unset while Grant is not read.`},{key:`headline`,value:{name:`union`,raw:`SummaryItem | Invalid`,elements:[{name:`NonNullable[number]`,raw:`NonNullable<
  NonNullable<PluginRequest["status"]>["summary"]
>[number]`},{name:`signature`,type:`object`,raw:`{ error: string }`,signature:{properties:[{key:`error`,value:{name:`string`,required:!0}}]}}],required:!1},description:`headline is the first item of status.summary.`},{key:`resources`,value:{name:`Array`,elements:[{name:`unknown`}],raw:`(ResourceRef | Invalid)[]`,required:!0}},{key:`grants`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  namespace: string;
  name: string;
  cluster?: string;
  /** request is the value of REQUEST_LABEL. */
  request?: string;
  applied?: ConditionRow;
  matchesTemplate?: ConditionRow;
  ready?: ConditionRow;
  state: "ready" | "refused" | "waiting";
  error?: string;
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`cluster`,value:{name:`string`,required:!1}},{key:`request`,value:{name:`string`,required:!1},description:`request is the value of REQUEST_LABEL.`},{key:`applied`,value:{name:`ConditionRow`,required:!1}},{key:`matchesTemplate`,value:{name:`ConditionRow`,required:!1}},{key:`ready`,value:{name:`ConditionRow`,required:!1}},{key:`state`,value:{name:`union`,raw:`"ready" | "refused" | "waiting"`,elements:[{name:`literal`,value:`"ready"`},{name:`literal`,value:`"refused"`},{name:`literal`,value:`"waiting"`}],required:!0}},{key:`error`,value:{name:`string`,required:!1}}]}}],raw:`GrantView[]`,required:!1},description:`grants is unset while Grant is not read.`},{key:`deletionPolicy`,value:{name:`union`,raw:`"Delete" | "Orphan"`,elements:[{name:`literal`,value:`"Delete"`},{name:`literal`,value:`"Orphan"`}],required:!0}}]}}],raw:`UsedByRow[]`},description:``},unread:{required:!0,tsType:{name:`unknown`},description:``}}}})))()}export{k as i,B as n,P as r,F as t};