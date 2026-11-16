import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./createLucideIcon-B55dJ-eE.js";var i,a;function o(){return(o=e((()=>{n(),i={name:`info`,size:24,node:[[`circle`,{cx:`12`,cy:`12`,r:`10`,key:`1mglay`}],[`path`,{d:`M12 16v-4`,key:`1dtifu`}],[`path`,{d:`M12 8h.01`,key:`e9boi3`}]]},i.node,a=r(i)})))()}function s(e){return[l(e),...e.unassigned?[`This is the first environment. The claims in ${e.unassigned.join(`, `)} have no environment label, so they take no change until their namespaces join an environment (NoEnvironment). Their clusters keep running.`]:[],...e.warnings.map(u)]}function c({impact:e}){let[t,...n]=s(e);return(0,f.jsxs)(`div`,{role:`note`,"aria-label":`Impact`,className:`flex gap-3 rounded-md border border-info/40 bg-info/10 p-3 text-sm text-text`,children:[(0,f.jsx)(a,{"aria-hidden":!0,className:`mt-0.5 size-4 shrink-0 text-info`}),(0,f.jsxs)(`div`,{className:`flex flex-col gap-1.5`,children:[(0,f.jsx)(`p`,{children:t}),n.length>0&&(0,f.jsx)(`ul`,{className:`flex list-disc flex-col gap-1 pl-5 text-text-2`,children:n.map((e,t)=>(0,f.jsx)(`li`,{children:e},t))})]})]})}function l({clusters:e,planes:t,unread:n}){let r=n?`The fleet manager cannot read ${n.join(`, `)}, so the claims that this change touches can be more than this note names.`:`Changes apply to new claims only.`,i=[...e>0?[d(e,`cluster`,`clusters`)]:[],...t>0?[d(t,`management plane`,`management planes`)]:[]];if(i.length===0)return r;let a=e+t===1;return`${r} ${i.join(` and `)} ${a?`keeps the policy it`:`keep the policy they`} recorded at creation.`}function u(e){let t=`${e.claim.name} in ${e.claim.namespace}`;if(`removed`in e){let[n,r]=p[e.kind],i=e.claim.kind===`CamundaClusterClaim`?`The cluster keeps running.`:`The management plane keeps running.`;return`${t} uses ${n} ${e.name}, which this change no longer allows. ${i} A later change of its ${r} must pick an allowed one.`}if(`cleared`in e){let n=e.kind===`CamundaRelease`?`release`:`platform config`,r=`${t} names no ${n}, and this change removes the default ${n}.`;return e.claim.kind===`CamundaClusterClaim`?`${r} The cluster keeps running, but the claim is not ready and its cluster takes no further change.`:`${r} Its management plane keeps running, but the claim reports MissingInput and takes no further change.`}return e.kind===`CamundaRelease`&&e.refused?`${t} names no release, and its cluster refuses the move to ${e.name} (${e.refused}). The cluster keeps running and takes no further change.`:e.kind===`CamundaRelease`?`${t} names no release and moves to ${e.name}. Its brokers restart one at a time.`:`${t} names no platform config and moves to ${e.name}.`}function d(e,t,n){return`${e} ${e===1?t:n}`}var f,p;function m(){return(m=e((()=>{f=t(),o(),p={CamundaRelease:[`release`,`release`],CamundaClusterPreset:[`cluster preset`,`preset`],ElasticsearchClusterPreset:[`Elasticsearch preset`,`preset`],DatabaseServerPreset:[`PostgreSQL preset`,`preset`]},c.__docgenInfo={description:`ImpactLine shows impactLines as a note above the change bar.`,methods:[],displayName:`ImpactLine`,props:{impact:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  clusters: number;
  planes: number;
  warnings: ReferenceWarning[];
  /**
   * unassigned are the namespaces that hold claims and no environment label
   * when the edit creates the first environment, sorted.
   */
  unassigned?: string[];
  /** unread are the kinds of the lists that could not be read or are stale. */
  unread?: string[];
}`,signature:{properties:[{key:`clusters`,value:{name:`number`,required:!0}},{key:`planes`,value:{name:`number`,required:!0}},{key:`warnings`,value:{name:`Array`,elements:[{name:`union`,raw:`| {
    claim: ClaimRef;
    kind: "CamundaRelease" | "CamundaClusterPreset" | StoragePresetKind;
    name: string;
    removed: true;
  }
