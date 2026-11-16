import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{c as r,x as i}from"./labels-Be1Q9KU6.js";import{S as a}from"./release-C_p_MFbM.js";import{o,t as s,u as c}from"./policy-DwLw1C5g.js";import{n as l,t as u}from"./link-DOLMGIR2.js";import{a as d,n as f}from"./deletion-choice-CghhV1MF.js";import{d as p,f as m,i as h,l as g,o as _,p as v,r as ee,s as y,t as te,u as b}from"./delete-cluster-view-BZdosXcO.js";import{n as x,t as ne}from"./change-review-BAs1VE6i.js";import{a as re,r as S}from"./rows-CNIFHT37.js";import{i as ie,r as ae}from"./waiting-panel-BJU9AoNQ.js";import{c as oe,n as se}from"./cluster-edit-B2VxOfcf.js";function ce(e,t){return t===void 0?[]:e.filter(e=>e.spec?.managementAuthRef===t).sort((e,t)=>a(e.metadata?.namespace??``,t.metadata?.namespace??``)||a(e.metadata?.name??``,t.metadata?.name??``))}function C(){return(C=e((()=>{s()})))()}function le(e){let{plane:t,namespaces:n,environments:r,environmentsUnread:i}=e,a=t?.status?.clusters;return Array.isArray(a)?a.flatMap(e=>{if(typeof e!=`object`||!e)return[];let t=e,a=w(t.namespace),s=w(t.name);if(a===void 0||s===void 0)return[];let c=w(t.reason),l=w(t.message),u=n.find(e=>e.metadata?.name===a),d=i||!u?void 0:o(u,r);return[{namespace:a,name:s,...d!==void 0&&{environment:d},...i&&{environmentUnknown:!0},attached:t.attached===!0,...c!==void 0&&{reason:c},...l!==void 0&&{message:l},href:`/fleet/clusters/${encodeURIComponent(a)}/${encodeURIComponent(s)}`}]}):[]}function w(e){return typeof e==`string`&&e!==``?e:void 0}function T(){return(T=e((()=>{s()})))()}function ue(e){let{claim:t}=e,n=e.namespace?.metadata?.labels?.[r],i=n===void 0?void 0:e.environments.find(e=>e.metadata?.name===n);return c({claim:{...t,spec:{...t.spec,plugins:t.spec?.plugins}},environment:i,sections:e.sections,registrations:e.registrations,kind:`CamundaManagementClusterClaim`})}function E(e,t,n=[]){let{claim:r}=e,i=r.metadata?.name??``,a=r.metadata?.namespace??``,o=ue(e),s=r.metadata?.uid,c=e=>s!==void 0&&(e.metadata?.ownerReferences??[]).some(e=>e.controller===!0&&e.uid===s),l=e.requests.filter(c),u=m({claim:r,sections:e.sections,registrations:e.registrations,requests:l,policy:t}),d=n.includes(`CamundaManagementCluster`),f=e.plane?c(e.plane):r.status?.managementCluster?.name===i,p=f?e.plane:void 0,h=d?void 0:p?.status?.managementAuthConfig||void 0,g=(e,t)=>({kind:e,name:t,label:`${e} ${t}`}),_=[...f?[g(`CamundaManagementCluster`,i)]:[],...l.map(e=>{let t=e.metadata?.name??``,n=r.status?.requests?.find(e=>e.name===t)?.section??e.spec?.section,i=u.find(e=>e.section===n)?.label;return{...g(`PluginRequest`,t),...i&&{label:`${i} request ${t}`}}}),g(`CamundaManagementClusterClaim`,i)],v=!n.includes(`CamundaOptimize`);return{name:i,namespace:a,choice:{...o,policy:t},keepRefused:o.policy===`Orphan`&&!o.keepAllowed,targets:_,kept:e.plane&&!f?[{...g(`CamundaManagementCluster`,i),note:de}]:[],groups:u,...!d&&{served:p?le({plane:p,namespaces:[],environments:[]}).filter(e=>e.attached).map(({namespace:e,name:t})=>({namespace:e,name:t})):[]},...h&&{contract:h},...h&&v&&{readers:ce(e.optimizes,h).map(e=>({namespace:e.metadata?.namespace??``,name:e.metadata?.name??``}))},ownKeycloak:p?.spec?.identityProvider?.keycloak!=null,unread:[...n],fatesKnown:!n.some(e=>D.includes(e))}}var D,de;function fe(){return(fe=e((()=>{v(),i(),s(),C(),T(),D=[`PluginRequest`,`PluginRegistration`,`PluginSection`],de=`Kept: the claim did not make it`})))()}function pe({model:e,mode:t,stage:n,errors:r,remove:i,policyStep:a,keepRefused:o,onPick:s,preview:c,apply:u,show:d,onWritten:p,onStopWaiting:m}){let{name:h,namespace:g}=e,v=`/fleet/planes/${encodeURIComponent(g)}/${encodeURIComponent(h)}`,y=he(e,t===`git`&&a),[b,x]=(0,A.useState)(!1),S=(0,k.jsx)(`section`,{"aria-label":`What the delete takes`,children:(0,k.jsx)(`ul`,{className:`flex list-disc flex-col gap-1 pl-5 text-sm text-text-2`,children:y.map((e,t)=>(0,k.jsx)(`li`,{children:e},t))})});return(0,k.jsxs)(`div`,{className:`flex max-w-3xl min-w-0 flex-col gap-6`,children:[(0,k.jsxs)(`div`,{className:`flex flex-col gap-2`,children:[(0,k.jsx)(l,{href:v,className:`relative w-fit text-sm text-muted hit-area hover:text-text`,children:`Back to the management plane`}),(0,k.jsxs)(`h1`,{className:`font-heading text-2xl font-bold tracking-[-0.02em] break-all`,children:[`Delete the management plane `,h]})]}),n.kind===`deleted`?(0,k.jsxs)(`div`,{className:`flex flex-col gap-2 text-sm`,children:[(0,k.jsx)(`p`,{role:`status`,className:`text-text`,children:e.kept.length>0?`The claim ${h} is deleted. The management plane ${h} stays: the claim did not make it.`:`The management plane ${h} is deleted.`}),(0,k.jsx)(l,{href:`/fleet/planes`,className:`relative w-fit text-accent hit-area hover:underline`,children:`All management planes`})]}):n.kind===`replaced`?(0,k.jsxs)(`div`,{className:`flex flex-col gap-2 text-sm`,children:[(0,k.jsxs)(`p`,{role:`status`,className:`text-text`,children:[`The claim `,h,` was made again after this page opened. Open the delete page again to delete the new claim.`]}),(0,k.jsx)(l,{href:v,className:`relative w-fit text-accent hit-area hover:underline`,children:`Back to the management plane`})]}):n.kind===`waiting`?(0,k.jsxs)(k.Fragment,{children:[(0,k.jsx)(_,{errors:r}),(0,k.jsx)(ae,{text:t===`git`?`Waiting for GitOps to apply the deletion policy. The fleet manager then removes the claim file.`:`Waiting for the operator to read the deletion policy. The fleet manager then deletes the claim.`,onStopWaiting:m})]}):n.kind===`deleting`?(0,k.jsxs)(k.Fragment,{children:[(0,k.jsx)(_,{errors:r}),(0,k.jsx)(ee,{name:h,stage:n})]}):(0,k.jsxs)(k.Fragment,{children:[(0,k.jsx)(f,{subject:`management plane`,choice:e.choice,onChange:s,disabled:b}),e.keepRefused&&e.choice.policy!==`Orphan`&&(0,k.jsx)(`p`,{className:`-mt-3 text-sm text-warn`,children:`The claim keeps its resources, but an option below cannot keep them. The delete removes them.`}),t===`git`&&(0,k.jsx)(`p`,{className:`text-sm text-text-2`,children:M}),(0,k.jsx)(_,{errors:r}),n.errors&&n.errors.length>0&&(0,k.jsx)(te,{texts:n.errors.map(e=>e.message)}),n.afterPolicyWrite&&(0,k.jsx)(`p`,{role:`status`,className:`text-sm text-warn`,children:ge(h,n.afterPolicyWrite)}),t===void 0?(0,k.jsxs)(k.Fragment,{children:[(0,k.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:oe}),S]}):e.unread.length>0&&!b?(0,k.jsxs)(k.Fragment,{children:[(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:re(e.unread)}),S]}):o&&!b?(0,k.jsxs)(k.Fragment,{children:[(0,k.jsx)(`p`,{className:`text-sm text-warn`,children:o}),S]}):(0,k.jsx)(ne,{change:i,mode:t,diffLabel:`The claim that the delete removes`,effects:y,caution:me(e),confirm:{label:`Delete the management plane`,gitLabel:`Push the delete to Git`,danger:!0,typed:h},savedMessage:t===`direct`?a?`Set the deletion policy of ${h}. The delete follows when the operator reads it.`:`The claim ${h} is being deleted.`:a?`Pushed the deletion policy of ${h}. The delete follows when GitOps applies it.`:`Pushed the delete of ${h}.`,preview:c,apply:u,show:d,onWritten:p,onWriting:x})]})]})}function me(e){let{served:t}=e;if(t&&t.length!==0)return[`This plane still serves ${t.length===1?`1 cluster`:`${t.length} clusters`}. Sign-in to Console, Web Modeler, and Optimize stops for them.`,...e.ownKeycloak?[`Those that use OIDC sign in at the Keycloak of this plane, so their users and API clients can no longer sign in or get tokens.`,...e.choice.policy===`Delete`?[N]:[]]:[]].join(` `)}function O({namespace:e,name:t}){return`${e}/${t}`}function he(e,t){let n=e=>e.map(e=>(0,k.jsxs)(`li`,{children:[e.label,e.note&&(0,k.jsxs)(`span`,{className:`text-muted`,children:[`: `,e.note]})]},`${e.kind}/${e.name}`)),r=e.targets.filter(e=>e.kind!==`PluginRequest`),i=e.unread.includes(`CamundaManagementCluster`),a=e.served;return[(0,k.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,k.jsx)(`span`,{className:`font-medium text-text`,children:`Always deleted`}),(0,k.jsxs)(`ul`,{className:`flex flex-col gap-0.5`,children:[n(r),e.targets.some(e=>e.kind===`CamundaManagementCluster`)&&(0,k.jsx)(`li`,{className:`text-muted`,children:`The plane takes its workloads, Services, and generated Secrets with it.`})]})]},`always`),...e.kept.length>0?[(0,k.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,k.jsx)(`span`,{className:`font-medium text-text`,children:`Kept`}),(0,k.jsx)(`ul`,{className:`flex flex-col gap-0.5`,children:n(e.kept)})]},`kept`)]:[],(0,k.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,k.jsx)(`span`,{className:`font-medium text-text`,children:`Clusters served`}),i||a===void 0?(0,k.jsx)(`span`,{children:`Not known: the page could not read CamundaManagementCluster.`}):e.kept.length>0?(0,k.jsx)(`span`,{children:`This delete does not touch the plane or the clusters that it serves.`}):a.length===0?(0,k.jsx)(`span`,{children:`The plane serves no cluster now.`}):(0,k.jsxs)(k.Fragment,{children:[(0,k.jsxs)(`span`,{className:`text-warn`,children:[`These clusters stay and keep running. The plane removes its Console settings and its claim from each, and each rolls its pods once. A cluster with basic authentication also loses its web-modeler user.`,e.ownKeycloak&&` A cluster that uses OIDC signs in at the Keycloak of this plane, so its users and API clients can no longer sign in or get tokens.`]}),(0,k.jsx)(`ul`,{className:`flex flex-col gap-0.5`,children:a.map(e=>(0,k.jsx)(`li`,{className:`break-all`,children:O(e)},O(e)))})]})]},`served`),...i||e.contract===void 0?[]:[(0,k.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,k.jsx)(`span`,{className:`font-medium text-text`,children:`Contract`}),(0,k.jsxs)(`span`,{children:[`The ManagementAuthConfig `,e.contract,` goes.`,` `,e.readers===void 0?`Which CamundaOptimize reads it is not known: the page could not read CamundaOptimize.`:e.readers.length===0?`No CamundaOptimize reads it.`:`Each CamundaOptimize that reads it then reports InvalidReference: ${e.readers.map(O).join(`, `)}.`]})]},`contract`)],...e.groups.map(t=>(0,k.jsx)(h,{group:t,known:e.fatesKnown},t.id)),...e.choice.policy===`Delete`?[j]:[],`The Secrets that you referenced stay.`,...t?[`Pushes two commits: the deletion policy first, then the delete once GitOps applied it.`]:[]]}function ge(e,t){let n=e=>e===`Orphan`?`Keep`:`Delete`,r=`The claim ${e} has the deletion policy ${n(t.held)}.`;return t.unapplied?`The management plane is not deleted. The repository holds the commit that sets the deletion policy of ${e} to ${n(t.unapplied)}, and GitOps has not applied it yet. ${r}`:`The management plane is not deleted. ${r}`}var k,A,j,M,N;function P(){return(P=e((()=>{k=n(),u(),A=t(),y(),se(),ie(),x(),d(),S(),j=`Deleting a Database keeps its data on its server. Deleting a DatabaseServer erases the server and its data.`,M=`Your GitOps tool must prune: the fleet manager removes the claim file, and GitOps deletes the claim.`,N=`When a DatabaseServer of an option holds the Keycloak database, the delete erases the users and clients of the realm.`,pe.__docgenInfo={description:`DeletePlaneView is the delete page of a management plane. It never
writes: the confirm of its review calls apply with remove, and the
review previews remove once for each remove object. Without a mode it
shows CHANGES_OFF and what the delete takes, and no review. While
model.unread has a kind it shows what the delete takes, with each fact
that depends on that kind as not known, and no review, unless a write
runs. While keepRefused is set it shows keepRefused in place of the
review, unless a write runs. policyStep says that the delete writes the
deletion policy first.`,methods:[],displayName:`DeletePlaneView`,props:{model:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  name: string;
  namespace: string;
  choice: DeletionChoice;
  keepRefused: boolean;
  targets: DeleteTarget[];
  kept: DeleteTarget[];
  groups: DeletionGroup[];
  served?: ObjectRef[];
  contract?: string;
  readers?: ObjectRef[];
  ownKeycloak: boolean;
  unread: string[];
  fatesKnown: boolean;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`namespace`,value:{name:`string`,required:!0}},{key:`choice`,value:{name:`DeletionChoice`,required:!0}},{key:`keepRefused`,value:{name:`boolean`,required:!0}},{key:`targets`,value:{name:`Array`,elements:[{name:`DeleteTarget`}],raw:`DeleteTarget[]`,required:!0}},{key:`kept`,value:{name:`Array`,elements:[{name:`DeleteTarget`}],raw:`DeleteTarget[]`,required:!0}},{key:`groups`,value:{name:`Array`,elements:[{name:`DeletionGroup`}],raw:`DeletionGroup[]`,required:!0}},{key:`served`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ namespace: string; name: string }`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]}}],raw:`ObjectRef[]`,required:!1}},{key:`contract`,value:{name:`string`,required:!1}},{key:`readers`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ namespace: string; name: string }`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]}}],raw:`ObjectRef[]`,required:!1}},{key:`ownKeycloak`,value:{name:`boolean`,required:!0}},{key:`unread`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`fatesKnown`,value:{name:`boolean`,required:!0}}]}},description:``},mode:{required:!0,tsType:{name:`union`,raw:`WriteMode | undefined`,elements:[{name:`WriteMode`},{name:`undefined`}]},description:``},stage:{required:!0,tsType:{name:`union`,raw:`| {
    kind: "choose";
    errors?: ApiError[];
    afterPolicyWrite?: PolicyAfterWrite;
  }
| { kind: "waiting" }
| {
    kind: "deleting";
    progress: { target: DeleteTarget; state: DeleteState }[];
    gitWaiting: boolean;
    claim?: Resource<"CamundaManagementClusterClaim">;
    keepRefused?: string;
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
  claim?: Resource<"CamundaManagementClusterClaim">;
  keepRefused?: string;
}`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"deleting"`,required:!0}},{key:`progress`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ target: DeleteTarget; state: DeleteState }`,signature:{properties:[{key:`target`,value:{name:`DeleteTarget`,required:!0}},{key:`state`,value:{name:`DeleteState`,required:!0}}]}}],raw:`{ target: DeleteTarget; state: DeleteState }[]`,required:!0}},{key:`gitWaiting`,value:{name:`boolean`,required:!0}},{key:`claim`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaManagementClusterClaim"`}],raw:`Resource<"CamundaManagementClusterClaim">`,required:!1}},{key:`keepRefused`,value:{name:`string`,required:!1}}]}},{name:`signature`,type:`object`,raw:`{ kind: "deleted" }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"deleted"`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{ kind: "replaced" }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"replaced"`,required:!0}}]}}]},description:``},errors:{required:!0,tsType:{name:`unknown`},description:``},remove:{required:!0,tsType:{name:`Change`},description:``},policyStep:{required:!0,tsType:{name:`boolean`},description:``},keepRefused:{required:!1,tsType:{name:`string`},description:``}}}})))()}var F,I,L,R,z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q,$,_e;function ve(){return(ve=e((()=>{b(),v(),fe(),P(),{expect:F,fn:I,userEvent:L,within:R}=__STORYBOOK_MODULE_TEST__,z=e=>e,B=`3b2d7f0e-5c4a-4e0b-9f61-2a7d8c9e1f20`,V={ownerReferences:[{controller:!0,uid:B}]},H=[{section:`management-database`,registration:`cloudnative-pg`},{section:`access`,registration:`gateway`}],U=e=>z({apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaManagementClusterClaim`,metadata:{namespace:`platform`,name:`plane-a`,resourceVersion:`7`,uid:B},spec:{environments:[`platform`],plugins:H,deletionPolicy:e},status:{resolvedPolicy:{deletionPolicy:`Delete`},managementCluster:{name:`plane-a`},requests:H.map(e=>({...e,name:`plane-a-${e.section}`}))}}),W=(e,t,n,r)=>z({metadata:{name:e},spec:{label:t,sections:n,deletionPolicies:r}}),G=(e={})=>({claim:U(),plane:z({metadata:{namespace:`platform`,name:`plane-a`,...V},spec:{},status:{managementAuthConfig:`plane-a`,clusters:[{namespace:`team-orders`,name:`orders`,attached:!0},{namespace:`team-payments`,name:`payments`,attached:!0}]}}),environments:[],sections:[z({metadata:{name:`management-database`},spec:{label:`Management database`,order:10,selection:`One`,appliesTo:[`CamundaManagementClusterClaim`]}}),z({metadata:{name:`access`},spec:{label:`Access`,order:20,selection:`One`,appliesTo:[`CamundaManagementClusterClaim`]}})],registrations:[W(`cloudnative-pg`,`CloudNativePG`,[`management-database`],[`Delete`,`Orphan`]),W(`gateway`,`Gateway`,[`access`],[`Delete`,`Orphan`])],requests:[z({metadata:{namespace:`platform`,name:`plane-a-management-database`,...V},status:{resources:[{kind:`DatabaseServer`,name:`plane-a-pg`,onOrphan:`Kept`},{kind:`Database`,name:`plane-a-identity`,onOrphan:`Kept`}]}}),z({metadata:{namespace:`platform`,name:`plane-a-access`,...V},status:{resources:[{kind:`HTTPRoute`,name:`plane-a`,onOrphan:`Deleted`}]}})],optimizes:[z({metadata:{namespace:`team-orders`,name:`orders`},spec:{managementAuthRef:`plane-a`}})],errors:[],...e}),K=p(G().claim,`Delete`,`Delete the management plane plane-a`).remove,q={title:`planes/DeletePlaneView`,component:pe,args:{model:E(G(),`Delete`),mode:`direct`,stage:{kind:`choose`},errors:[],remove:K,policyStep:!1,onPick:I(),preview:I(async()=>({objects:[],errors:[]})),apply:I(async()=>({ok:!0,mode:`direct`})),show:I(),onWritten:I(),onStopWaiting:I()}},J={name:`Delete preselected, name typed`,play:async({canvasElement:e})=>{let t=R(e);await L.type(await t.findByLabelText(`Type plane-a to confirm`),`plane-a`),await F(t.getByLabelText(`Type plane-a to confirm`)).toHaveValue(`plane-a`)}},Y={name:`Keep off for an option that always deletes`,args:{model:E(G({registrations:[W(`cloudnative-pg`,`CloudNativePG`,[`management-database`],[`Delete`]),W(`gateway`,`Gateway`,[`access`],[`Delete`,`Orphan`])]}),`Delete`)}},X={name:`Keep chosen, two commits in Git mode`,args:{model:E(G({claim:U(`Orphan`)}),`Orphan`),mode:`git`,policyStep:!0}},Z={name:`Plane not read`,args:{model:E(G(),`Delete`,[`CamundaManagementCluster`]),errors:[{kind:`CamundaManagementCluster`,message:`connection lost`}]}},Q={name:`Deleting`,args:{stage:{kind:`deleting`,gitWaiting:!1,progress:g(E(G(),`Delete`).targets,{CamundaManagementCluster:[{metadata:{name:`plane-a`,deletionTimestamp:`2026-11-16T10:00:00Z`}}],PluginRequest:[],CamundaManagementClusterClaim:[{metadata:{name:`plane-a`}}]})}}},$={name:`Plane with its own Keycloak`,args:{model:E(G({plane:z({metadata:{namespace:`platform`,name:`plane-a`,...V},spec:{identityProvider:{keycloak:{}}},status:{managementAuthConfig:`plane-a`,clusters:[{namespace:`team-orders`,name:`orders`,attached:!0}]}})}),`Delete`)},play:async({canvasElement:e})=>{let t=R(e);await F(await t.findByText(/This plane still serves 1 cluster\./)).toBeVisible()}},_e=[`DeletePreselected`,`KeepOff`,`OrphanChosen`,`PlaneNotRead`,`Deleting`,`OwnKeycloak`],J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Delete preselected, name typed",
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await userEvent.type(await canvas.findByLabelText("Type plane-a to confirm"), "plane-a");
    await expect(canvas.getByLabelText("Type plane-a to confirm")).toHaveValue("plane-a");
  }
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Keep off for an option that always deletes",
  args: {
    model: deletePlaneModel(data({
      registrations: [registration("cloudnative-pg", "CloudNativePG", ["management-database"], ["Delete"]), registration("gateway", "Gateway", ["access"], ["Delete", "Orphan"])]
    }), "Delete")
  }
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Keep chosen, two commits in Git mode",
  args: {
    model: deletePlaneModel(data({
      claim: claim("Orphan")
    }), "Orphan"),
    mode: "git",
    policyStep: true
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Plane not read",
  args: {
    model: deletePlaneModel(data(), "Delete", ["CamundaManagementCluster"]),
    errors: [{
      kind: "CamundaManagementCluster",
      message: "connection lost"
    }]
  }
}`,...Z.parameters?.docs?.source}}},Q.parameters={...Q.parameters,docs:{...Q.parameters?.docs,source:{originalSource:`{
  name: "Deleting",
  args: {
    stage: {
      kind: "deleting",
      gitWaiting: false,
      progress: deleteProgress(deletePlaneModel(data(), "Delete").targets, {
        CamundaManagementCluster: [{
          metadata: {
            name: "plane-a",
            deletionTimestamp: "2026-11-16T10:00:00Z"
          }
        }],
        PluginRequest: [],
        CamundaManagementClusterClaim: [{
          metadata: {
            name: "plane-a"
          }
        }]
      })
    }
  }
}`,...Q.parameters?.docs?.source}}},$.parameters={...$.parameters,docs:{...$.parameters?.docs,source:{originalSource:`{
  name: "Plane with its own Keycloak",
  args: {
    model: deletePlaneModel(data({
      plane: object({
        metadata: {
          namespace: "platform",
          name: "plane-a",
          ...controlled
        },
        spec: {
          identityProvider: {
            keycloak: {}
          }
        },
        status: {
          managementAuthConfig: "plane-a",
          clusters: [{
            namespace: "team-orders",
            name: "orders",
            attached: true
          }]
        }
      })
    }), "Delete")
  },
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText(/This plane still serves 1 cluster\\./)).toBeVisible();
  }
}`,...$.parameters?.docs?.source}}}})))()}ve();export{J as DeletePreselected,Q as Deleting,Y as KeepOff,X as OrphanChosen,$ as OwnKeycloak,Z as PlaneNotRead,_e as __namedExportsOrder,q as default};