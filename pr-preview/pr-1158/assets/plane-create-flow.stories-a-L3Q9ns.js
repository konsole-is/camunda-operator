import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{t as r}from"./hooks-Bys_naRE.js";import{a as i,b as a,c as o,t as s,x as c}from"./labels-Be1Q9KU6.js";import{n as l,t as u}from"./circle-exclamation-point-jpEoxFxv.js";import{s as d,t as f}from"./policy-C1QquhaH.js";import{i as p,t as m}from"./drafts-w24ODmP9.js";import{C as ee,S as te,a as ne,l as re,s as ie,x as ae}from"./draft-DUaPspop.js";import{A as oe,D as se,F as ce,I as le,J as ue,L as de,M as h,P as fe,Q as pe,R as me,T as he,Y as ge,Z as _e,a as ve,b as ye,f as be,g as xe,h as Se,j as Ce,k as we,m as Te,o as Ee,p as De,w as Oe,y as ke,z as Ae}from"./session-BmEcT8yX.js";import{a as je,n as Me,r as Ne}from"./urls-O-B1cP1Y.js";import{n as Pe,t as Fe}from"./locked-field-BEGQxrUK.js";import{n as Ie,t as Le}from"./input-Bo754NJr.js";import{n as Re,r as ze,t as Be}from"./native-select-dGlSSadv.js";import{r as Ve,t as He}from"./button-3wYKWpp7.js";import{n as Ue}from"./secret-ref-picker-Bo_OV4sT.js";import{a as We,n as Ge}from"./deletion-choice-BT3xTdj-.js";import{n as Ke,t as qe}from"./diff-BLIgMNCc.js";import{n as Je,t as Ye}from"./review-panel-DTXmOI6J.js";import{n as Xe}from"./toast-yKngc27L.js";import{i as Ze,r as Qe}from"./fixtures-u63i3KTD.js";import{n as $e,t as et}from"./precheck-line-qAgtupO6.js";import{n as tt,r as nt,t as rt}from"./wizard-stepper-DWyWQbOL.js";import{p as it,t as at}from"./usage-D37gqmWE.js";import{n as ot,t as st}from"./namespace-picker-CiNnmE75.js";import{n as ct,t as lt}from"./claim-name-field-DCjBZELw.js";import{n as ut,t as dt}from"./environment-cards-dUMSb5Iu.js";import{n as ft,t as pt}from"./use-create-flow-CllyieDU.js";import{n as mt,t as ht}from"./progress-rows-DTjqwXT3.js";import{n as gt,t as _t}from"./use-create-review-BfdfBgJB.js";import{n as vt,t as yt}from"./section-cards-BqD5gouv.js";import{i as bt,t as xt}from"./plane-C-R8gstT.js";import{n as St}from"./planes-table-D29WZ_I_.js";import{n as Ct,t as wt}from"./app-shell-CrPNklGY.js";import{a as Tt,i as Et,l as Dt,n as Ot,o as kt,r as At,s as jt}from"./served-step-dQ6bKZVd.js";function Mt(){return(Mt=e((()=>{t(),Ue(),r()})))()}function Nt({draft:e,set:t,ctx:n,namespaces:r,clusters:i,secrets:a}){let o=(0,Pt.useId)(),s=n.environments.find(t=>t.metadata.name===e.environment),c=re(e.namespace,s);if((0,Pt.useEffect)(()=>{c!==e.namespace&&t([`namespace`],c)}),n.environments.length===0)return(0,g.jsx)(`p`,{className:`text-sm text-muted`,children:De});let l=s&&(s.spec.displayName||s.metadata.name),u=Oe(e,n),f=n.platformConfigs.find(e=>e.metadata.name===u),p=u!==void 0&&u===s?.spec.platformConfigRef,m=n.platformConfigs.map(e=>e.metadata.name??``).sort();return(0,g.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-8`,children:[(0,g.jsx)(dt,{environments:n.environments,counts:it(n.environments,r,i),value:e.environment,onChange:n=>{n!==e.environment&&(t([`environment`],n),t([`namespace`],void 0))}}),(0,g.jsx)(lt,{value:e.name,errors:ae(e.name,n.takenNames),hosts:[],hostsLabel:`The plane gets these hosts:`,onChange:e=>t([`name`],e)}),s?(0,g.jsx)(st,{namespaces:r.map(e=>({name:e.metadata?.name??``,environment:d(e,n.environments).environment?.metadata.name})),environment:ie(s),defaultValue:e.namespace,onChange:(e,n)=>t([`namespace`],n.length?void 0:e)},e.environment??``):(0,g.jsxs)(`section`,{className:`flex min-w-0 flex-col gap-3`,children:[(0,g.jsx)(`h3`,{className:`font-heading text-base font-semibold`,children:`Namespace`}),(0,g.jsx)(`p`,{className:`text-sm text-muted`,children:`Choose an environment first.`})]}),l&&(0,g.jsx)(`p`,{className:`border-l-2 border-accent pl-3 text-sm text-text-2`,children:e.namespace?`Policy from the ${l} environment (namespace ${e.namespace.name}).`:`Policy from the ${l} environment.`}),(0,g.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1`,children:[(0,g.jsx)(`label`,{htmlFor:`${o}-config`,className:`font-heading text-base font-semibold`,children:`Platform config`}),(0,g.jsxs)(Be,{id:`${o}-config`,value:u??``,"aria-describedby":p?`${o}-config-note`:void 0,onChange:e=>t([`platformConfig`],e.target.value||void 0),children:[!f&&(0,g.jsx)(Re,{value:u??``,children:`Pick a platform config`}),m.map(e=>(0,g.jsx)(Re,{value:e,children:e},e))]}),p&&l&&(0,g.jsx)(`p`,{id:`${o}-config-note`,className:`text-sm text-muted`,children:`Default from the ${l} environment.`})]}),(0,g.jsx)(Tt,{value:e.mode,platformConfig:f,onChange:e=>t([`mode`],e)}),e.mode===`externalKeycloak`&&(0,g.jsx)(Et,{value:e.externalKeycloak,secrets:a,namespaceIsNew:e.namespace?.kind===`new`,onChange:e=>t([`externalKeycloak`],e)})]})}var g,Pt;function Ft(){return(Ft=e((()=>{g=n(),Pt=t(),ct(),ut(),ot(),ze(),te(),ne(),xe(),f(),at(),kt(),Nt.__docgenInfo={description:`PlaneBasicsStep is step 1 of the plane create flow: the environment, the
name, the namespace, the platform config, and where users sign in.
Without any Environment it shows only that a plane needs one. A new
environment clears the namespace. secrets are the Secrets of the chosen
namespace, none for a new one.`,methods:[],displayName:`PlaneBasicsStep`,props:{draft:{required:!0,tsType:{name:`PlaneDraft`},description:``},set:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(path: DraftPath, value: unknown) => void`,signature:{arguments:[{type:{name:`DraftPath`},name:`path`},{type:{name:`unknown`},name:`value`}],return:{name:`void`}}},description:``},ctx:{required:!0,tsType:{name:`PlaneContext`},description:``},namespaces:{required:!0,tsType:{name:`unknown`},description:``},clusters:{required:!0,tsType:{name:`unknown`},description:``},secrets:{required:!0,tsType:{name:`SecretRefs`},description:``}}}})))()}function It(e,t){let n=e?.status,r=e&&Ee(e),i={id:`environment`,label:`Environment and policy`,...n?.environment&&{detail:n.environment},checks:[{label:`Policy`,condition:r?Bt(r):n?.environment?{status:`True`}:void 0}]},a=Rt(t,`managementDatabases`),o=Rt(t,`externalAccess`),s=(e?.spec.plugins??[]).filter(e=>e.section!==a&&e.section!==o),c=e=>s.filter(t=>t.section===e).length,l=(e?.spec.plugins??[]).some(e=>e.section===o),u=n?.conditions?.find(e=>e.type===Vt);return[i,Lt(`database`,`Databases`,a,e),...l?[Lt(`access`,`Access`,o,e)]:[],...s.map(({section:n,registration:r})=>{let i=t.find(e=>e.metadata.name===n)?.spec.label??n;return Lt(`section:${n}/${r}`,c(n)>1?`${i}: ${r}`:i,n,e,r)}),{id:`managementCluster`,label:`Management cluster`,checks:[{label:`Ready`,condition:u&&Bt(u)}]}]}function Lt(e,t,n,r,i){let a=r?.status?.requests?.find(e=>e.section===n&&(i===void 0||e.registration===i));return{id:e,label:t,...a&&{detail:a.name},checks:[{label:`Outputs`,condition:zt(a?.outputsReady)},{label:`Ready`,condition:zt(a?.ready)}]}}function Rt(e,t){return e.find(e=>e.spec.slot===t)?.metadata.name}function zt(e){return e===`True`||e===`False`||e===`Unknown`?{status:e}:void 0}function Bt(e){return{status:e.status,...e.reason!==void 0&&{reason:e.reason},...e.message!==void 0&&{message:e.message}}}var Vt;function Ht(){return(Ht=e((()=>{ve(),Vt=`ManagementClusterReady`})))()}function Ut({name:e,claim:t,sections:n}){let r=(0,Wt.useId)(),i=t?.status?.conditions?.find(e=>e.type===Gt);return(0,_.jsxs)(`section`,{"aria-labelledby":r,className:`flex max-w-3xl min-w-0 flex-col gap-4`,children:[(0,_.jsx)(`h2`,{id:r,className:`font-heading text-base font-semibold`,children:`Creating the management plane ${e}`}),(0,_.jsx)(ht,{ready:i,rows:It(t,n)}),t&&(0,_.jsx)(et,{claim:t})]})}var _,Wt,Gt;function Kt(){return(Kt=e((()=>{_=n(),Wt=t(),$e(),mt(),r(),Ht(),Gt=`Ready`,Ut.__docgenInfo={description:`PlaneProgressView shows the progress of the plane claim named name: its
Ready condition, planeProgressRows, and the refusal of the claim
controller.`,methods:[],displayName:`PlaneProgressView`,props:{name:{required:!0,tsType:{name:`string`},description:``},claim:{required:!1,tsType:{name:`CamundaManagementClusterClaim`},description:``},sections:{required:!0,tsType:{name:`unknown`},description:``}}}})))()}function qt({change:e,claim:t,draft:n,ctx:r,mode:i,preview:a,create:o,onWritten:s,blocked:c=!1,onBack:l}){let{refusals:d,pending:f,confirm:p}=gt({change:e,preview:a,create:o,onWritten:s,blocked:c}),m=nt();return(0,v.jsx)(Ye,{diffLabel:`The claim`,lines:Ke(void 0,t),effects:Jt(n,r,t),caution:n.console?Yt:void 0,notices:d.length>0&&(0,v.jsxs)(`div`,{role:`alert`,className:`flex gap-3 rounded-md border border-bad/40 bg-bad/10 p-3 text-sm text-text`,children:[(0,v.jsx)(u,{"aria-hidden":!0,className:`mt-0.5 size-4 shrink-0 text-bad`}),(0,v.jsx)(`ul`,{className:`flex flex-col gap-1`,children:d.map((e,t)=>(0,v.jsx)(`li`,{children:e},t))})]}),mode:i,confirmLabel:`Create`,pending:f,describedBy:m,onConfirm:p,onCancel:l})}function Jt(e,t,n){let{name:r,namespace:i}=n.metadata,a=e=>t.registrations.find(t=>t.metadata.name===e)?.spec.label??e,o=Ce(t.sections)?.metadata.name,s=de(e).find(e=>e.section===o),c=we(e);return[...e.namespace?.kind===`new`?[`Creates the namespace ${i}.`]:[],`Creates the claim ${r}.`,...s?[`Requests the databases: ${a(s.registration)}.`]:[],...c?[`Requests access: ${a(c.registration)}.`]:[],`Creates the CamundaManagementCluster ${r}.`,`The plane writes the contract ${oe(e)}.`]}var v,Yt;function Xt(){return(Xt=e((()=>{v=n(),l(),_t(),Je(),tt(),qe(),h(),Yt=`Each cluster that attaches restarts its pods once, because it gets the Console settings.`,qt.__docgenInfo={description:`PlaneReviewStep is step 5 of the plane create flow: the YAML of claim,
what change creates, and the confirm. It previews change once for each
change, and Create stays disabled until that preview is clean and while
a write runs or while blocked. Create sends change to create once and
gives the result to onWritten, also when the write is refused. With
onBack, Back sits beside Create.`,methods:[],displayName:`PlaneReviewStep`,props:{change:{required:!0,tsType:{name:`Change`},description:``},claim:{required:!0,tsType:{name:`PlaneCreateClaim`},description:``},draft:{required:!0,tsType:{name:`PlaneDraft`},description:``},ctx:{required:!0,tsType:{name:`PlaneContext`},description:``},mode:{required:!0,tsType:{name:`WriteMode`},description:``},preview:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}}},description:``},create:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}}},description:``},onWritten:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(result: WriteResult) => void`,signature:{arguments:[{type:{name:`WriteResult`},name:`result`}],return:{name:`void`}}},description:``},blocked:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`false`,computed:!1}},onBack:{required:!1,tsType:{name:`signature`,type:`function`,raw:`() => void`,signature:{arguments:[],return:{name:`void`}}},description:``}}}})))()}function Zt({draft:e,set:t,ctx:n,objects:r,onSectionErrors:i}){let a=(0,$t.useId)(),o=pe(e),s=le(e,n,r),c=je(o),l=fe(e,n),u=t=>e.storage[t.name]?.[0],d=e=>(n,r)=>{t([`storage`,e],n),i(e,r)},f=p(me(e,n)),m=e=>{t([`storage`,e],[]),i(e,[])},ee=s.databases&&u(s.databases),te=[`keycloak`,`identity`,`webModeler`].filter(e=>e===`identity`||o[e]).map(t=>`${e.name}-${se[t]}`);return(0,y.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-8`,children:[f.map(e=>(0,y.jsxs)(`div`,{className:`flex flex-col gap-3 rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text sm:flex-row sm:items-center sm:justify-between`,children:[(0,y.jsx)(`span`,{children:`The section ${e} is no longer offered. Its pick does not apply.`}),(0,y.jsx)(He,{variant:`outline`,size:`sm`,onClick:()=>m(e),children:`Remove the ${e} pick`})]},e)),s.databases&&(0,y.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-3`,children:[(0,y.jsx)(Qt,{view:s.databases,draft:e,onChange:d(s.databases.name)}),ee?.registration===en&&(0,y.jsx)(`p`,{className:`text-sm text-text-2`,children:`Creates one DatabaseServer ${e.name}-db and one Database for each app that runs: ${ue(te)}.`}),ee?.registration===`existing-database-configs`&&(0,y.jsx)(`p`,{className:`text-sm text-text-2`,children:`Uses the DatabaseConfigs that you pick. Each app needs its own.`})]}),(0,y.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-3`,children:[s.access&&(0,y.jsx)(Qt,{view:s.access,draft:e,onChange:d(s.access.name)}),we(e)?l&&(0,y.jsxs)(`div`,{className:`flex flex-col gap-3`,children:[(0,y.jsx)(Fe,{label:`Hostname`,value:(0,y.jsx)(`span`,{className:`font-mono`,children:l.host}),reason:`From the pattern ${l.pattern}.`}),(0,y.jsx)(`ul`,{"aria-label":`URLs`,className:`flex flex-col divide-y divide-divider rounded-md border border-border text-sm`,children:Object.entries(Me(l.host,c)).map(([e,t])=>(0,y.jsxs)(`li`,{className:`flex flex-col gap-0.5 px-3 py-2 sm:flex-row sm:gap-4`,children:[(0,y.jsx)(`span`,{className:`text-text-2 sm:w-48`,children:c.find(t=>t.id===e)?.label}),(0,y.jsx)(`span`,{className:`font-mono break-all text-text`,children:t})]},e))}),(0,y.jsx)(`p`,{className:`text-xs text-muted`,children:`Paths are fixed. Each app serves under its path.`})]}):(0,y.jsx)(`div`,{className:`flex max-w-md flex-col gap-4`,children:c.map(n=>(0,y.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,y.jsx)(`label`,{htmlFor:`${a}-${n.id}`,className:`text-sm font-medium text-text`,children:`${n.label} URL`}),(0,y.jsx)(Le,{id:`${a}-${n.id}`,type:`url`,required:!0,value:e.urls[n.id]??``,placeholder:n.placeholder,autoComplete:`off`,spellCheck:!1,className:`font-mono`,onChange:e=>t([`urls`,n.id],e.target.value)})]},n.id))})]}),s.others?.map(t=>(0,y.jsx)(Qt,{view:t,draft:e,onChange:d(t.name)},t.name)),(0,y.jsx)(Ge,{subject:`management plane`,choice:ce(e,n,r),onChange:e=>t([`deletionPolicy`],e)}),(0,y.jsxs)(`section`,{className:`flex max-w-md min-w-0 flex-col gap-1`,children:[(0,y.jsx)(`label`,{htmlFor:`${a}-contract`,className:`font-heading text-base font-semibold`,children:`Contract name`}),(0,y.jsx)(`p`,{id:`${a}-contract-hint`,className:`text-sm text-muted`,children:`The ManagementAuthConfig that Optimize reads. Empty means the plane name.`}),(0,y.jsx)(Le,{id:`${a}-contract`,value:e.contractName,placeholder:e.name,autoComplete:`off`,spellCheck:!1,"aria-describedby":`${a}-contract-hint`,onChange:e=>t([`contractName`],e.target.value)})]})]})}function Qt({view:e,draft:t,onChange:n}){let r=JSON.stringify(e),i=(0,$t.useMemo)(()=>JSON.parse(r),[r]),a=Object.hasOwn(t.storage,e.name);return(0,y.jsx)(yt,{section:i,defaultValue:a?t.storage[e.name]:void 0,onChange:n})}var y,$t,en;function tn(){return(tn=e((()=>{y=n(),$t=t(),We(),Pe(),vt(),Ve(),Ie(),m(),_e(),h(),Ne(),en=`management-database-server`,Zt.__docgenInfo={description:`StorageAccessStep is step 4 of the plane create flow: the database
option, the access option with the URLs of the running apps, the other
sections of planeSectionViews, the deletion choice of planeDeletion, and
the contract name. A section that the step showed before restores its
selections from draft.storage; any other preselects its default. Each
section reports its selections to draft.storage and its errors to
onSectionErrors. A pick of a section that no PluginSection holds any more
gets a button that empties it and its errors. objects are the source
kinds of builtinOption, in every namespace.`,methods:[],displayName:`StorageAccessStep`,props:{draft:{required:!0,tsType:{name:`PlaneDraft`},description:``},set:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(path: DraftPath, value: unknown) => void`,signature:{arguments:[{type:{name:`DraftPath`},name:`path`},{type:{name:`unknown`},name:`value`}],return:{name:`void`}}},description:``},ctx:{required:!0,tsType:{name:`PlaneContext`},description:``},objects:{required:!0,tsType:{name:`BuiltinObjects`},description:``},onSectionErrors:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(section: string, errors: ValidationError[]) => void`,signature:{arguments:[{type:{name:`string`},name:`section`},{type:{name:`Array`,elements:[{name:`ValidationError`}],raw:`ValidationError[]`},name:`errors`}],return:{name:`void`}}},description:``}}}})))()}function nn(e,t){return{environments:e.environments,platformConfigs:e.platformConfigs,takenNames:t.namespace?.kind===`existing`?ee(t.namespace.name,e.sources.claims,e.sources.clusters):[],namespaces:e.namespaces.map(t=>({name:t.metadata?.name??``,environment:d(t,e.environments).environment?.metadata.name})),releases:e.releases,images:e.images,sections:e.sections,registrations:e.registrations,accessRequests:e.accessRequests,authConfigs:e.authConfigs,planeClaims:on(e.sources.claims)}}function rn(e,t,n=[],r={},i){let a=e=>`The fleet manager could not read the ${e.kind} objects: ${e.message}`,o=n.filter(e=>sn.includes(e.kind)),s=n.filter(e=>!sn.includes(e.kind)),c=e=>i&&(e.namespace?.kind===`existing`?i:[]);return cn.map(n=>({...n,check:i=>{let l=nn(e,i);switch(n.id){case`basics`:return[...s.map(a),...Te(i,l,c(i))];case`served`:return he(i,l,t);case`components`:return Se(i,l,c(i));case`storage`:{let t=le(i,l,e.objects);return[...o.map(a),...Ae(i,l,r,t,e.objects)]}default:return[]}}}))}function an({current:e,draft:t,set:n,models:r,secrets:i,onSelectorErrors:a,onSectionErrors:o,review:s}){let c=nn(r,t);return e===`review`&&s?(0,b.jsx)(qt,{change:ye(t,c),claim:ke(t,c),draft:t,ctx:c,...s}):e===`storage`?(0,b.jsx)(Zt,{draft:t,set:n,ctx:c,objects:r.objects,onSectionErrors:o}):e===`components`?(0,b.jsx)(jt,{draft:t,set:n,ctx:c,secrets:i}):e===`served`?(0,b.jsx)(Ot,{draft:t,set:n,ctx:c,namespaces:r.namespaces,clusters:r.clusters,planes:r.planes,onSelectorErrors:a}):(0,b.jsx)(Nt,{draft:t,set:n,ctx:c,namespaces:r.namespaces,clusters:r.clusters,secrets:i})}function on(e){return e}var b,sn,cn;function ln(){return(ln=e((()=>{b=n(),t(),pt(),Mt(),Xe(),tt(),St(),te(),c(),f(),r(),xe(),h(),Dt(),Ft(),Kt(),Xt(),At(),tn(),`${a}${s}`,sn=[`PluginSection`,`PluginRegistration`,`PluginRequest`,`ManagementAuthConfig`,`DatabaseServerPreset`,`DatabaseConfig`],cn=[{id:`basics`,label:`Basics`},{id:`served`,label:`Environments served`},{id:`components`,label:`Components and versions`},{id:`storage`,label:`Storage and access`},{id:`review`,label:`Review`}],an.__docgenInfo={description:`PlaneStepBody renders the step current of the plane create flow. secrets
are the Secrets of the namespace of draft. The review step needs review.`,methods:[],displayName:`PlaneStepBody`,props:{current:{required:!0,tsType:{name:`string`},description:``},draft:{required:!0,tsType:{name:`PlaneDraft`},description:``},set:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(path: DraftPath, value: unknown) => void`,signature:{arguments:[{type:{name:`DraftPath`},name:`path`},{type:{name:`unknown`},name:`value`}],return:{name:`void`}}},description:``},models:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  environments: Environment[];
  namespaces: V1Namespace[];
  clusters: CamundaCluster[];
  platformConfigs: CamundaPlatformConfig[];
  releases: CamundaRelease[];
  sections: PluginSection[];
  registrations: PluginRegistration[];
  accessRequests: PluginRequest[];
  authConfigs: string[];
  objects: BuiltinObjects;
  planes: Plane[];
  sources: PlaneSources;
  images: ImagesTable;
}`,signature:{properties:[{key:`environments`,value:{name:`Array`,elements:[{name:`Environment`}],raw:`Environment[]`,required:!0}},{key:`namespaces`,value:{name:`Array`,elements:[{name:`V1Namespace`}],raw:`V1Namespace[]`,required:!0}},{key:`clusters`,value:{name:`Array`,elements:[{name:`CamundaCluster`}],raw:`CamundaCluster[]`,required:!0}},{key:`platformConfigs`,value:{name:`Array`,elements:[{name:`CamundaPlatformConfig`}],raw:`CamundaPlatformConfig[]`,required:!0}},{key:`releases`,value:{name:`Array`,elements:[{name:`CamundaRelease`}],raw:`CamundaRelease[]`,required:!0}},{key:`sections`,value:{name:`Array`,elements:[{name:`PluginSection`}],raw:`PluginSection[]`,required:!0}},{key:`registrations`,value:{name:`Array`,elements:[{name:`PluginRegistration`}],raw:`PluginRegistration[]`,required:!0}},{key:`accessRequests`,value:{name:`Array`,elements:[{name:`PluginRequest`}],raw:`PluginRequest[]`,required:!0}},{key:`authConfigs`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`objects`,value:{name:`BuiltinObjects`,required:!0}},{key:`planes`,value:{name:`Array`,elements:[{name:`Plane`}],raw:`Plane[]`,required:!0}},{key:`sources`,value:{name:`PlaneSources`,required:!0}},{key:`images`,value:{name:`ImagesTable`,required:!0}}]}},description:``},secrets:{required:!0,tsType:{name:`SecretRefs`},description:``},onSelectorErrors:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(errors: ValidationError[]) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`ValidationError`}],raw:`ValidationError[]`},name:`errors`}],return:{name:`void`}}},description:``},onSectionErrors:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(section: string, errors: ValidationError[]) => void`,signature:{arguments:[{type:{name:`string`},name:`section`},{type:{name:`Array`,elements:[{name:`ValidationError`}],raw:`ValidationError[]`},name:`errors`}],return:{name:`void`}}},description:``},review:{required:!1,tsType:{name:`signature`,type:`object`,raw:`{
  mode: WriteMode;
  preview: (change: Change) => Promise<Preview>;
  create: (change: Change) => Promise<WriteResult>;
  onWritten: (result: WriteResult) => void;
  blocked?: boolean;
  onBack?: () => void;
}`,signature:{properties:[{key:`mode`,value:{name:`WriteMode`,required:!0}},{key:`preview`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`create`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}},{key:`onWritten`,value:{name:`signature`,type:`function`,raw:`(result: WriteResult) => void`,signature:{arguments:[{type:{name:`WriteResult`},name:`result`}],return:{name:`void`}},required:!0}},{key:`blocked`,value:{name:`boolean`,required:!1}},{key:`onBack`,value:{name:`signature`,type:`function`,raw:`() => void`,signature:{arguments:[],return:{name:`void`}},required:!1}}]}},description:``}}}})))()}function un({data:e,initial:t,step:n,...r}){let i={environments:e.environments,namespaces:e.namespaces,clusters:e.clusters,platformConfigs:e.platformConfigs,releases:e.releases,sections:e.sections,registrations:e.registrations,accessRequests:e.accessRequests,authConfigs:e.authConfigs.map(e=>e.metadata?.name??``),objects:{DatabaseServerPreset:e.databaseServerPresets,DatabaseConfig:e.databaseConfigs},planes:bt(e.planes),sources:e.planes,images:e.images},[a,o]=(0,dn.useState)([]),[s,c]=(0,dn.useState)({}),l=rn(i,a,[],s),u=ft({steps:l,initial:t}),d=l.slice(0,l.findIndex(e=>e.id===n)+1);return(0,x.jsx)(rt,{steps:l.map(e=>({id:e.id,label:e.label,errors:d.includes(e)?e.check?.(u.draft):void 0})),current:n,children:(0,x.jsx)(an,{current:n,draft:u.draft,set:u.set,models:i,secrets:[{name:`keycloak-admin`,keys:[`password`,`username`]}],onSelectorErrors:o,onSectionErrors:(e,t)=>c(n=>({...n,[e]:t})),review:r})})}var x,dn,fn,S,pn,C,w,T,E,D,O,k,A,j,M,N,P,F,mn,hn,I,L,R,z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q,$,gn;function _n(){return(_n=e((()=>{x=n(),dn=t(),pt(),Ct(),Ze(),tt(),xt(),c(),_e(),xe(),ln(),Kt(),{expect:fn,fn:S,within:pn}=__STORYBOOK_MODULE_TEST__,C=(e,t,n,r)=>({apiVersion:`fleet.camunda.io/v1alpha1`,kind:`Environment`,metadata:{name:e},spec:{displayName:t,color:n,...r&&{platformConfigRef:r},releases:{allowed:[`camunda-8-9-23`],default:`camunda-8-9-23`},namespaceCreation:{allowed:!0,requiredLabelKeys:[`team`]},sections:{"management-database":{required:!0,allowed:[`existing-database-configs`,`management-database-server`],default:`management-database-server`},access:{allowed:[`gateway`]}},presets:{databaseServer:[`small`,`large`]},parameters:{"management-database-server":{preset:{claimEditable:!0}},"existing-database-configs":{keycloak:{claimEditable:!0},identity:{claimEditable:!0},webModeler:{claimEditable:!0}}}}}),w=(e,t,n,r,i,a)=>({apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginSection`,metadata:{name:e},spec:{appliesTo:[`CamundaManagementClusterClaim`],label:t,description:n,selection:`One`,slot:r,order:i,...a&&{emptyOptionLabel:a}}}),T=(e,t,n,r)=>({apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginRegistration`,metadata:{name:e},spec:{mode:`Optional`,label:t,description:n,deletionPolicies:[`Delete`],serviceAccount:{name:e,namespace:`plugins`},...r},status:{conditions:[`Accepted`,`Ready`].map(e=>({type:e,status:`True`,reason:`Ready`,message:``,lastTransitionTime:`2026-11-10T00:00:00Z`}))}}),E={name:`camunda-operator`,version:`0.0.0`},D=e=>({type:`string`,minLength:1,title:e}),O=(e,t,n)=>({apiVersion:`core.camunda.io/v1`,kind:`DatabaseServerPreset`,metadata:{name:e},spec:{server:{instances:t,storageSize:n}}}),k=e=>({apiVersion:`core.camunda.io/v1`,kind:`DatabaseConfig`,metadata:{namespace:`platform`,name:e},spec:{}}),A=(e,t)=>({apiVersion:`v1`,kind:`Namespace`,metadata:{name:e,...t&&{labels:{"fleet.camunda.io/environment":t}}}}),j=(e,t,n={})=>({apiVersion:`core.camunda.io/v1`,kind:`CamundaCluster`,metadata:{namespace:e,name:t,...n.holder&&{annotations:{"camunda.io/management-cluster":n.holder}}},spec:n.config===``?{}:{platformConfigRef:n.config??`basic-auth`},...n.gateway!==!1&&{status:{gateway:{grpcEndpoint:`${t}-gateway.${e}.svc:26500`,restEndpoint:`http://${t}-gateway.${e}.svc:8080`}}}}),M={environments:[C(`production`,`Production`,`#2E7D32`,`basic-auth`),C(`staging`,`Staging`,`#F9A825`,`basic-auth`)],namespaces:[A(`platform`,`production`),A(`team-orders`,`production`),A(`team-billing`,`production`),A(`team-search`,`staging`),A(`sandbox`)],clusters:[j(`team-orders`,`orders`),j(`team-billing`,`billing`,{holder:`ops/plane-legacy`}),j(`team-orders`,`orders-next`,{gateway:!1}),j(`team-billing`,`billing-sso`,{config:`sso`}),j(`team-search`,`search`,{config:``})],platformConfigs:[{apiVersion:`core.camunda.io/v1`,kind:`CamundaPlatformConfig`,metadata:{name:`basic-auth`},spec:{}},{apiVersion:`core.camunda.io/v1`,kind:`CamundaPlatformConfig`,metadata:{name:`sso`},spec:{auth:{method:`oidc`,oidc:{issuerUrl:`https://login.example.com/realms/camunda`}}}}],releases:[{apiVersion:`core.camunda.io/v1`,kind:`CamundaRelease`,metadata:{name:`camunda-8-9-23`},spec:{version:`8.9.23`}}],images:{schemaVersion:1,minors:{"8.9":{identity:{repository:`camunda/identity`,version:`8.9.10`},console:{repository:`camunda/console`,version:`8.9.122`},"web-modeler-restapi":{repository:`camunda/web-modeler-restapi`,version:`8.9.11`},keycloak:{repository:`camunda/keycloak`,tagPrefix:`quay-optimized-`,version:`26.6.4`}}}},sections:[w(`management-database`,`Databases`,`The databases of Keycloak, Management Identity, and Web Modeler.`,`managementDatabases`,80),w(`access`,`Access`,`How users and clients reach the cluster or the management plane from outside.`,`externalAccess`,60,`No access from outside`)],registrations:[T(`management-database-server`,`New PostgreSQL server`,`Runs a PostgreSQL server with one database for each component of the management plane.`,{sections:[`management-database`],controller:E,parameters:{type:`object`,required:[`preset`],properties:{preset:{type:`string`,minLength:1,title:`Preset`},databases:{type:`array`,items:{type:`string`}}}}}),T(`existing-database-configs`,`Existing databases`,`Uses DatabaseConfigs that are already in the namespace, one for each component.`,{sections:[`management-database`],controller:E,parameters:{type:`object`,properties:{keycloak:D(`Keycloak`),identity:D(`Management Identity`),webModeler:D(`Web Modeler`)}}}),T(`gateway`,`Gateway`,`Routes the plane through the shared Gateway.`,{sections:[`access`],parameterValues:{managementHostnamePattern:`{name}.mgmt.example.com`}})],accessRequests:[],authConfigs:[{apiVersion:`core.camunda.io/v1`,kind:`ManagementAuthConfig`,metadata:{name:`plane-legacy`}}],databaseServerPresets:[O(`small`,1,`10Gi`),O(`large`,3,`100Gi`)],databaseConfigs:[k(`keycloak-db`),k(`identity-db`),k(`modeler-db`)],planes:{clusters:[{apiVersion:`core.camunda.io/v1`,kind:`CamundaManagementCluster`,metadata:{namespace:`ops`,name:`plane-legacy`,labels:{[i]:`plane-legacy`}},spec:{identityProvider:{keycloak:{}}}}],claims:[{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaManagementClusterClaim`,metadata:{namespace:`ops`,name:`plane-legacy`},spec:{environments:[`staging`],management:{identityProvider:{keycloak:{}}}}}],namespaces:[],environments:[]}},N={...be,environment:`production`,name:`plane-a`,namespace:{kind:`existing`,name:`platform`},mode:`keycloak`},P={...N,admin:{...ge,username:`admin`,email:`admin@example.com`},console:!0},F={"management-database":[{section:`management-database`,registration:`management-database-server`,parameters:{preset:`small`}}],access:[{section:`access`,registration:`gateway`}]},mn={apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaManagementClusterClaim`,metadata:{namespace:`platform`,name:`plane-a`,generation:1},spec:{environments:[`production`],management:{identityProvider:{keycloak:{}},identity:{admin:{}}},versions:{identity:`8.9.10`},plugins:F[`management-database`]},status:{environment:`production`,requests:[{section:`management-database`,registration:`management-database-server`,name:`plane-a-management-database`,outputsReady:`True`,ready:`True`}],conditions:[{type:`Ready`,status:`False`,reason:`Creating`,message:`The CamundaManagementCluster plane-a has not reported Ready yet.`,lastTransitionTime:`2026-11-10T00:00:00Z`},{type:`ManagementClusterReady`,status:`False`,reason:`Creating`,message:`The CamundaManagementCluster plane-a has not reported Ready yet.`,lastTransitionTime:`2026-11-10T00:00:00Z`}]}},hn={title:`planes/create/PlaneCreateFlow`,component:un,args:{data:M,initial:N,step:`basics`,mode:`direct`,preview:S(async()=>({objects:[],errors:[]})),create:S(async()=>({ok:!0,mode:`direct`})),onWritten:S(),onBack:S()},render:e=>(0,x.jsx)(un,{...e},JSON.stringify(e.initial))},I={name:`Basics`},L={name:`Basics with no Environment`,args:{data:{...M,environments:[]},initial:be}},R={name:`External Keycloak`,args:{initial:{...N,mode:`externalKeycloak`,externalKeycloak:{url:`https://keycloak.example.com/auth`,realm:`camunda-platform`,adminSecret:`keycloak-admin`}}}},z={name:`Environments served with every result`,args:{step:`served`,initial:{...N,platformConfig:`sso`,mode:`oidc`,served:[`production`,`staging`]}}},B={name:`Keycloak mode preview`,args:{step:`served`}},V={name:`Components and versions in Keycloak mode`,args:{step:`components`,initial:{...P,webModeler:!0}}},H={name:`Components and versions in OIDC mode`,args:{step:`components`,initial:{...P,platformConfig:`sso`,mode:`oidc`,admin:{...ge,claimName:`sub`}}}},U={name:`Keycloak version with a warning`,args:{step:`components`,initial:{...P,versions:{keycloak:`26.7.0`}}}},W={name:`Storage and access with no access from outside`,args:{step:`storage`,initial:{...P,urls:{keycloak:`https://management.example.com/auth`}}}},G={name:`Storage and access with an access option`,args:{step:`storage`,initial:{...P,storage:{"management-database":[{section:`management-database`,registration:`existing-database-configs`,parameters:{keycloak:`keycloak-db`,identity:`identity-db`}}],access:[{section:`access`,registration:`gateway`}]}}}},K={name:`Storage and access in the page frame`,args:G.args,parameters:{layout:`fullscreen`},render:e=>(0,x.jsx)(wt,{pathname:`/fleet/planes/new`,children:(0,x.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[Qe(`New management plane`),(0,x.jsx)(un,{...e},JSON.stringify(e.initial))]})})},q={name:`Storage and access with a taken host`,args:{data:{...M,accessRequests:[{apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginRequest`,metadata:{namespace:`ops`,name:`plane-b-access`},spec:{claimRef:{kind:`CamundaManagementClusterClaim`,name:`plane-b`},section:`access`,registration:`gateway`},status:{outputs:{managementHost:`plane-a.mgmt.example.com`}}}]},step:`storage`,initial:{...P,storage:{access:[{section:`access`,registration:`gateway`}]}}}},J={name:`Storage and access with a pick of a section that is gone`,args:{step:`storage`,initial:{...P,storage:{"dns-zone":[{section:`dns-zone`,registration:`route53`}]}}}},Y={name:`Storage and access with a default that keeps the resources`,args:{data:{...M,environments:M.environments.map(e=>e.metadata.name===`production`?{...e,spec:{...e.spec,deletionPolicy:`Orphan`}}:e)},step:`storage`,initial:{...P,storage:F}}},X={name:`Review in direct mode`,args:{step:`review`,initial:{...P,storage:F}},play:async({canvasElement:e})=>{let t=pn(pn(e).getByRole(`region`,{name:`Review`}));await fn(t.getByRole(`button`,{name:`Create`})).toBeVisible(),await fn(t.getByRole(`button`,{name:`Back`})).toBeVisible()}},Z={name:`Review in Git mode`,args:{step:`review`,mode:`git`,initial:{...P,namespace:{kind:`new`,name:`team-plane`,labels:{[o]:`production`,team:`platform`}},storage:F}}},Q={name:`Progress with every row pending`,parameters:{controls:{disable:!0}},render:()=>(0,x.jsx)(Ut,{name:`plane-a`,sections:M.sections})},$={name:`Progress with the databases ready`,parameters:{controls:{disable:!0}},render:()=>(0,x.jsx)(Ut,{name:`plane-a`,sections:M.sections,claim:mn})},gn=[`Basics`,`BasicsWithNoEnvironment`,`ExternalKeycloak`,`EnvironmentsServedWithEveryResult`,`KeycloakModePreview`,`ComponentsAndVersionsInKeycloakMode`,`ComponentsAndVersionsInOidcMode`,`KeycloakVersionWithAWarning`,`StorageAndAccessWithNoAccessFromOutside`,`StorageAndAccessWithAnAccessOption`,`StorageAndAccessInThePageFrame`,`StorageAndAccessWithATakenHost`,`StorageAndAccessWithAPickOfASectionThatIsGone`,`StorageAndAccessWithADefaultThatKeepsTheResources`,`ReviewInDirectMode`,`ReviewInGitMode`,`ProgressWithEveryRowPending`,`ProgressWithTheDatabasesReady`],I.parameters={...I.parameters,docs:{...I.parameters?.docs,source:{originalSource:`{
  name: "Basics"
}`,...I.parameters?.docs?.source}}},L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
  name: "Basics with no Environment",
  args: {
    data: {
      ...data,
      environments: []
    },
    initial: EMPTY_PLANE_DRAFT
  }
}`,...L.parameters?.docs?.source}}},R.parameters={...R.parameters,docs:{...R.parameters?.docs,source:{originalSource:`{
  name: "External Keycloak",
  args: {
    initial: {
      ...basics,
      mode: "externalKeycloak",
      externalKeycloak: {
        url: "https://keycloak.example.com/auth",
        realm: "camunda-platform",
        adminSecret: "keycloak-admin"
      }
    }
  }
}`,...R.parameters?.docs?.source}}},z.parameters={...z.parameters,docs:{...z.parameters?.docs,source:{originalSource:`{
  name: "Environments served with every result",
  args: {
    step: "served",
    initial: {
      ...basics,
      platformConfig: "sso",
      mode: "oidc",
      served: ["production", "staging"]
    }
  }
}`,...z.parameters?.docs?.source}}},B.parameters={...B.parameters,docs:{...B.parameters?.docs,source:{originalSource:`{
  name: "Keycloak mode preview",
  args: {
    step: "served"
  }
}`,...B.parameters?.docs?.source}}},V.parameters={...V.parameters,docs:{...V.parameters?.docs,source:{originalSource:`{
  name: "Components and versions in Keycloak mode",
  args: {
    step: "components",
    initial: {
      ...components,
      webModeler: true
    }
  }
}`,...V.parameters?.docs?.source}}},H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
  name: "Components and versions in OIDC mode",
  args: {
    step: "components",
    initial: {
      ...components,
      platformConfig: "sso",
      mode: "oidc",
      admin: {
        ...EMPTY_ADMIN,
        claimName: "sub"
      }
    }
  }
}`,...H.parameters?.docs?.source}}},U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
  name: "Keycloak version with a warning",
  args: {
    step: "components",
    initial: {
      ...components,
      versions: {
        keycloak: "26.7.0"
      }
    }
  }
}`,...U.parameters?.docs?.source}}},W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "Storage and access with no access from outside",
  args: {
    step: "storage",
    initial: {
      ...components,
      urls: {
        keycloak: "https://management.example.com/auth"
      }
    }
  }
}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "Storage and access with an access option",
  args: {
    step: "storage",
    initial: {
      ...components,
      storage: {
        "management-database": [{
          section: "management-database",
          registration: "existing-database-configs",
          parameters: {
            keycloak: "keycloak-db",
            identity: "identity-db"
          }
        }],
        access: [{
          section: "access",
          registration: "gateway"
        }]
      }
    }
  }
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "Storage and access in the page frame",
  args: StorageAndAccessWithAnAccessOption.args,
  parameters: {
    layout: "fullscreen"
  },
  render: args => <AppShell pathname="/fleet/planes/new">
      <div className="flex flex-col gap-6">
        {heading("New management plane")}
        <PlaneCreateHarness key={JSON.stringify(args.initial)} {...args} />
      </div>
    </AppShell>
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "Storage and access with a taken host",
  args: {
    data: {
      ...data,
      accessRequests: [{
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginRequest",
        metadata: {
          namespace: "ops",
          name: "plane-b-access"
        },
        spec: {
          claimRef: {
            kind: "CamundaManagementClusterClaim",
            name: "plane-b"
          },
          section: "access",
          registration: "gateway"
        },
        status: {
          outputs: {
            managementHost: "plane-a.mgmt.example.com"
          }
        }
      }] as unknown as PlaneCreateData["accessRequests"]
    },
    step: "storage",
    initial: {
      ...components,
      storage: {
        access: [{
          section: "access",
          registration: "gateway"
        }]
      }
    }
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Storage and access with a pick of a section that is gone",
  args: {
    step: "storage",
    initial: {
      ...components,
      storage: {
        "dns-zone": [{
          section: "dns-zone",
          registration: "route53"
        }]
      }
    }
  }
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Storage and access with a default that keeps the resources",
  args: {
    data: {
      ...data,
      environments: data.environments.map(e => e.metadata.name === "production" ? {
        ...e,
        spec: {
          ...e.spec,
          deletionPolicy: "Orphan" as const
        }
      } : e)
    },
    step: "storage",
    initial: {
      ...components,
      storage: reviewStorage
    }
  }
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Review in direct mode",
  args: {
    step: "review",
    initial: {
      ...components,
      storage: reviewStorage
    }
  },
  play: async ({
    canvasElement
  }) => {
    const review = within(within(canvasElement).getByRole("region", {
      name: "Review"
    }));
    await expect(review.getByRole("button", {
      name: "Create"
    })).toBeVisible();
    await expect(review.getByRole("button", {
      name: "Back"
    })).toBeVisible();
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Review in Git mode",
  args: {
    step: "review",
    mode: "git",
    initial: {
      ...components,
      namespace: {
        kind: "new",
        name: "team-plane",
        labels: {
          [ENVIRONMENT_LABEL]: "production",
          team: "platform"
        }
      },
      storage: reviewStorage
    }
  }
}`,...Z.parameters?.docs?.source}}},Q.parameters={...Q.parameters,docs:{...Q.parameters?.docs,source:{originalSource:`{
  name: "Progress with every row pending",
  parameters: {
    controls: {
      disable: true
    }
  },
  render: () => <PlaneProgressView name="plane-a" sections={data.sections as unknown as PluginSection[]} />
}`,...Q.parameters?.docs?.source}}},$.parameters={...$.parameters,docs:{...$.parameters?.docs,source:{originalSource:`{
  name: "Progress with the databases ready",
  parameters: {
    controls: {
      disable: true
    }
  },
  render: () => <PlaneProgressView name="plane-a" sections={data.sections as unknown as PluginSection[]} claim={progressClaim} />
}`,...$.parameters?.docs?.source}}}})))()}_n();export{I as Basics,L as BasicsWithNoEnvironment,V as ComponentsAndVersionsInKeycloakMode,H as ComponentsAndVersionsInOidcMode,z as EnvironmentsServedWithEveryResult,R as ExternalKeycloak,B as KeycloakModePreview,U as KeycloakVersionWithAWarning,Q as ProgressWithEveryRowPending,$ as ProgressWithTheDatabasesReady,X as ReviewInDirectMode,Z as ReviewInGitMode,K as StorageAndAccessInThePageFrame,Y as StorageAndAccessWithADefaultThatKeepsTheResources,J as StorageAndAccessWithAPickOfASectionThatIsGone,q as StorageAndAccessWithATakenHost,G as StorageAndAccessWithAnAccessOption,W as StorageAndAccessWithNoAccessFromOutside,gn as __namedExportsOrder,hn as default};