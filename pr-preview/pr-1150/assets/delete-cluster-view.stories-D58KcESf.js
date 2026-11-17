import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{b as r,c as i}from"./labels-CoLYcJ9C.js";import{n as a,t as o}from"./circle-exclamation-point-jpEoxFxv.js";import{t as s,u as c}from"./policy-ClW2tmwE.js";import{b as l,t as u}from"./merge-CPpqnaQp.js";import{n as d,t as f}from"./link-DOLMGIR2.js";import{a as p,c as m,i as h,n as g,o as _,r as v,s as y,t as b}from"./table-Bas-jI5j.js";import{n as x,t as ee}from"./badge-DLantns8.js";import{a as te,n as ne}from"./deletion-choice-PZ6Sq2-t.js";import{a as re,i as ie,r as ae,t as oe}from"./deletion-Dh19j7ZX.js";import{n as se,t as ce}from"./change-review-DyQBfLt3.js";import{n as le,t as ue}from"./precheck-line-DQC9DKcU.js";import{a as de,r as fe}from"./rows-Dqg_vML3.js";import{i as pe,r as me}from"./waiting-panel-fEihCLDW.js";import{c as he,n as ge}from"./cluster-edit-Bk4C7tir.js";function _e(e){let{claim:t}=e;return c({claim:{...t,spec:{...t.spec,plugins:t.spec?.plugins}},environment:ve(e),sections:e.sections,registrations:e.registrations,kind:`CamundaClusterClaim`})}function ve(e){let t=e.namespace?.metadata?.labels?.[i];return t===void 0?void 0:e.environments.find(e=>e.metadata?.name===t)}function S(e,t,n=[]){let{claim:r}=e,i=r.metadata?.name??``,a=r.metadata?.namespace??``,o=_e(e),s=r.metadata?.uid,c=e=>s!==void 0&&(e.metadata?.ownerReferences??[]).some(e=>e.controller===!0&&e.uid===s),l=e.requests.filter(c),u=ie({claim:r,sections:e.sections,registrations:e.registrations,requests:l,policy:t}),d=e=>e.spec.clusterRef.name===i,f=(e,t,n)=>({kind:e,name:t,label:`${e} ${t}`,...n&&{note:n}}),p=(e,t,n)=>{let r=e=>e.metadata?.name===n&&c(e),i=t.filter(e=>d(e)||e.metadata?.name===n);return{goes:i.filter(r).map(t=>f(e,t.metadata?.name??``)),stays:i.filter(e=>!r(e)).map(t=>f(e,t.metadata?.name??``,Ce))}},m=p(`CamundaOptimize`,e.optimizes,`${i}-optimize`),h=p(`BackupSchedule`,e.schedules,`${i}-backups`),g=!n.some(e=>Se.includes(e)),_=!n.some(e=>xe.includes(e)),v=e.cluster?c(e.cluster):r.status?.cluster?.name===i,y=!v&&e.cluster?[f(`CamundaCluster`,i,Ce)]:[],b=[...m.goes,...h.goes,...v?[f(`CamundaCluster`,i,g?e.cluster&&ye(e.cluster,e):`Volumes not known: the page could not read what decides them`)]:[],...l.map(e=>{let t=e.metadata?.name??``,n=r.status?.requests?.find(e=>e.name===t)?.section??e.spec?.section,i=u.find(e=>e.section===n)?.label;return{...f(`PluginRequest`,t),...i&&{label:`${i} request ${t}`}}}),f(`CamundaClusterClaim`,i)],x=_?oe(i,u):void 0;return{name:i,namespace:a,choice:{...o,policy:t},keepRefused:o.policy===`Orphan`&&!o.keepAllowed,targets:b,kept:[...y,...m.stays,...h.stays],groups:u,backups:e.backups.elasticsearch.filter(d).length+e.backups.rdbms.filter(d).length,...x&&{warning:x},unread:[...n],fatesKnown:_}}function ye(e,t){let n=e.metadata?.generation,r=t=>e.status?.conditions?.find(e=>e.type===t&&e.observedGeneration===n),i=r(`Ready`),a=r(`ZeebeReady`);if(n===void 0||e.status?.observedGeneration!==n||!i||!a)return`Volumes not known: the operator has not applied the current spec of the cluster yet`;if(we.includes(i.reason)||Te.includes(a.reason))return`Volumes not known: the Ready condition of the cluster reports a problem that can block its spec`;let{spec:o}=e,s=o?.zeebe?.persistentVolumeClaimRetentionPolicy?.whenDeleted,c=o&&l(t.presets.find(e=>e.metadata?.name===o.presetRef)?.spec,t.releases.find(e=>e.metadata?.name===o.releaseRef)?.spec,o).spec.zeebe?.persistentVolumeClaimRetentionPolicy?.whenDeleted;return!s&&o?.presetRef?`Volumes not known: the cluster takes its volume policy from its preset, and the page cannot tell if the operator applied it`:c===`Retain`?`Volumes kept for a cluster of the same name`:`Volumes deleted`}function be(e,t,n=[]){return e.map(e=>{if(n.includes(e.kind))return{target:e,state:`Not known`};let r=t[e.kind]?.find(t=>t.metadata?.name===e.name);return{target:e,state:r===void 0?`Deleted`:r.metadata?.deletionTimestamp?`Deleting`:`Waiting`}})}var xe,Se,Ce,we,Te;function Ee(){return(Ee=e((()=>{re(),r(),u(),s(),xe=[`PluginRequest`,`PluginRegistration`,`PluginSection`],Se=[`CamundaCluster`,`CamundaClusterPreset`,`CamundaRelease`],Ce=`Kept: the claim did not make it`,we=[`InvalidReference`,`MissingSecret`,`ConnectionFailed`,`VersionDowngradeRefused`,`VersionSkipRefused`],Te=[`Unknown`,`Error`,`FeatureGateError`,`PrerequisiteNotMet`,`Blocked`]})))()}function De({model:e,mode:t,stage:n,errors:r,remove:i,policyStep:a,keepRefused:o,onPick:s,preview:c,apply:l,show:u,onWritten:f,onStopWaiting:p}){let{name:m,namespace:h}=e,g=`/fleet/clusters/${h}/${m}`,_=Oe(e,t===`git`&&a),v=e.kept.some(e=>e.kind===`CamundaCluster`),[y,b]=(0,Ne.useState)(!1),x=(0,w.jsx)(`section`,{"aria-label":`What the delete takes`,children:(0,w.jsx)(`ul`,{className:`flex list-disc flex-col gap-1 pl-5 text-sm text-text-2`,children:_.map((e,t)=>(0,w.jsx)(`li`,{children:e},t))})});return(0,w.jsxs)(`div`,{className:`flex max-w-3xl min-w-0 flex-col gap-6`,children:[(0,w.jsxs)(`div`,{className:`flex flex-col gap-2`,children:[(0,w.jsx)(d,{href:g,className:`relative w-fit text-sm text-muted hit-area hover:text-text`,children:`Back to the cluster`}),(0,w.jsxs)(`h1`,{className:`font-heading text-2xl font-bold tracking-[-0.02em] break-all`,children:[`Delete the cluster `,m]})]}),n.kind===`deleted`?(0,w.jsxs)(`div`,{className:`flex flex-col gap-2 text-sm`,children:[(0,w.jsx)(`p`,{role:`status`,className:`text-text`,children:v?`The claim ${m} is deleted. The cluster ${m} stays: the claim did not make it.`:`The cluster ${m} is deleted.`}),(0,w.jsx)(d,{href:`/fleet`,className:`relative w-fit text-accent hit-area hover:underline`,children:`Back to the fleet`})]}):n.kind===`replaced`?(0,w.jsxs)(`div`,{className:`flex flex-col gap-2 text-sm`,children:[(0,w.jsxs)(`p`,{role:`status`,className:`text-text`,children:[`The claim `,m,` was made again after this page opened. Open the delete page again to delete the new claim.`]}),(0,w.jsx)(d,{href:g,className:`relative w-fit text-accent hit-area hover:underline`,children:`Back to the cluster`})]}):n.kind===`waiting`?(0,w.jsxs)(w.Fragment,{children:[(0,w.jsx)(C,{errors:r}),n.blocked&&(0,w.jsx)(`p`,{role:`status`,className:`text-sm text-warn`,children:n.blocked}),(0,w.jsx)(me,{text:t===`git`?`Waiting for GitOps to apply the deletion policy. The operator then reads it, and the fleet manager removes the claim file.`:`Waiting for the operator to read the deletion policy. The fleet manager then deletes the claim.`,onStopWaiting:p})]}):n.kind===`deleting`?(0,w.jsxs)(w.Fragment,{children:[(0,w.jsx)(C,{errors:r}),(0,w.jsx)(Ae,{name:m,stage:n})]}):(0,w.jsxs)(w.Fragment,{children:[(0,w.jsx)(ne,{subject:`cluster`,choice:e.choice,onChange:s,disabled:y}),e.keepRefused&&e.choice.policy!==`Orphan`&&(0,w.jsx)(`p`,{className:`-mt-3 text-sm text-warn`,children:`The claim keeps its resources, but an option below cannot keep them. The delete removes them.`}),t===`git`&&(0,w.jsx)(`p`,{className:`text-sm text-text-2`,children:T}),(0,w.jsx)(C,{errors:r}),n.errors&&n.errors.length>0&&(0,w.jsx)(Me,{texts:n.errors.map(e=>e.message)}),n.afterPolicyWrite&&(0,w.jsx)(`p`,{role:`status`,className:`text-sm text-warn`,children:je(m,n.afterPolicyWrite)}),t===void 0?(0,w.jsxs)(w.Fragment,{children:[(0,w.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:he}),x]}):e.unread.length>0&&!y?(0,w.jsxs)(w.Fragment,{children:[(0,w.jsx)(`p`,{className:`text-sm text-muted`,children:de(e.unread)}),x]}):o&&!y?(0,w.jsxs)(w.Fragment,{children:[(0,w.jsx)(`p`,{className:`text-sm text-warn`,children:o}),x]}):(0,w.jsx)(ce,{change:i,mode:t,diffLabel:`The claim that the delete removes`,effects:_,caution:e.warning,confirm:{label:`Delete the cluster`,gitLabel:`Push the delete to Git`,danger:!0,typed:m},savedMessage:t===`direct`?a?`Set the deletion policy of ${m}. The delete follows when the operator reads it.`:`The claim ${m} is being deleted.`:a?`Pushed the deletion policy of ${m}. The delete follows when GitOps applies it.`:`Pushed the delete of ${m}.`,preview:c,apply:l,show:u,onWritten:f,onWriting:b})]})]})}function Oe(e,t){let n=e.unread.filter(e=>Pe.includes(e)),r=e=>!n.includes(e.kind),i=e.targets.filter(e=>e.kind!==`PluginRequest`&&r(e)),a=e.kept.filter(r),o=[...e.targets,...e.kept].filter(e=>!r(e)),s=(e,t)=>e.map(e=>(0,w.jsxs)(`li`,{children:[e.label,(t??e.note)&&(0,w.jsxs)(`span`,{className:`text-muted`,children:[`: `,t??e.note]})]},`${e.kind}/${e.name}`));return[(0,w.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,w.jsx)(`span`,{className:`font-medium text-text`,children:`Always deleted`}),(0,w.jsx)(`ul`,{className:`flex flex-col gap-0.5`,children:s(i)})]},`always`),...a.length>0?[(0,w.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,w.jsx)(`span`,{className:`font-medium text-text`,children:`Kept`}),(0,w.jsx)(`ul`,{className:`flex flex-col gap-0.5`,children:s(a)})]},`kept`)]:[],...n.length>0?[(0,w.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,w.jsx)(`span`,{className:`font-medium text-text`,children:`Not known`}),(0,w.jsxs)(`ul`,{className:`flex flex-col gap-0.5`,children:[s(o,`not known`),n.filter(e=>!o.some(t=>t.kind===e)).map(e=>(0,w.jsxs)(`li`,{children:[e,(0,w.jsx)(`span`,{className:`text-muted`,children:`: not known`})]},e))]})]},`unknown`)]:[],...e.groups.map(t=>(0,w.jsx)(ke,{group:t,known:e.fatesKnown},t.id)),...e.backups>0?[`The ${e.backups} backups of ${e.name} stay, and their artifacts stay in the bucket.`]:[],...t?[`Pushes two commits: the deletion policy first, then the delete once GitOps applied it.`]:[]]}function ke({group:e,known:t}){let n=e.id.includes(`/`)?`${e.label} (${e.registration})`:e.label;return(0,w.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-1`,children:[e.rows.length===0?(0,w.jsxs)(w.Fragment,{children:[(0,w.jsx)(`span`,{className:`font-medium text-text`,children:n}),(0,w.jsx)(`span`,{children:t?`This option made no resources.`:`What this option made is not known.`})]}):(0,w.jsxs)(b,{label:`Resources of ${n}`,children:[(0,w.jsx)(v,{className:`mt-0 mb-1 caption-top text-left font-medium text-text`,children:n}),(0,w.jsx)(_,{children:(0,w.jsxs)(y,{children:[(0,w.jsx)(p,{children:`Kind`}),(0,w.jsx)(p,{children:`Name`}),(0,w.jsx)(p,{children:`ID`}),(0,w.jsx)(p,{children:`What happens`})]})}),(0,w.jsx)(g,{children:e.rows.map(e=>(0,w.jsxs)(y,{children:[(0,w.jsx)(h,{children:e.kind}),(0,w.jsx)(h,{className:`break-all whitespace-normal`,children:e.name}),(0,w.jsx)(h,{className:`break-all whitespace-normal text-muted`,children:e.id??`-`}),(0,w.jsx)(h,{children:t?(0,w.jsx)(ee,{variant:e.fate===`Deleted`?`destructive`:`outline`,className:e.fate===`Kept`?`text-muted`:void 0,children:e.fate}):(0,w.jsx)(ee,{variant:`outline`,children:`Not known`})})]},`${e.kind}/${e.name}`))})]}),e.alwaysDeletes&&(0,w.jsx)(`span`,{className:`text-warn`,children:`This option always deletes what it made.`})]})}function Ae({name:e,stage:t}){return(0,w.jsxs)(`section`,{"aria-label":`Delete progress`,className:`flex flex-col gap-3`,children:[t.gitWaiting&&(0,w.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[(0,w.jsxs)(`p`,{role:`status`,className:`text-text`,children:[`Waiting for GitOps to delete `,e]}),(0,w.jsx)(`p`,{className:`text-text-2`,children:T})]}),t.claim&&(0,w.jsx)(ue,{claim:t.claim}),t.keepRefused&&(0,w.jsx)(`p`,{className:`text-sm text-warn`,children:t.keepRefused}),(0,w.jsx)(`ol`,{className:`flex flex-col divide-y divide-divider rounded-md border border-border`,children:t.progress.map(({target:e,state:t})=>(0,w.jsxs)(`li`,{className:`flex flex-wrap items-center justify-between gap-x-4 gap-y-1 px-3 py-2 text-sm`,children:[(0,w.jsxs)(`span`,{className:`flex min-w-0 flex-col`,children:[(0,w.jsx)(`span`,{className:`break-all text-text`,children:e.label}),e.note&&(0,w.jsx)(`span`,{className:`text-muted`,children:e.note})]}),(0,w.jsx)(ee,{variant:t===`Waiting`||t===`Not known`?`outline`:`secondary`,className:t===`Deleted`?`text-muted`:void 0,children:t})]},`${e.kind}/${e.name}`))})]})}function je(e,t){let n=e=>e===`Orphan`?`Keep`:`Delete`,r=`The claim ${e} has the deletion policy ${n(t.held)}.`;return t.unapplied?`The cluster is not deleted. The repository holds the commit that sets the deletion policy of ${e} to ${n(t.unapplied)}, and GitOps has not applied it yet. ${r}`:`The cluster is not deleted. ${r}`}function C({errors:e}){return e.map((e,t)=>(0,w.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},t))}function Me({texts:e}){return(0,w.jsxs)(`div`,{role:`alert`,className:`flex gap-3 rounded-md border border-bad/40 bg-bad/10 p-3 text-sm text-text`,children:[(0,w.jsx)(o,{"aria-hidden":!0,className:`mt-0.5 size-4 shrink-0 text-bad`}),(0,w.jsx)(`ul`,{className:`flex flex-col gap-1`,children:e.map((e,t)=>(0,w.jsx)(`li`,{children:e},t))})]})}var w,Ne,T,Pe;function Fe(){return(Fe=e((()=>{w=n(),a(),f(),Ne=t(),se(),te(),le(),fe(),x(),m(),ge(),pe(),T=`Your GitOps tool must prune: the fleet manager removes the claim file, and GitOps deletes the claim.`,Pe=[`CamundaCluster`,`CamundaOptimize`,`BackupSchedule`],De.__docgenInfo={description:`DeleteClusterView is the delete page of a cluster. It never writes: the
confirm of its review calls apply with remove, and the review previews
remove once for each remove object. Without a mode it shows CHANGES_OFF
and what the delete takes, and no review. While model.unread has a kind
it shows what the delete takes, and no review, unless a write runs.
While keepRefused is set, a mode is set, and model.unread is empty, it
shows keepRefused in place of the review, unless a write runs.
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
| { kind: "waiting"; blocked?: string }
| {
    kind: "deleting";
    progress: { target: DeleteTarget; state: DeleteState }[];
    gitWaiting: boolean;
    claim?: Resource<"CamundaClusterClaim">;
    keepRefused?: string;
  }
| { kind: "deleted" }
| { kind: "replaced" }`,elements:[{name:`signature`,type:`object`,raw:`{
  kind: "choose";
  errors?: ApiError[];
  afterPolicyWrite?: PolicyAfterWrite;
}`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"choose"`,required:!0}},{key:`errors`,value:{name:`Array`,elements:[{name:`ApiError`}],raw:`ApiError[]`,required:!1}},{key:`afterPolicyWrite`,value:{name:`PolicyAfterWrite`,required:!1}}]}},{name:`signature`,type:`object`,raw:`{ kind: "waiting"; blocked?: string }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"waiting"`,required:!0}},{key:`blocked`,value:{name:`string`,required:!1}}]}},{name:`signature`,type:`object`,raw:`{
  kind: "deleting";
  progress: { target: DeleteTarget; state: DeleteState }[];
  gitWaiting: boolean;
  claim?: Resource<"CamundaClusterClaim">;
  keepRefused?: string;
}`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"deleting"`,required:!0}},{key:`progress`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ target: DeleteTarget; state: DeleteState }`,signature:{properties:[{key:`target`,value:{name:`signature`,type:`object`,raw:`{
  kind: KindName;
  name: string;
  label: string;
  note?: string;
}`,signature:{properties:[{key:`kind`,value:{name:`KindName`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`note`,value:{name:`string`,required:!1}}]},required:!0}},{key:`state`,value:{name:`union`,raw:`"Waiting" | "Deleting" | "Deleted" | "Not known"`,elements:[{name:`literal`,value:`"Waiting"`},{name:`literal`,value:`"Deleting"`},{name:`literal`,value:`"Deleted"`},{name:`literal`,value:`"Not known"`}],required:!0}}]}}],raw:`{ target: DeleteTarget; state: DeleteState }[]`,required:!0}},{key:`gitWaiting`,value:{name:`boolean`,required:!0}},{key:`claim`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaClusterClaim"`}],raw:`Resource<"CamundaClusterClaim">`,required:!1}},{key:`keepRefused`,value:{name:`string`,required:!1}}]}},{name:`signature`,type:`object`,raw:`{ kind: "deleted" }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"deleted"`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{ kind: "replaced" }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"replaced"`,required:!0}}]}}]},description:``},errors:{required:!0,tsType:{name:`unknown`},description:``},remove:{required:!0,tsType:{name:`Change`},description:``},policyStep:{required:!0,tsType:{name:`boolean`},description:``},keepRefused:{required:!1,tsType:{name:`string`},description:``}}}})))()}var Ie,E,D,Le,O,Re,k,A,j,M,N,ze,Be,P,Ve,F,He,Ue,We,I,L,R,z,B,V,H,U,Ge,W,G,K,q,J,Y,X,Z,Q,$,Ke;function qe(){return(qe=e((()=>{Ie=n(),re(),Ee(),Fe(),{fn:E}=__STORYBOOK_MODULE_TEST__,D=(e,t)=>t,Le=`8c1f6c1e-1d1a-4c5e-9d43-0f3b1c2d4e5f`,O={ownerReferences:[{controller:!0,uid:Le}]},Re=[{section:`access`,registration:`gateway`},{section:`backup-storage`,registration:`aws-s3`},{section:`secondary-storage`,registration:`elasticsearch`}],k=(e,t={})=>D(`CamundaClusterClaim`,{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{namespace:`team-orders`,name:`orders`,resourceVersion:`41`,uid:Le},spec:{releaseRef:`camunda-8-9-1`,plugins:Re,deletionPolicy:e},status:{resolvedPolicy:{deletionPolicy:`Orphan`},requests:Re.map(e=>({...e,name:`orders-${e.section}`})),cluster:{name:`orders`},...t}}),A=(e,t,n,r)=>D(`PluginSection`,{metadata:{name:e},spec:{label:t,order:n,selection:`One`,appliesTo:[`CamundaClusterClaim`],...r&&{slot:r}}}),j=(e,t,n,r)=>D(`PluginRegistration`,{metadata:{name:e},spec:{label:t,sections:n,deletionPolicies:r}}),M=(e,t)=>D(`PluginRequest`,{metadata:{namespace:`team-orders`,name:e,...O},status:{resources:t}}),N=e=>({spec:{clusterRef:{name:e}}}),ze=(e=`True`,t=`Healthy`)=>({observedGeneration:4,conditions:[{type:`Ready`,status:e,reason:t,observedGeneration:4},{type:`ZeebeReady`,status:`True`,reason:`Healthy`,observedGeneration:4}]}),Be=e=>D(`CamundaCluster`,{metadata:{namespace:`team-orders`,name:`orders`,generation:4,...O},spec:{zeebe:{persistentVolumeClaimRetentionPolicy:{whenDeleted:`Retain`}}},status:e}),P=(e={})=>({claim:k(),cluster:Be(ze()),environments:[],sections:[A(`access`,`Access`,10,`externalAccess`),A(`backup-storage`,`Backup storage`,20,`backupStorage`),A(`secondary-storage`,`Secondary storage`,30,`secondaryStorage`)],registrations:[j(`gateway`,`Gateway`,[`access`],[`Delete`,`Orphan`]),j(`aws-s3`,`Amazon S3`,[`backup-storage`],[`Delete`,`Orphan`]),j(`elasticsearch`,`Elasticsearch`,[`secondary-storage`],[`Delete`,`Orphan`])],requests:[M(`orders-access`,[{kind:`HTTPRoute`,name:`orders`,onOrphan:`Deleted`},{kind:`Certificate`,name:`orders-tls`,onOrphan:`Kept`}]),M(`orders-backup-storage`,[{kind:`S3Bucket`,name:`orders-backups-7f3a`,id:`arn:aws:s3:::orders-backups-7f3a`,onOrphan:`Kept`}]),M(`orders-secondary-storage`,[{kind:`ElasticsearchCluster`,name:`orders-es`,onOrphan:`Kept`}])],optimizes:[D(`CamundaOptimize`,{metadata:{namespace:`team-orders`,name:`orders-optimize`,...O},...N(`orders`)})],schedules:[D(`BackupSchedule`,{metadata:{namespace:`team-orders`,name:`orders-backups`,...O},...N(`orders`)}),D(`BackupSchedule`,{metadata:{namespace:`team-orders`,name:`orders-hourly`},...N(`orders`)})],backups:{elasticsearch:[`orders-1`,`orders-2`,`orders-3`].map(e=>D(`LogicalBackupElasticsearch`,{metadata:{namespace:`team-orders`,name:e},...N(`orders`)})),rdbms:[]},presets:[],releases:[],errors:[],...e}),Ve=P({claim:k(`Orphan`),registrations:[j(`gateway`,`Gateway`,[`access`],[`Delete`]),j(`aws-s3`,`Amazon S3`,[`backup-storage`],[`Delete`,`Orphan`]),j(`elasticsearch`,`Elasticsearch`,[`secondary-storage`],[`Delete`,`Orphan`])]}),F=S(P(),`Orphan`),He=ae(P().claim,`Orphan`,`Delete the cluster orders`),Ue={title:`cluster/DeleteClusterView`,component:De,args:{model:F,mode:`direct`,stage:{kind:`choose`},errors:[],remove:He.remove,policyStep:!1,onPick:E(),preview:E(async()=>({objects:[],errors:[]})),apply:E(async()=>({ok:!0,mode:`direct`})),show:E(),onWritten:E(),onStopWaiting:E()},decorators:[e=>(0,Ie.jsx)(`div`,{className:`max-w-3xl`,children:(0,Ie.jsx)(e,{})})]},We=be(F.targets,{CamundaOptimize:[],BackupSchedule:[],CamundaCluster:[{metadata:{name:`orders`,deletionTimestamp:`2026-11-15T10:00:00Z`}}],PluginRequest:[{metadata:{name:`orders-access`}},{metadata:{name:`orders-backup-storage`}},{metadata:{name:`orders-secondary-storage`}}],CamundaClusterClaim:[{metadata:{name:`orders`,deletionTimestamp:`2026-11-15T10:00:00Z`}}]}),I={name:`Keep the resources`},L={name:`Delete the resources, with the bucket warning`,args:{model:S(P(),`Delete`),policyStep:!0}},R={name:`A claim on Keep with an option that cannot keep`,args:{model:S(Ve,`Delete`)}},z={name:`The requests could not be read`,args:{model:S(P(),`Orphan`,[`PluginRequest`]),errors:[{kind:`PluginRequest`,message:`pluginrequests.plugins.camunda.io is forbidden`}]}},B={name:`Git: the delete was refused after the policy commit`,args:{mode:`git`,stage:{kind:`choose`,errors:[{code:`Stale`,message:`The claim orders changed since you opened it.`}],afterPolicyWrite:{held:`Delete`,unapplied:`Orphan`}}}},V={name:`Keep picked, and an option cannot keep`,args:{model:S(Ve,`Orphan`),keepRefused:`PluginRegistration "gateway" of section "access" cannot keep the resources. Choose Delete the resources.`}},H={name:`The Ready condition of the cluster reports a problem`,args:{model:S(P({cluster:Be(ze(`False`,`InvalidReference`))}),`Orphan`)}},U={name:`The Optimize objects could not be read`,args:{model:S(P(),`Orphan`,[`CamundaOptimize`]),errors:[{kind:`CamundaOptimize`,message:`camundaoptimizes.core.camunda.io is forbidden`}]}},Ge=P({claim:k(void 0,{cluster:void 0}),cluster:D(`CamundaCluster`,{metadata:{namespace:`team-orders`,name:`orders`}})}),W={name:`A cluster that the claim did not make`,args:{model:S(Ge,`Orphan`)}},G={name:`The claim is deleted, and its cluster stays`,args:{model:S(Ge,`Orphan`),stage:{kind:`deleted`}}},K={name:`Waiting for the operator to read the deletion policy`,args:{stage:{kind:`waiting`}}},q={name:`The cluster and its requests going`,args:{stage:{kind:`deleting`,progress:We,gitWaiting:!1}}},J={name:`The operator refuses the delete, because an option cannot keep`,args:{stage:{kind:`deleting`,progress:We,gitWaiting:!1,keepRefused:`The delete waits, because PluginRegistration "gateway" of section "access" cannot keep the resources. Set the deletion policy of the claim to Delete, or add Orphan back to the deletion policies of the registration.`}}},Y={name:`Git: waiting for the deletion policy`,args:{mode:`git`,stage:{kind:`waiting`}}},X={name:`Git: waiting for GitOps to delete the claim`,args:{mode:`git`,stage:{kind:`deleting`,progress:be(F.targets,{CamundaOptimize:[{metadata:{name:`orders-optimize`}}],BackupSchedule:[{metadata:{name:`orders-backups`}}],CamundaCluster:[{metadata:{name:`orders`}}],PluginRequest:[{metadata:{name:`orders-access`}},{metadata:{name:`orders-backup-storage`}},{metadata:{name:`orders-secondary-storage`}}],CamundaClusterClaim:[{metadata:{name:`orders`}}]}),gitWaiting:!0}}},Z={name:`The cluster is deleted`,args:{stage:{kind:`deleted`}}},Q={name:`The claim was made again`,args:{stage:{kind:`replaced`}}},$={name:`Changes are off`,args:{mode:void 0}},Ke=[`KeepTheResources`,`DeleteTheResources`,`KeepRefused`,`RequestsNotRead`,`PolicyWritten`,`KeepPickRefused`,`VolumesNotApplied`,`AttachmentsNotRead`,`ClusterNotMade`,`ClusterNotMadeDeleted`,`DirectPolicyWait`,`DirectProgress`,`DeleteKeepRefused`,`GitPolicyWait`,`GitDeleteWait`,`Deleted`,`Replaced`,`ChangesOff`],I.parameters={...I.parameters,docs:{...I.parameters?.docs,source:{originalSource:`{
  name: "Keep the resources"
}`,...I.parameters?.docs?.source}}},L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
  name: "Delete the resources, with the bucket warning",
  args: {
    model: deleteClusterModel(data(), "Delete"),
    policyStep: true
  }
}`,...L.parameters?.docs?.source}}},R.parameters={...R.parameters,docs:{...R.parameters?.docs,source:{originalSource:`{
  name: "A claim on Keep with an option that cannot keep",
  args: {
    model: deleteClusterModel(refused, "Delete")
  }
}`,...R.parameters?.docs?.source}}},z.parameters={...z.parameters,docs:{...z.parameters?.docs,source:{originalSource:`{
  name: "The requests could not be read",
  args: {
    model: deleteClusterModel(data(), "Orphan", ["PluginRequest"]),
    errors: [{
      kind: "PluginRequest",
      message: "pluginrequests.plugins.camunda.io is forbidden"
    }]
  }
}`,...z.parameters?.docs?.source}}},B.parameters={...B.parameters,docs:{...B.parameters?.docs,source:{originalSource:`{
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
}`,...B.parameters?.docs?.source}}},V.parameters={...V.parameters,docs:{...V.parameters?.docs,source:{originalSource:`{
  name: "Keep picked, and an option cannot keep",
  args: {
    model: deleteClusterModel(refused, "Orphan"),
    keepRefused: 'PluginRegistration "gateway" of section "access" cannot keep the resources. Choose Delete the resources.'
  }
}`,...V.parameters?.docs?.source}}},H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
  name: "The Ready condition of the cluster reports a problem",
  args: {
    model: deleteClusterModel(data({
      cluster: retaining(reconciled("False", "InvalidReference"))
    }), "Orphan")
  }
}`,...H.parameters?.docs?.source}}},U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
  name: "The Optimize objects could not be read",
  args: {
    model: deleteClusterModel(data(), "Orphan", ["CamundaOptimize"]),
    errors: [{
      kind: "CamundaOptimize",
      message: "camundaoptimizes.core.camunda.io is forbidden"
    }]
  }
}`,...U.parameters?.docs?.source}}},W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "A cluster that the claim did not make",
  args: {
    model: deleteClusterModel(notMade, "Orphan")
  }
}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "The claim is deleted, and its cluster stays",
  args: {
    model: deleteClusterModel(notMade, "Orphan"),
    stage: {
      kind: "deleted"
    }
  }
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "Waiting for the operator to read the deletion policy",
  args: {
    stage: {
      kind: "waiting"
    }
  }
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "The cluster and its requests going",
  args: {
    stage: {
      kind: "deleting",
      progress,
      gitWaiting: false
    }
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "The operator refuses the delete, because an option cannot keep",
  args: {
    stage: {
      kind: "deleting",
      progress,
      gitWaiting: false,
      keepRefused: 'The delete waits, because PluginRegistration "gateway" of section "access" cannot keep the resources. Set the deletion policy of the claim to Delete, or add Orphan back to the deletion policies of the registration.'
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
}`,...$.parameters?.docs?.source}}}})))()}qe();export{U as AttachmentsNotRead,$ as ChangesOff,W as ClusterNotMade,G as ClusterNotMadeDeleted,J as DeleteKeepRefused,L as DeleteTheResources,Z as Deleted,K as DirectPolicyWait,q as DirectProgress,X as GitDeleteWait,Y as GitPolicyWait,V as KeepPickRefused,R as KeepRefused,I as KeepTheResources,B as PolicyWritten,Q as Replaced,z as RequestsNotRead,H as VolumesNotApplied,Ke as __namedExportsOrder,Ue as default};