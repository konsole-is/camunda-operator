import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{c as n,x as r}from"./labels-Be1Q9KU6.js";import{n as i}from"./json-CPrrxgTM.js";import{o as a,t as o,u as s}from"./policy-C1QquhaH.js";import{a as c,i as l,r as u}from"./deletion-BnvIweCB.js";import{a as d,c as f,i as p,n as m,r as h,s as g}from"./delete-cluster-view-BV_ZGCeM.js";function _(e,t){return t===void 0?[]:e.filter(e=>e.spec?.managementAuthRef===t).sort((e,t)=>i(e.metadata?.namespace??``,t.metadata?.namespace??``)||i(e.metadata?.name??``,t.metadata?.name??``))}function v(){return(v=e((()=>{o()})))()}function ee(e){let{plane:t,namespaces:n,environments:r,environmentsUnread:i}=e,o=t?.status?.clusters;return Array.isArray(o)?o.flatMap(e=>{if(typeof e!=`object`||!e)return[];let t=e,o=y(t.namespace),s=y(t.name);if(o===void 0||s===void 0)return[];let c=y(t.reason),l=y(t.message),u=n.find(e=>e.metadata?.name===o),d=i||!u?void 0:a(u,r);return[{namespace:o,name:s,...d!==void 0&&{environment:d},...i&&{environmentUnknown:!0},attached:t.attached===!0,...c!==void 0&&{reason:c},...l!==void 0&&{message:l},href:`/fleet/clusters/${encodeURIComponent(o)}/${encodeURIComponent(s)}`}]}):[]}function y(e){return typeof e==`string`&&e!==``?e:void 0}function b(){return(b=e((()=>{o()})))()}function te(e){let{claim:t}=e,r=e.namespace?.metadata?.labels?.[n],i=r===void 0?void 0:e.environments.find(e=>e.metadata?.name===r);return s({claim:{...t,spec:{...t.spec,plugins:t.spec?.plugins}},environment:i,sections:e.sections,registrations:e.registrations,kind:`CamundaManagementClusterClaim`})}function x(e,t,n=[]){let{claim:r}=e,i=r.metadata?.name??``,a=r.metadata?.namespace??``,o=te(e),s=r.metadata?.uid,c=e=>s!==void 0&&(e.metadata?.ownerReferences??[]).some(e=>e.controller===!0&&e.uid===s),u=e.requests.filter(c),d=l({claim:r,sections:e.sections,registrations:e.registrations,requests:u,policy:t}),f=n.includes(`CamundaManagementCluster`),p=e.plane?c(e.plane):r.status?.managementCluster?.name===i,m=p?e.plane:void 0,h=f?void 0:m?.status?.managementAuthConfig||void 0,g=(e,t)=>({kind:e,name:t,label:`${e} ${t}`}),v=[...p?[g(`CamundaManagementCluster`,i)]:[],...u.map(e=>{let t=e.metadata?.name??``,n=r.status?.requests?.find(e=>e.name===t)?.section??e.spec?.section,i=d.find(e=>e.section===n)?.label;return{...g(`PluginRequest`,t),...i&&{label:`${i} request ${t}`}}}),g(`CamundaManagementClusterClaim`,i)],y=!n.includes(`CamundaOptimize`);return{name:i,namespace:a,choice:{...o,policy:t},keepRefused:o.policy===`Orphan`&&!o.keepAllowed,targets:v,kept:e.plane&&!p?[{...g(`CamundaManagementCluster`,i),note:C}]:[],groups:d,...!f&&{served:m?ee({plane:m,namespaces:[],environments:[]}).filter(e=>e.attached).map(({namespace:e,name:t})=>({namespace:e,name:t})):[]},...h&&{contract:h},...h&&y&&{readers:_(e.optimizes,h).map(e=>({namespace:e.metadata?.namespace??``,name:e.metadata?.name??``}))},ownKeycloak:m?.spec?.identityProvider?.keycloak!=null,unread:[...n],fatesKnown:!n.some(e=>S.includes(e))}}var S,C;function w(){return(w=e((()=>{c(),r(),o(),v(),b(),S=[`PluginRequest`,`PluginRegistration`,`PluginSection`],C=`Kept: the claim did not make it`})))()}function T({model:e,mode:t,policyStep:n,...r}){let{name:i,namespace:a}=e;return(0,O.jsx)(p,{subject:`management plane`,name:i,back:{href:`/fleet/planes/${encodeURIComponent(a)}/${encodeURIComponent(i)}`,label:`Back to the management plane`},done:{href:`/fleet/planes`,label:`All management planes`},deletedText:e.kept.length>0?`The claim ${i} is deleted. The management plane ${i} stays: the claim did not make it.`:`The management plane ${i} is deleted.`,choice:e.choice,keepRefusedOnDelete:e.keepRefused,unread:e.unread,effects:ne(e,t===`git`&&n),caution:E(e),mode:t,policyStep:n,...r})}function E(e){let t=e.served??[],n=[...t.length>0?[`This plane still serves ${t.length===1?`1 cluster`:`${t.length} clusters`}. Sign-in to Console, Web Modeler, and Optimize stops for them.`]:[],...e.ownKeycloak?[A,...e.choice.policy===`Delete`?[j]:[]]:[]];return n.length>0?n.join(` `):void 0}function D({namespace:e,name:t}){return`${e}/${t}`}function ne(e,t){let n=e=>e.map(e=>(0,O.jsxs)(`li`,{children:[e.label,e.note&&(0,O.jsxs)(`span`,{className:`text-muted`,children:[`: `,e.note]})]},`${e.kind}/${e.name}`)),r=e.unread.includes(`CamundaManagementCluster`),i=e=>!r||e.kind!==`CamundaManagementCluster`,a=e.targets.filter(e=>e.kind!==`PluginRequest`&&i(e)),o=e.kept.filter(i),s=e.served;return[(0,O.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,O.jsx)(`span`,{className:`font-medium text-text`,children:`Always deleted`}),(0,O.jsxs)(`ul`,{className:`flex flex-col gap-0.5`,children:[n(a),a.some(e=>e.kind===`CamundaManagementCluster`)&&(0,O.jsx)(`li`,{className:`text-muted`,children:`The plane takes its workloads, Services, and generated Secrets with it.`})]})]},`always`),...o.length>0?[(0,O.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,O.jsx)(`span`,{className:`font-medium text-text`,children:`Kept`}),(0,O.jsx)(`ul`,{className:`flex flex-col gap-0.5`,children:n(o)})]},`kept`)]:[],...r?[(0,O.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,O.jsx)(`span`,{className:`font-medium text-text`,children:`Not known`}),(0,O.jsx)(`ul`,{className:`flex flex-col gap-0.5`,children:(0,O.jsxs)(`li`,{children:[`CamundaManagementCluster `,e.name,(0,O.jsx)(`span`,{className:`text-muted`,children:`: not known`})]})})]},`unknown`)]:[],(0,O.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,O.jsx)(`span`,{className:`font-medium text-text`,children:`Clusters served`}),r||s===void 0?(0,O.jsx)(`span`,{children:`Not known: the page could not read CamundaManagementCluster.`}):e.kept.length>0?(0,O.jsx)(`span`,{children:`This delete does not touch the plane or the clusters that it serves.`}):s.length===0?(0,O.jsx)(`span`,{children:e.ownKeycloak?`No cluster is attached now. ${A}`:`The plane serves no cluster now.`}):(0,O.jsxs)(O.Fragment,{children:[(0,O.jsxs)(`span`,{className:`text-warn`,children:[`These clusters stay and keep running. The plane removes its Console settings and its claim from each, and each rolls its pods once. A cluster with basic authentication also loses its web-modeler user.`,e.ownKeycloak&&` ${A}`]}),(0,O.jsx)(`ul`,{className:`flex flex-col gap-0.5`,children:s.map(e=>(0,O.jsx)(`li`,{className:`break-all`,children:D(e)},D(e)))})]})]},`served`),...r||e.contract===void 0?[]:[(0,O.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,O.jsx)(`span`,{className:`font-medium text-text`,children:`Contract`}),(0,O.jsxs)(`span`,{children:[`The ManagementAuthConfig `,e.contract,` goes.`,` `,e.readers===void 0?`Which CamundaOptimize reads it is not known: the page could not read CamundaOptimize.`:e.readers.length===0?`No CamundaOptimize reads it.`:`Each CamundaOptimize that reads it then reports InvalidReference: ${e.readers.map(D).join(`, `)}.`]})]},`contract`)],...e.groups.map(t=>(0,O.jsx)(m,{group:t,known:e.fatesKnown},t.id)),...e.choice.policy===`Delete`?[k]:[],`The Secrets that you referenced stay.`,...t?[`Pushes two commits: the deletion policy first, then the delete once GitOps applied it.`]:[]]}var O,k,A,j;function M(){return(M=e((()=>{O=t(),h(),d(),k=`Deleting a Database keeps its data on its server. Deleting a DatabaseServer erases the server and its data.`,A=`Each cluster that uses the Keycloak of this plane as its OIDC issuer loses sign-in: its users and API clients can no longer sign in or get tokens.`,j=`When a DatabaseServer of an option holds the Keycloak database, the delete erases the users and clients of the realm.`,T.__docgenInfo={description:`DeletePlaneView is the delete page of a management plane:
ClaimDeleteView with what the delete of the plane takes. The other props
are those of ClaimDeleteView.`,methods:[],displayName:`DeletePlaneView`,props:{model:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`namespace`,value:{name:`string`,required:!0}},{key:`choice`,value:{name:`DeletionChoice`,required:!0}},{key:`keepRefused`,value:{name:`boolean`,required:!0}},{key:`targets`,value:{name:`Array`,elements:[{name:`DeleteTarget`}],raw:`DeleteTarget[]`,required:!0}},{key:`kept`,value:{name:`Array`,elements:[{name:`DeleteTarget`}],raw:`DeleteTarget[]`,required:!0}},{key:`groups`,value:{name:`Array`,elements:[{name:`DeletionGroup`}],raw:`DeletionGroup[]`,required:!0}},{key:`served`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ namespace: string; name: string }`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]}}],raw:`ObjectRef[]`,required:!1}},{key:`contract`,value:{name:`string`,required:!1}},{key:`readers`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ namespace: string; name: string }`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]}}],raw:`ObjectRef[]`,required:!1}},{key:`ownKeycloak`,value:{name:`boolean`,required:!0}},{key:`unread`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`fatesKnown`,value:{name:`boolean`,required:!0}}]}},description:``},mode:{required:!0,tsType:{name:`union`,raw:`WriteMode | undefined`,elements:[{name:`WriteMode`},{name:`undefined`}]},description:``},stage:{required:!0,tsType:{name:`DeleteStage`},description:``},errors:{required:!0,tsType:{name:`unknown`},description:``},remove:{required:!0,tsType:{name:`Change`},description:``},policyStep:{required:!0,tsType:{name:`boolean`},description:``},keepRefused:{required:!1,tsType:{name:`string`},description:``}}}})))()}var N,P,F,I,L,R,z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{f(),c(),w(),M(),{expect:N,fn:P,userEvent:F,within:I}=__STORYBOOK_MODULE_TEST__,L=e=>e,R=`3b2d7f0e-5c4a-4e0b-9f61-2a7d8c9e1f20`,z={ownerReferences:[{controller:!0,uid:R}]},B=[{section:`management-database`,registration:`cloudnative-pg`},{section:`access`,registration:`gateway`}],V=e=>L({apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaManagementClusterClaim`,metadata:{namespace:`platform`,name:`plane-a`,resourceVersion:`7`,uid:R},spec:{environments:[`platform`],plugins:B,deletionPolicy:e},status:{resolvedPolicy:{deletionPolicy:`Delete`},managementCluster:{name:`plane-a`},requests:B.map(e=>({...e,name:`plane-a-${e.section}`}))}}),H=(e,t,n,r)=>L({metadata:{name:e},spec:{label:t,sections:n,deletionPolicies:r}}),U=(e={})=>({claim:V(),plane:L({metadata:{namespace:`platform`,name:`plane-a`,...z},spec:{},status:{managementAuthConfig:`plane-a`,clusters:[{namespace:`team-orders`,name:`orders`,attached:!0},{namespace:`team-payments`,name:`payments`,attached:!0}]}}),environments:[],sections:[L({metadata:{name:`management-database`},spec:{label:`Management database`,order:10,selection:`One`,appliesTo:[`CamundaManagementClusterClaim`]}}),L({metadata:{name:`access`},spec:{label:`Access`,order:20,selection:`One`,appliesTo:[`CamundaManagementClusterClaim`]}})],registrations:[H(`cloudnative-pg`,`CloudNativePG`,[`management-database`],[`Delete`,`Orphan`]),H(`gateway`,`Gateway`,[`access`],[`Delete`,`Orphan`])],requests:[L({metadata:{namespace:`platform`,name:`plane-a-management-database`,...z},status:{resources:[{kind:`DatabaseServer`,name:`plane-a-pg`,onOrphan:`Kept`},{kind:`Database`,name:`plane-a-identity`,onOrphan:`Kept`}]}}),L({metadata:{namespace:`platform`,name:`plane-a-access`,...z},status:{resources:[{kind:`HTTPRoute`,name:`plane-a`,onOrphan:`Deleted`}]}})],optimizes:[L({metadata:{namespace:`team-orders`,name:`orders`},spec:{managementAuthRef:`plane-a`}})],errors:[],...e}),W=u(U().claim,`Delete`,`Delete the management plane plane-a`).remove,G={title:`planes/DeletePlaneView`,component:T,args:{model:x(U(),`Delete`),mode:`direct`,stage:{kind:`choose`},errors:[],remove:W,policyStep:!1,onPick:P(),preview:P(async()=>({objects:[],errors:[]})),apply:P(async()=>({ok:!0,mode:`direct`})),show:P(),onWritten:P(),onStopWaiting:P()}},K={name:`Delete preselected, name typed`,play:async({canvasElement:e})=>{let t=I(e);await F.type(await t.findByLabelText(`Type plane-a to confirm`),`plane-a`),await N(t.getByLabelText(`Type plane-a to confirm`)).toHaveValue(`plane-a`)}},q={name:`Keep off for an option that always deletes`,args:{model:x(U({registrations:[H(`cloudnative-pg`,`CloudNativePG`,[`management-database`],[`Delete`]),H(`gateway`,`Gateway`,[`access`],[`Delete`,`Orphan`])]}),`Delete`)}},J={name:`Keep chosen, two commits in Git mode`,args:{model:x(U({claim:V(`Orphan`)}),`Orphan`),mode:`git`,policyStep:!0}},Y={name:`Plane not read`,args:{model:x(U(),`Delete`,[`CamundaManagementCluster`]),errors:[{kind:`CamundaManagementCluster`,message:`connection lost`}]}},X={name:`Deleting`,args:{stage:{kind:`deleting`,gitWaiting:!1,progress:g(x(U(),`Delete`).targets,{CamundaManagementCluster:[{metadata:{name:`plane-a`,deletionTimestamp:`2026-11-16T10:00:00Z`}}],PluginRequest:[],CamundaManagementClusterClaim:[{metadata:{name:`plane-a`}}]})}}},Z={name:`Plane with its own Keycloak`,args:{model:x(U({plane:L({metadata:{namespace:`platform`,name:`plane-a`,...z},spec:{identityProvider:{keycloak:{}}},status:{managementAuthConfig:`plane-a`,clusters:[{namespace:`team-orders`,name:`orders`,attached:!0}]}})}),`Delete`)},play:async({canvasElement:e})=>{let t=I(e);await N(await t.findByText(/This plane still serves 1 cluster\./)).toBeVisible()}},Q=[`DeletePreselected`,`KeepOff`,`OrphanChosen`,`PlaneNotRead`,`Deleting`,`OwnKeycloak`],K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "Delete preselected, name typed",
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await userEvent.type(await canvas.findByLabelText("Type plane-a to confirm"), "plane-a");
    await expect(canvas.getByLabelText("Type plane-a to confirm")).toHaveValue("plane-a");
  }
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "Keep off for an option that always deletes",
  args: {
    model: deletePlaneModel(data({
      registrations: [registration("cloudnative-pg", "CloudNativePG", ["management-database"], ["Delete"]), registration("gateway", "Gateway", ["access"], ["Delete", "Orphan"])]
    }), "Delete")
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Keep chosen, two commits in Git mode",
  args: {
    model: deletePlaneModel(data({
      claim: claim("Orphan")
    }), "Orphan"),
    mode: "git",
    policyStep: true
  }
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Plane not read",
  args: {
    model: deletePlaneModel(data(), "Delete", ["CamundaManagementCluster"]),
    errors: [{
      kind: "CamundaManagementCluster",
      message: "connection lost"
    }]
  }
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
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
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
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
}`,...Z.parameters?.docs?.source}}}})))()}$();export{K as DeletePreselected,X as Deleting,q as KeepOff,J as OrphanChosen,Z as OwnKeycloak,Y as PlaneNotRead,Q as __namedExportsOrder,G as default};