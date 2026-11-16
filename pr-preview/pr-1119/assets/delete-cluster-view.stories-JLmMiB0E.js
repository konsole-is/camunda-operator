import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{b as r,c as i}from"./labels-CoLYcJ9C.js";import{n as a,t as o}from"./circle-exclamation-point-jpEoxFxv.js";import{d as s,t as c}from"./merge-E_SYsi8x.js";import{f as l,t as u,u as d,w as f}from"./policy-BjdGZV2S.js";import{Z as ee,it as p,t as m}from"./session-BwN97dQE.js";import{n as h}from"./refs-CNug01Rr.js";import{n as g,t as _}from"./link-DOLMGIR2.js";import{a as v,c as te,i as y,n as b,o as ne,r as re,s as ie,t as ae}from"./table-eKockZTr.js";import{r as oe,t as se}from"./sections-DXo8K5no.js";import{n as ce,t as x}from"./badge-zSjG-ZWH.js";import{a as le,n as ue}from"./deletion-choice-DNTRlGUp.js";import{n as de,t as fe}from"./change-review-BQHPkEiV.js";import{n as pe,t as me}from"./precheck-line-CvdAgOm2.js";import{a as he,r as ge}from"./rows-tKUpB1Au.js";import{i as _e,r as ve}from"./waiting-panel-BuuBPgSN.js";import{c as ye,n as be}from"./cluster-edit-RoL4YM-d.js";function xe(e,t){return t===`Delete`?`Deleted`:e.onOrphan}function Se(e,t,n){let{apiVersion:r,kind:i,metadata:a,spec:o,status:s}=e,c=a.name??``,u={apiVersion:r,kind:i,namespace:a.namespace,name:c},d={intent:n,apply:[],delete:[u],...a.uid&&{uids:{[h(u)]:a.uid}},...a.resourceVersion&&{preconditions:{[h(u)]:a.resourceVersion}}};if(l(o?.deletionPolicy,s?.resolvedPolicy)===t)return{remove:d};let{session:f}=p({claim:e,pending:new Map,locks:()=>void 0},[{field:`spec`},{field:`deletionPolicy`}],t);return{policy:ee(f,`Set the deletion policy of ${c} to ${t} before the delete`),remove:d}}function Ce(e){let{claim:t,sections:n,registrations:r,requests:i,policy:a}=e;return oe({claim:t,sections:n,requests:i}).map(e=>{let{registration:t}=e.selection,o=e.request??i.find(n=>n.spec?.section===e.section&&n.spec?.registration===t),s=n.find(t=>t.metadata?.name===e.section),c=s?f(s):void 0,l=r.find(e=>e.metadata?.name===t)?.spec.deletionPolicies;return{id:e.id,section:e.section,label:e.label,registration:t,...c&&{slot:c},alwaysDeletes:l!==void 0&&!l.includes(`Orphan`),rows:(o?.status?.resources??[]).map(e=>({...e,fate:xe(e,a)}))}})}function we(e,t){return t.some(e=>e.slot===`backupStorage`&&e.rows.some(e=>e.fate===`Deleted`))?`You cannot restore ${e} after this: the backup bucket goes too.`:void 0}function S(){return(S=e((()=>{u(),m(),se()})))()}function Te(e){let{claim:t}=e;return d({claim:{...t,spec:{...t.spec,plugins:t.spec?.plugins}},environment:Ee(e),sections:e.sections,registrations:e.registrations,kind:`CamundaClusterClaim`})}function Ee(e){let t=e.namespace?.metadata?.labels?.[i];return t===void 0?void 0:e.environments.find(e=>e.metadata?.name===t)}function C(e,t,n=[]){let{claim:r}=e,i=r.metadata?.name??``,a=r.metadata?.namespace??``,o=Te(e),s=r.metadata?.uid,c=e=>s!==void 0&&(e.metadata?.ownerReferences??[]).some(e=>e.controller===!0&&e.uid===s),l=e.requests.filter(c),u=Ce({claim:r,sections:e.sections,registrations:e.registrations,requests:l,policy:t}),d=e=>e.spec.clusterRef.name===i,f=(e,t,n)=>({kind:e,name:t,label:`${e} ${t}`,...n&&{note:n}}),ee=e=>r.status?.requests?.find(t=>t.name===e)?.section,p=(e,t,n)=>{let r=e=>e.metadata?.name===n&&c(e),i=t.filter(e=>d(e)||e.metadata?.name===n);return{goes:i.filter(r).map(t=>f(e,t.metadata?.name??``)),stays:i.filter(e=>!r(e)).map(t=>f(e,t.metadata?.name??``,w))}},m=p(`CamundaOptimize`,e.optimizes,`${i}-optimize`),h=p(`BackupSchedule`,e.schedules,`${i}-backups`),g=!n.some(e=>Ae.includes(e)),_=!n.some(e=>ke.includes(e)),v=e.cluster?c(e.cluster):r.status?.cluster?.name===i,te=!v&&e.cluster?[f(`CamundaCluster`,i,w)]:[],y=[...m.goes,...h.goes,...v?[f(`CamundaCluster`,i,g?e.cluster?De(e)===`Retain`?`Volumes kept for a cluster of the same name`:`Volumes deleted`:void 0:`Volumes not known`)]:[],...l.map(e=>{let t=e.metadata?.name??``,n=ee(t),r=u.find(e=>e.section===n)?.label;return{...f(`PluginRequest`,t),...r&&{label:`${r} request ${t}`}}}),f(`CamundaClusterClaim`,i)],b=_?we(i,u):void 0;return{name:i,namespace:a,choice:{...o,policy:t},keepRefused:o.policy===`Orphan`&&!o.keepAllowed,targets:y,kept:[...te,...m.stays,...h.stays],groups:u,backups:e.backups.elasticsearch.filter(d).length+e.backups.rdbms.filter(d).length,...b&&{warning:b},unread:[...n],fatesKnown:_}}function De(e){let t=e.cluster?.spec;return t?s(e.presets.find(e=>e.metadata?.name===t.presetRef)?.spec,e.releases.find(e=>e.metadata?.name===t.releaseRef)?.spec,t).spec.zeebe?.persistentVolumeClaimRetentionPolicy?.whenDeleted??`Delete`:`Delete`}function Oe(e,t,n=[]){return e.map(e=>{if(n.includes(e.kind))return{target:e,state:`Not known`};let r=t[e.kind]?.find(t=>t.metadata?.name===e.name);return{target:e,state:r===void 0?`Deleted`:r.metadata?.deletionTimestamp?`Deleting`:`Waiting`}})}var ke,Ae,w;function je(){return(je=e((()=>{S(),r(),c(),u(),ke=[`PluginRequest`,`PluginRegistration`,`PluginSection`],Ae=[`CamundaCluster`,`CamundaClusterPreset`,`CamundaRelease`],w=`Kept: the claim did not make it`})))()}function Me({model:e,mode:t,stage:n,errors:r,remove:i,policyStep:a,onPick:o,preview:s,apply:c,show:l,onWritten:u,onStopWaiting:d}){let{name:f,namespace:ee}=e,p=`/fleet/clusters/${ee}/${f}`,m=Ne(e,t===`git`&&a),h=e.kept.some(e=>e.kind===`CamundaCluster`),_=e.choice.policy===`Orphan`&&!e.choice.keepAllowed,[v,te]=(0,Re.useState)(!1),y=(0,E.jsx)(`section`,{"aria-label":`What the delete takes`,children:(0,E.jsx)(`ul`,{className:`flex list-disc flex-col gap-1 pl-5 text-sm text-text-2`,children:m.map((e,t)=>(0,E.jsx)(`li`,{children:e},t))})});return(0,E.jsxs)(`div`,{className:`flex max-w-3xl min-w-0 flex-col gap-6`,children:[(0,E.jsxs)(`div`,{className:`flex flex-col gap-2`,children:[(0,E.jsx)(g,{href:p,className:`relative w-fit text-sm text-muted hit-area hover:text-text`,children:`Back to the cluster`}),(0,E.jsxs)(`h1`,{className:`font-heading text-2xl font-bold tracking-[-0.02em] break-all`,children:[`Delete the cluster `,f]})]}),n.kind===`deleted`?(0,E.jsxs)(`div`,{className:`flex flex-col gap-2 text-sm`,children:[(0,E.jsx)(`p`,{role:`status`,className:`text-text`,children:h?`The claim ${f} is deleted. The cluster ${f} stays: the claim did not make it.`:`The cluster ${f} is deleted.`}),(0,E.jsx)(g,{href:`/fleet`,className:`relative w-fit text-accent hit-area hover:underline`,children:`Back to the fleet`})]}):n.kind===`replaced`?(0,E.jsxs)(`div`,{className:`flex flex-col gap-2 text-sm`,children:[(0,E.jsxs)(`p`,{role:`status`,className:`text-text`,children:[`The claim `,f,` was made again after this page opened. Open the delete page again to delete the new claim.`]}),(0,E.jsx)(g,{href:p,className:`relative w-fit text-accent hit-area hover:underline`,children:`Back to the cluster`})]}):n.kind===`waiting`?(0,E.jsxs)(E.Fragment,{children:[(0,E.jsx)(T,{errors:r}),(0,E.jsx)(ve,{text:t===`git`?`Waiting for GitOps to apply the deletion policy. The fleet manager then removes the claim file.`:`Waiting for the operator to read the deletion policy. The fleet manager then deletes the claim.`,onStopWaiting:d})]}):n.kind===`deleting`?(0,E.jsxs)(E.Fragment,{children:[(0,E.jsx)(T,{errors:r}),(0,E.jsx)(Fe,{name:f,stage:n})]}):(0,E.jsxs)(E.Fragment,{children:[(0,E.jsx)(ue,{subject:`cluster`,choice:e.choice,onChange:o}),e.keepRefused&&e.choice.policy!==`Orphan`&&(0,E.jsx)(`p`,{className:`-mt-3 text-sm text-warn`,children:`The claim keeps its resources, but an option below cannot keep them. The delete removes them.`}),t===`git`&&(0,E.jsx)(`p`,{className:`text-sm text-text-2`,children:Be}),(0,E.jsx)(T,{errors:r}),n.errors&&n.errors.length>0&&(0,E.jsx)(Le,{texts:n.errors.map(e=>e.message)}),n.afterPolicyWrite&&(0,E.jsx)(`p`,{role:`status`,className:`text-sm text-warn`,children:Ie(f,n.afterPolicyWrite)}),t===void 0?(0,E.jsxs)(E.Fragment,{children:[(0,E.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:ye}),y]}):e.unread.length>0&&!v?(0,E.jsxs)(E.Fragment,{children:[(0,E.jsx)(`p`,{className:`text-sm text-muted`,children:he(e.unread)}),y]}):_&&!v?(0,E.jsxs)(E.Fragment,{children:[(0,E.jsx)(`p`,{className:`text-sm text-warn`,children:ze}),y]}):(0,E.jsx)(fe,{change:i,mode:t,diffLabel:`The claim that the delete removes`,effects:m,caution:e.warning,confirm:{label:`Delete the cluster`,gitLabel:`Push the delete to Git`,danger:!0,typed:f},savedMessage:t===`direct`?a?`Set the deletion policy of ${f}. The delete follows when the operator reads it.`:`The claim ${f} is being deleted.`:a?`Pushed the deletion policy of ${f}. The delete follows when GitOps applies it.`:`Pushed the delete of ${f}.`,preview:s,apply:c,show:l,onWritten:u,onWriting:te})]})]})}function Ne(e,t){let n=e.unread.filter(e=>Ve.includes(e)),r=e=>!n.includes(e.kind),i=e.targets.filter(e=>e.kind!==`PluginRequest`&&r(e)),a=e.kept.filter(r),o=[...e.targets,...e.kept].filter(e=>!r(e)),s=(e,t)=>e.map(e=>(0,E.jsxs)(`li`,{children:[e.label,(t??e.note)&&(0,E.jsxs)(`span`,{className:`text-muted`,children:[`: `,t??e.note]})]},`${e.kind}/${e.name}`));return[(0,E.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,E.jsx)(`span`,{className:`font-medium text-text`,children:`Always deleted`}),(0,E.jsx)(`ul`,{className:`flex flex-col gap-0.5`,children:s(i)})]},`always`),...a.length>0?[(0,E.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,E.jsx)(`span`,{className:`font-medium text-text`,children:`Kept`}),(0,E.jsx)(`ul`,{className:`flex flex-col gap-0.5`,children:s(a)})]},`kept`)]:[],...n.length>0?[(0,E.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,E.jsx)(`span`,{className:`font-medium text-text`,children:`Not known`}),(0,E.jsxs)(`ul`,{className:`flex flex-col gap-0.5`,children:[s(o,`not known`),n.filter(e=>!o.some(t=>t.kind===e)).map(e=>(0,E.jsxs)(`li`,{children:[e,(0,E.jsx)(`span`,{className:`text-muted`,children:`: not known`})]},e))]})]},`unknown`)]:[],...e.groups.map(t=>(0,E.jsx)(Pe,{group:t,known:e.fatesKnown},t.id)),...e.backups>0?[`The ${e.backups} backups of ${e.name} stay, and their artifacts stay in the bucket.`]:[],...t?[`Pushes two commits: the deletion policy first, then the delete once GitOps applied it.`]:[]]}function Pe({group:e,known:t}){let n=e.id.includes(`/`)?`${e.label} (${e.registration})`:e.label;return(0,E.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-1`,children:[e.rows.length===0?(0,E.jsxs)(E.Fragment,{children:[(0,E.jsx)(`span`,{className:`font-medium text-text`,children:n}),(0,E.jsx)(`span`,{children:t?`This option made no resources.`:`What this option made is not known.`})]}):(0,E.jsxs)(ae,{label:`Resources of ${n}`,children:[(0,E.jsx)(re,{className:`mt-0 mb-1 caption-top text-left font-medium text-text`,children:n}),(0,E.jsx)(ne,{children:(0,E.jsxs)(ie,{children:[(0,E.jsx)(v,{children:`Kind`}),(0,E.jsx)(v,{children:`Name`}),(0,E.jsx)(v,{children:`ID`}),(0,E.jsx)(v,{children:`What happens`})]})}),(0,E.jsx)(b,{children:e.rows.map(e=>(0,E.jsxs)(ie,{children:[(0,E.jsx)(y,{children:e.kind}),(0,E.jsx)(y,{className:`break-all whitespace-normal`,children:e.name}),(0,E.jsx)(y,{className:`break-all whitespace-normal text-muted`,children:e.id??`-`}),(0,E.jsx)(y,{children:t?(0,E.jsx)(x,{variant:e.fate===`Deleted`?`destructive`:`outline`,className:e.fate===`Kept`?`text-muted`:void 0,children:e.fate}):(0,E.jsx)(x,{variant:`outline`,children:`Not known`})})]},`${e.kind}/${e.name}`))})]}),e.alwaysDeletes&&(0,E.jsx)(`span`,{className:`text-warn`,children:`This option always deletes what it made.`})]})}function Fe({name:e,stage:t}){return(0,E.jsxs)(`section`,{"aria-label":`Delete progress`,className:`flex flex-col gap-3`,children:[t.gitWaiting&&(0,E.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[(0,E.jsxs)(`p`,{role:`status`,className:`text-text`,children:[`Waiting for GitOps to delete `,e]}),(0,E.jsx)(`p`,{className:`text-text-2`,children:Be})]}),t.claim&&(0,E.jsx)(me,{claim:t.claim}),(0,E.jsx)(`ol`,{className:`flex flex-col divide-y divide-divider rounded-md border border-border`,children:t.progress.map(({target:e,state:t})=>(0,E.jsxs)(`li`,{className:`flex flex-wrap items-center justify-between gap-x-4 gap-y-1 px-3 py-2 text-sm`,children:[(0,E.jsxs)(`span`,{className:`flex min-w-0 flex-col`,children:[(0,E.jsx)(`span`,{className:`break-all text-text`,children:e.label}),e.note&&(0,E.jsx)(`span`,{className:`text-muted`,children:e.note})]}),(0,E.jsx)(x,{variant:t===`Waiting`||t===`Not known`?`outline`:`secondary`,className:t===`Deleted`?`text-muted`:void 0,children:t})]},`${e.kind}/${e.name}`))})]})}function Ie(e,t){let n=e=>e===`Orphan`?`Keep`:`Delete`,r=`The claim ${e} has the deletion policy ${n(t.held)}.`;return t.unapplied?`The cluster is not deleted. The repository holds the commit that sets the deletion policy of ${e} to ${n(t.unapplied)}, and GitOps has not applied it yet. ${r}`:`The cluster is not deleted. ${r}`}function T({errors:e}){return e.map((e,t)=>(0,E.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},t))}function Le({texts:e}){return(0,E.jsxs)(`div`,{role:`alert`,className:`flex gap-3 rounded-md border border-bad/40 bg-bad/10 p-3 text-sm text-text`,children:[(0,E.jsx)(o,{"aria-hidden":!0,className:`mt-0.5 size-4 shrink-0 text-bad`}),(0,E.jsx)(`ul`,{className:`flex flex-col gap-1`,children:e.map((e,t)=>(0,E.jsx)(`li`,{children:e},t))})]})}var E,Re,ze,Be,Ve;function He(){return(He=e((()=>{E=n(),a(),_(),Re=t(),de(),le(),pe(),ge(),ce(),te(),be(),_e(),ze=`An option below cannot keep its resources. Choose Delete the resources to delete the cluster.`,Be=`Your GitOps tool must prune: the fleet manager removes the claim file, and GitOps deletes the claim.`,Ve=[`CamundaCluster`,`CamundaOptimize`,`BackupSchedule`],Me.__docgenInfo={description:`DeleteClusterView is the delete page of a cluster. It never writes: the
confirm of its review calls apply with remove, and the review previews
remove once for each remove object. Without a mode it shows CHANGES_OFF
and what the delete takes, and no review. While model.unread has a kind
it shows what the delete takes, and no review, unless a write runs.
policyStep says that the delete writes the deletion policy first.`,methods:[],displayName:`DeleteClusterView`,props:{model:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  name: string;
  namespace: string;
  choice: DeletionChoice;
  keepRefused: boolean;
  targets: DeleteTarget[];
  kept: DeleteTarget[];
  groups: DeletionGroup[];
  backups: number;
  warning?: string;
  unread: string[];
  fatesKnown: boolean;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`namespace`,value:{name:`string`,required:!0}},{key:`choice`,value:{name:`DeletionChoice`,required:!0}},{key:`keepRefused`,value:{name:`boolean`,required:!0}},{key:`targets`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  kind: KindName;
  name: string;
  label: string;
  note?: string;
}`,signature:{properties:[{key:`kind`,value:{name:`KindName`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`note`,value:{name:`string`,required:!1}}]}}],raw:`DeleteTarget[]`,required:!0}},{key:`kept`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  kind: KindName;
  name: string;
  label: string;
  note?: string;
}`,signature:{properties:[{key:`kind`,value:{name:`KindName`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`note`,value:{name:`string`,required:!1}}]}}],raw:`DeleteTarget[]`,required:!0}},{key:`groups`,value:{name:`Array`,elements:[{name:`DeletionGroup`}],raw:`DeletionGroup[]`,required:!0}},{key:`backups`,value:{name:`number`,required:!0}},{key:`warning`,value:{name:`string`,required:!1}},{key:`unread`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`fatesKnown`,value:{name:`boolean`,required:!0}}]}},description:``},mode:{required:!0,tsType:{name:`union`,raw:`WriteMode | undefined`,elements:[{name:`WriteMode`},{name:`undefined`}]},description:``},stage:{required:!0,tsType:{name:`union`,raw:`| {
    kind: "choose";
    errors?: ApiError[];
    afterPolicyWrite?: PolicyAfterWrite;
  }