| {
    claim: ClaimRef;
    kind: "CamundaRelease" | "CamundaPlatformConfig";
    name: string;
    followsDefault: true;
    refused?: "VersionDowngradeRefused" | "VersionSkipRefused";
  }
| {
    claim: ClaimRef;
    kind: "CamundaRelease" | "CamundaPlatformConfig";
    cleared: true;
  }`,elements:[{name:`signature`,type:`object`,raw:`{
  claim: ClaimRef;
  kind: "CamundaRelease" | "CamundaClusterPreset" | StoragePresetKind;
  name: string;
  removed: true;
}`,signature:{properties:[{key:`claim`,value:{name:`signature`,type:`object`,raw:`{ kind: ClaimKind; namespace: string; name: string }`,signature:{properties:[{key:`kind`,value:{name:`ClaimKind`,required:!0}},{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]},required:!0}},{key:`kind`,value:{name:`union`,raw:`"CamundaRelease" | "CamundaClusterPreset" | StoragePresetKind`,elements:[{name:`literal`,value:`"CamundaRelease"`},{name:`literal`,value:`"CamundaClusterPreset"`},{name:`unknown`}],required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`removed`,value:{name:`literal`,value:`true`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{
  claim: ClaimRef;
  kind: "CamundaRelease" | "CamundaPlatformConfig";
  name: string;
  followsDefault: true;
  refused?: "VersionDowngradeRefused" | "VersionSkipRefused";
}`,signature:{properties:[{key:`claim`,value:{name:`signature`,type:`object`,raw:`{ kind: ClaimKind; namespace: string; name: string }`,signature:{properties:[{key:`kind`,value:{name:`ClaimKind`,required:!0}},{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]},required:!0}},{key:`kind`,value:{name:`union`,raw:`"CamundaRelease" | "CamundaPlatformConfig"`,elements:[{name:`literal`,value:`"CamundaRelease"`},{name:`literal`,value:`"CamundaPlatformConfig"`}],required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`followsDefault`,value:{name:`literal`,value:`true`,required:!0}},{key:`refused`,value:{name:`union`,raw:`"VersionDowngradeRefused" | "VersionSkipRefused"`,elements:[{name:`literal`,value:`"VersionDowngradeRefused"`},{name:`literal`,value:`"VersionSkipRefused"`}],required:!1}}]}},{name:`signature`,type:`object`,raw:`{
  claim: ClaimRef;
  kind: "CamundaRelease" | "CamundaPlatformConfig";
  cleared: true;
}`,signature:{properties:[{key:`claim`,value:{name:`signature`,type:`object`,raw:`{ kind: ClaimKind; namespace: string; name: string }`,signature:{properties:[{key:`kind`,value:{name:`ClaimKind`,required:!0}},{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]},required:!0}},{key:`kind`,value:{name:`union`,raw:`"CamundaRelease" | "CamundaPlatformConfig"`,elements:[{name:`literal`,value:`"CamundaRelease"`},{name:`literal`,value:`"CamundaPlatformConfig"`}],required:!0}},{key:`cleared`,value:{name:`literal`,value:`true`,required:!0}}]}}]}],raw:`ReferenceWarning[]`,required:!0}},{key:`unassigned`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!1},description:`unassigned are the namespaces that hold claims and no environment label
when the edit creates the first environment, sorted.`},{key:`unread`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!1},description:`unread are the kinds of the lists that could not be read or are stale.`}]}},description:``}}}})))()}export{u as i,s as n,m as r,c as t};