| { kind: "waiting" }
| {
    kind: "deleting";
    progress: { target: DeleteTarget; state: DeleteState }[];
    gitWaiting: boolean;
    claim?: Resource<"CamundaClusterClaim">;
  }
| { kind: "deleted" }
| { kind: "replaced" }`,elements:[{name:`signature`,type:`object`,raw:`{
  kind: "choose";
  errors?: ApiError[];
  afterPolicyWrite?: PolicyAfterWrite;
}`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"choose"`,required:!0}},{key:`errors`,value:{name:`Array`,elements:[{name:`ApiError`}],raw:`ApiError[]`,required:!1}},{key:`afterPolicyWrite`,value:{name:`PolicyAfterWrite`,required:!1}}]}},{name:`signature`,type:`object`,raw:`{ kind: "waiting" }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"waiting"`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{
  kind: "deleting";
  progress: { target: DeleteTarget; state: DeleteState }[];
  gitWaiting: boolean;
  claim?: Resource<"CamundaClusterClaim">;
}`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"deleting"`,required:!0}},{key:`progress`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ target: DeleteTarget; state: DeleteState }`,signature:{properties:[{key:`target`,value:{name:`signature`,type:`object`,raw:`{
  kind: KindName;
  name: string;
  label: string;
  note?: string;
}`,signature:{properties:[{key:`kind`,value:{name:`KindName`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`note`,value:{name:`string`,required:!1}}]},required:!0}},{key:`state`,value:{name:`union`,raw:`"Waiting" | "Deleting" | "Deleted" | "Not known"`,elements:[{name:`literal`,value:`"Waiting"`},{name:`literal`,value:`"Deleting"`},{name:`literal`,value:`"Deleted"`},{name:`literal`,value:`"Not known"`}],required:!0}}]}}],raw:`{ target: DeleteTarget; state: DeleteState }[]`,required:!0}},{key:`gitWaiting`,value:{name:`boolean`,required:!0}},{key:`claim`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaClusterClaim"`}],raw:`Resource<"CamundaClusterClaim">`,required:!1}}]}},{name:`signature`,type:`object`,raw:`{ kind: "deleted" }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"deleted"`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{ kind: "replaced" }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"replaced"`,required:!0}}]}}]},description:``},errors:{required:!0,tsType:{name:`unknown`},description:``},remove:{required:!0,tsType:{name:`Change`},description:``},policyStep:{required:!0,tsType:{name:`boolean`},description:``}}}})))()}var Ue,D,O,We,k,Ge,A,j,M,N,P,F,I,L,Ke,qe,Je,R,z,B,V,H,U,W,Ye,G,K,q,J,Y,X,Z,Q,$,Xe;function Ze(){return(Ze=e((()=>{Ue=n(),S(),je(),He(),{fn:D}=__STORYBOOK_MODULE_TEST__,O=(e,t)=>t,We=`8c1f6c1e-1d1a-4c5e-9d43-0f3b1c2d4e5f`,k={ownerReferences:[{controller:!0,uid:We}]},Ge=[{section:`access`,registration:`gateway`},{section:`backup-storage`,registration:`aws-s3`},{section:`secondary-storage`,registration:`elasticsearch`}],A=(e,t={})=>O(`CamundaClusterClaim`,{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{namespace:`team-orders`,name:`orders`,resourceVersion:`41`,uid:We},spec:{releaseRef:`camunda-8-9-1`,plugins:Ge,deletionPolicy:e},status:{resolvedPolicy:{deletionPolicy:`Orphan`},requests:Ge.map(e=>({...e,name:`orders-${e.section}`})),cluster:{name:`orders`},...t}}),j=(e,t,n,r)=>O(`PluginSection`,{metadata:{name:e},spec:{label:t,order:n,selection:`One`,appliesTo:[`CamundaClusterClaim`],...r&&{slot:r}}}),M=(e,t,n,r)=>O(`PluginRegistration`,{metadata:{name:e},spec:{label:t,sections:n,deletionPolicies:r}}),N=(e,t)=>O(`PluginRequest`,{metadata:{namespace:`team-orders`,name:e,...k},status:{resources:t}}),P=e=>({spec:{clusterRef:{name:e}}}),F=(e={})=>({claim:A(),cluster:O(`CamundaCluster`,{metadata:{namespace:`team-orders`,name:`orders`,...k},spec:{zeebe:{persistentVolumeClaimRetentionPolicy:{whenDeleted:`Retain`}}}}),environments:[],sections:[j(`access`,`Access`,10,`externalAccess`),j(`backup-storage`,`Backup storage`,20,`backupStorage`),j(`secondary-storage`,`Secondary storage`,30,`secondaryStorage`)],registrations:[M(`gateway`,`Gateway`,[`access`],[`Delete`,`Orphan`]),M(`aws-s3`,`Amazon S3`,[`backup-storage`],[`Delete`,`Orphan`]),M(`elasticsearch`,`Elasticsearch`,[`secondary-storage`],[`Delete`,`Orphan`])],requests:[N(`orders-access`,[{kind:`HTTPRoute`,name:`orders`,onOrphan:`Deleted`},{kind:`Certificate`,name:`orders-tls`,onOrphan:`Kept`}]),N(`orders-backup-storage`,[{kind:`S3Bucket`,name:`orders-backups-7f3a`,id:`arn:aws:s3:::orders-backups-7f3a`,onOrphan:`Kept`}]),N(`orders-secondary-storage`,[{kind:`ElasticsearchCluster`,name:`orders-es`,onOrphan:`Kept`}])],optimizes:[O(`CamundaOptimize`,{metadata:{namespace:`team-orders`,name:`orders-optimize`,...k},...P(`orders`)})],schedules:[O(`BackupSchedule`,{metadata:{namespace:`team-orders`,name:`orders-backups`,...k},...P(`orders`)}),O(`BackupSchedule`,{metadata:{namespace:`team-orders`,name:`orders-hourly`},...P(`orders`)})],backups:{elasticsearch:[`orders-1`,`orders-2`,`orders-3`].map(e=>O(`LogicalBackupElasticsearch`,{metadata:{namespace:`team-orders`,name:e},...P(`orders`)})),rdbms:[]},presets:[],releases:[],errors:[],...e}),I=F({claim:A(`Orphan`),registrations:[M(`gateway`,`Gateway`,[`access`],[`Delete`]),M(`aws-s3`,`Amazon S3`,[`backup-storage`],[`Delete`,`Orphan`]),M(`elasticsearch`,`Elasticsearch`,[`secondary-storage`],[`Delete`,`Orphan`])]}),L=C(F(),`Orphan`),Ke=Se(F().claim,`Orphan`,`Delete the cluster orders`),qe={title:`cluster/DeleteClusterView`,component:Me,args:{model:L,mode:`direct`,stage:{kind:`choose`},errors:[],remove:Ke.remove,policyStep:!1,onPick:D(),preview:D(async()=>({objects:[],errors:[]})),apply:D(async()=>({ok:!0,mode:`direct`})),show:D(),onWritten:D(),onStopWaiting:D()},decorators:[e=>(0,Ue.jsx)(`div`,{className:`max-w-3xl`,children:(0,Ue.jsx)(e,{})})]},Je=Oe(L.targets,{CamundaOptimize:[],BackupSchedule:[],CamundaCluster:[{metadata:{name:`orders`,deletionTimestamp:`2026-11-15T10:00:00Z`}}],PluginRequest:[{metadata:{name:`orders-access`}},{metadata:{name:`orders-backup-storage`}},{metadata:{name:`orders-secondary-storage`}}],CamundaClusterClaim:[{metadata:{name:`orders`,deletionTimestamp:`2026-11-15T10:00:00Z`}}]}),R={name:`Keep the resources`},z={name:`Delete the resources, with the bucket warning`,args:{model:C(F(),`Delete`),policyStep:!0}},B={name:`A claim on Keep with an option that cannot keep`,args:{model:C(I,`Delete`)}},V={name:`The requests could not be read`,args:{model:C(F(),`Orphan`,[`PluginRequest`]),errors:[{kind:`PluginRequest`,message:`pluginrequests.plugins.camunda.io is forbidden`}]}},H={name:`Git: the delete was refused after the policy commit`,args:{mode:`git`,stage:{kind:`choose`,errors:[{code:`Stale`,message:`The claim orders changed since you opened it.`}],afterPolicyWrite:{held:`Delete`,unapplied:`Orphan`}}}},U={name:`Keep picked, and an option cannot keep`,args:{model:C(I,`Orphan`)}},W={name:`The Optimize objects could not be read`,args:{model:C(F(),`Orphan`,[`CamundaOptimize`]),errors:[{kind:`CamundaOptimize`,message:`camundaoptimizes.core.camunda.io is forbidden`}]}},Ye=F({claim:A(void 0,{cluster:void 0}),cluster:O(`CamundaCluster`,{metadata:{namespace:`team-orders`,name:`orders`}})}),G={name:`A cluster that the claim did not make`,args:{model:C(Ye,`Orphan`)}},K={name:`The claim is deleted, and its cluster stays`,args:{model:C(Ye,`Orphan`),stage:{kind:`deleted`}}},q={name:`Waiting for the operator to read the deletion policy`,args:{stage:{kind:`waiting`}}},J={name:`The cluster and its requests going`,args:{stage:{kind:`deleting`,progress:Je,gitWaiting:!1}}},Y={name:`Git: waiting for the deletion policy`,args:{mode:`git`,stage:{kind:`waiting`}}},X={name:`Git: waiting for GitOps to delete the claim`,args:{mode:`git`,stage:{kind:`deleting`,progress:Oe(L.targets,{CamundaOptimize:[{metadata:{name:`orders-optimize`}}],BackupSchedule:[{metadata:{name:`orders-backups`}}],CamundaCluster:[{metadata:{name:`orders`}}],PluginRequest:[{metadata:{name:`orders-access`}},{metadata:{name:`orders-backup-storage`}},{metadata:{name:`orders-secondary-storage`}}],CamundaClusterClaim:[{metadata:{name:`orders`}}]}),gitWaiting:!0}}},Z={name:`The cluster is deleted`,args:{stage:{kind:`deleted`}}},Q={name:`The claim was made again`,args:{stage:{kind:`replaced`}}},$={name:`Changes are off`,args:{mode:void 0}},Xe=[`KeepTheResources`,`DeleteTheResources`,`KeepRefused`,`RequestsNotRead`,`PolicyWritten`,`KeepPickRefused`,`AttachmentsNotRead`,`ClusterNotMade`,`ClusterNotMadeDeleted`,`DirectPolicyWait`,`DirectProgress`,`GitPolicyWait`,`GitDeleteWait`,`Deleted`,`Replaced`,`ChangesOff`],R.parameters={...R.parameters,docs:{...R.parameters?.docs,source:{originalSource:`{
  name: "Keep the resources"
}`,...R.parameters?.docs?.source}}},z.parameters={...z.parameters,docs:{...z.parameters?.docs,source:{originalSource:`{
  name: "Delete the resources, with the bucket warning",
  args: {
    model: deleteClusterModel(data(), "Delete"),
    policyStep: true
  }
}`,...z.parameters?.docs?.source}}},B.parameters={...B.parameters,docs:{...B.parameters?.docs,source:{originalSource:`{
  name: "A claim on Keep with an option that cannot keep",
  args: {
    model: deleteClusterModel(refused, "Delete")
  }
}`,...B.parameters?.docs?.source}}},V.parameters={...V.parameters,docs:{...V.parameters?.docs,source:{originalSource:`{
  name: "The requests could not be read",
  args: {
    model: deleteClusterModel(data(), "Orphan", ["PluginRequest"]),
    errors: [{
      kind: "PluginRequest",
      message: "pluginrequests.plugins.camunda.io is forbidden"
    }]
  }
}`,...V.parameters?.docs?.source}}},H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
  name: "Git: the delete was refused after the policy commit",
  args: {
    mode: "git",
    stage: {
      kind: "choose",
      errors: [{
        code: "Stale",
        message: "The claim orders changed since you opened it."
      }],
      afterPolicyWrite: {
        held: "Delete",
        unapplied: "Orphan"
      }
    }
  }
}`,...H.parameters?.docs?.source}}},U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
  name: "Keep picked, and an option cannot keep",
  args: {
    model: deleteClusterModel(refused, "Orphan")
  }
}`,...U.parameters?.docs?.source}}},W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "The Optimize objects could not be read",
  args: {
    model: deleteClusterModel(data(), "Orphan", ["CamundaOptimize"]),
    errors: [{
      kind: "CamundaOptimize",
      message: "camundaoptimizes.core.camunda.io is forbidden"
    }]
  }
}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "A cluster that the claim did not make",
  args: {
    model: deleteClusterModel(notMade, "Orphan")
  }
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "The claim is deleted, and its cluster stays",
  args: {
    model: deleteClusterModel(notMade, "Orphan"),
    stage: {
      kind: "deleted"
    }
  }
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "Waiting for the operator to read the deletion policy",
  args: {
    stage: {
      kind: "waiting"
    }
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "The cluster and its requests going",
  args: {
    stage: {
      kind: "deleting",
      progress,
      gitWaiting: false
    }
  }
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Git: waiting for the deletion policy",
  args: {
    mode: "git",
    stage: {
      kind: "waiting"
    }
  }
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Git: waiting for GitOps to delete the claim",
  args: {
    mode: "git",
    stage: {
      kind: "deleting",
      progress: deleteProgress(keep.targets, {
        CamundaOptimize: [{
          metadata: {
            name: "orders-optimize"
          }
        }],
        BackupSchedule: [{
          metadata: {
            name: "orders-backups"
          }
        }],
        CamundaCluster: [{
          metadata: {
            name: "orders"
          }
        }],
        PluginRequest: [{
          metadata: {
            name: "orders-access"
          }
        }, {
          metadata: {
            name: "orders-backup-storage"
          }
        }, {
          metadata: {
            name: "orders-secondary-storage"
          }
        }],
        CamundaClusterClaim: [{
          metadata: {
            name: "orders"
          }
        }]
      }),
      gitWaiting: true
    }
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "The cluster is deleted",
  args: {
    stage: {
      kind: "deleted"
    }
  }
}`,...Z.parameters?.docs?.source}}},Q.parameters={...Q.parameters,docs:{...Q.parameters?.docs,source:{originalSource:`{
  name: "The claim was made again",
  args: {
    stage: {
      kind: "replaced"
    }
  }
}`,...Q.parameters?.docs?.source}}},$.parameters={...$.parameters,docs:{...$.parameters?.docs,source:{originalSource:`{
  name: "Changes are off",
  args: {
    mode: undefined
  }
}`,...$.parameters?.docs?.source}}}})))()}Ze();export{W as AttachmentsNotRead,$ as ChangesOff,G as ClusterNotMade,K as ClusterNotMadeDeleted,z as DeleteTheResources,Z as Deleted,q as DirectPolicyWait,J as DirectProgress,X as GitDeleteWait,Y as GitPolicyWait,U as KeepPickRefused,B as KeepRefused,R as KeepTheResources,H as PolicyWritten,Q as Replaced,V as RequestsNotRead,Xe as __namedExportsOrder,qe as default};