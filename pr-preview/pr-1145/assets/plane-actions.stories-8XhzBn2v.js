import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{x as r,y as i}from"./labels-Be1Q9KU6.js";import{c as a,s as o}from"./iframe-B5mWhD3A.js";import{n as s}from"./refs-CNug01Rr.js";import{G as c,Q as l,t as u}from"./session-BGnoDMJM.js";import{n as d,t as f}from"./link-DOLMGIR2.js";import{n as p,r as m,t as h}from"./button-B_BCHFEs.js";import{n as g,t as _}from"./inline-confirm-BjcqzZsT.js";import{n as v,r as y,t as b}from"./toast-BDs8vH4K.js";import{r as x}from"./claim-edit-D6OFTx2q.js";import{n as S,t as C}from"./git-waiting-line-z5pGZ9JE.js";function w(e,t){let{session:n}=l({claim:e,pending:new Map,locks:()=>void 0},[{field:`spec`},{field:`suspend`}],t),r=e.metadata.name??``;return c(n,`${t?`Suspend`:`Resume`} the management plane ${r}`)}function T(){return(T=e((()=>{u()})))()}function E({claim:e,served:t,ownKeycloak:n=!1,apply:r,deleteHref:o,writeMode:c,unread:l=[],now:u}){let{toast:f,show:m,dismiss:g}=y(),{pushes:v}=a(),{namespace:x,name:S=``,resourceVersion:T}=e.metadata??{},E=s({kind:`CamundaManagementClusterClaim`,namespace:x,name:S}),A=v.find(e=>e.key===E),j=e.metadata?.annotations?.[i],M=A!==void 0&&(`deleted`in A||A.revision!==j),[N,P]=(0,O.useState)(!1),[F,I]=(0,O.useState)(),L=l.length>0||N||M||F!==void 0&&F===T,R=e.metadata?.deletionTimestamp!=null,z=L||R||c===void 0,B=async t=>{if(z)return;P(!0);let n=await r(w(e,t)).catch(e=>({ok:!1,errors:[{code:`Unavailable`,message:String(e)}]}));P(!1),n.ok&&I(T),m(n,`${t?`Suspended`:`Resumed`} the management plane ${S}.`)},V=e.spec.suspend===!0;return(0,D.jsxs)(`div`,{className:`flex max-w-xl flex-col items-end gap-2`,children:[(0,D.jsxs)(`div`,{className:`flex flex-wrap items-start justify-end gap-2`,children:[V||z?(0,D.jsx)(h,{variant:`outline`,disabled:z,onClick:()=>void B(!1),children:V?`Resume`:`Suspend`}):(0,D.jsx)(_,{triggerLabel:`Suspend`,confirmLabel:`Suspend the management plane`,goes:[`Every workload of the plane stops. Sign-in to Console, Web Modeler, and Optimize stops for every cluster that this plane serves (${t}).`,...n&&t>0?[k]:[]],stays:[`The served clusters keep running, and the databases keep the data of the plane.`],onConfirm:()=>void B(!0)}),L?(0,D.jsx)(h,{variant:`outline`,disabled:!0,children:`Delete`}):(0,D.jsx)(d,{href:o,className:p({variant:`outline`}),children:`Delete`})]}),c===void 0&&(0,D.jsx)(`p`,{className:`text-right text-sm text-muted`,children:`Changes are off: the fleet manager could not read its settings.`}),R&&(0,D.jsx)(`p`,{className:`text-right text-sm text-muted`,children:`Suspend and Resume are off while the claim is being deleted.`}),l.length>0&&(0,D.jsxs)(`p`,{className:`text-right text-sm text-muted`,children:[`Suspend and Delete are possible when the fleet manager can read`,` `,l.join(`, `),`.`]}),(0,D.jsx)(C,{pushed:A,liveRevision:j,now:u}),f&&(0,D.jsx)(b,{result:f,onDismiss:g})]})}var D,O,k;function A(){return(A=e((()=>{D=n(),f(),O=t(),x(),o(),S(),g(),v(),m(),r(),T(),k=`The served clusters that use OIDC sign in at the Keycloak of this plane. Their users and API clients cannot sign in or get tokens until the plane resumes.`,E.__docgenInfo={description:`PlaneActions holds Suspend or Resume, and Delete, of a plane that a
claim made. Suspend asks in place and says that sign-in stops for every
cluster that the plane serves; served is that number. ownKeycloak says
that the plane runs the Keycloak that its OIDC clusters sign in at.
Resume writes at once. Each write sets spec.suspend through apply, with
the resourceVersion of claim as its precondition. Delete links to
deleteHref. Both stay off while unread names a kind that the page could
not read, while a write runs, until the claim shows a newer version after
a write succeeds, and while a pushed change or delete of the claim waits
for GitOps; an open confirm then closes. Suspend and Resume are off while
the claim is being deleted, and while writeMode is undefined. It needs a
GitPushesProvider and a QueryClientProvider above it.`,methods:[],displayName:`PlaneActions`,props:{claim:{required:!0,tsType:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaManagementClusterClaim"`}],raw:`Resource<"CamundaManagementClusterClaim">`},description:``},served:{required:!0,tsType:{name:`number`},description:``},ownKeycloak:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`false`,computed:!1}},apply:{required:!0,tsType:{name:`ClaimActions["apply"]`,raw:`ClaimActions["apply"]`},description:``},deleteHref:{required:!0,tsType:{name:`string`},description:``},writeMode:{required:!0,tsType:{name:`union`,raw:`WriteMode | undefined`,elements:[{name:`WriteMode`},{name:`undefined`}]},description:``},unread:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}var j,M,N,P,F,I,L,R,z,B,V,H,U,W;function G(){return(G=e((()=>{A(),{expect:j,fn:M,userEvent:N,within:P}=__STORYBOOK_MODULE_TEST__,F=e=>({apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaManagementClusterClaim`,metadata:{namespace:`platform`,name:`plane-a`,resourceVersion:`7`},spec:{environments:[`platform`],...e&&{suspend:e}}}),I={title:`planes/PlaneActions`,component:E,args:{claim:F(),served:3,apply:M(async()=>({ok:!0,mode:`direct`})),deleteHref:`/fleet/planes/platform/plane-a/delete`,writeMode:`direct`,now:`2026-11-15T12:00:00Z`}},L={name:`Running plane`},R={name:`Suspend open`,play:async({canvasElement:e})=>{let t=P(e);await N.click(t.getByRole(`button`,{name:`Suspend`})),await j(t.getByRole(`button`,{name:`Suspend the management plane`})).toBeVisible()}},z={name:`Suspended, with Resume`,args:{claim:F(!0)}},B={name:`Plane not read`,args:{unread:[`CamundaManagementCluster`]}},V={name:`Suspend open, own Keycloak`,args:{ownKeycloak:!0},play:async({canvasElement:e})=>{let t=P(e);await N.click(t.getByRole(`button`,{name:`Suspend`})),await j(t.getByText(/sign in at the Keycloak of this plane/)).toBeVisible()}},H={name:`Claim being deleted`,args:{claim:{...F(),metadata:{...F().metadata,deletionTimestamp:`2026-11-15T11:00:00Z`}}}},U={name:`Write mode not known`,args:{writeMode:void 0}},W=[`Running`,`SuspendOpen`,`Suspended`,`NotRead`,`KeycloakSuspendOpen`,`ClaimDeleting`,`ChangesOff`],L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
  name: "Running plane"
}`,...L.parameters?.docs?.source}}},R.parameters={...R.parameters,docs:{...R.parameters?.docs,source:{originalSource:`{
  name: "Suspend open",
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", {
      name: "Suspend"
    }));
    await expect(canvas.getByRole("button", {
      name: "Suspend the management plane"
    })).toBeVisible();
  }
}`,...R.parameters?.docs?.source}}},z.parameters={...z.parameters,docs:{...z.parameters?.docs,source:{originalSource:`{
  name: "Suspended, with Resume",
  args: {
    claim: claim(true)
  }
}`,...z.parameters?.docs?.source}}},B.parameters={...B.parameters,docs:{...B.parameters?.docs,source:{originalSource:`{
  name: "Plane not read",
  args: {
    unread: ["CamundaManagementCluster"]
  }
}`,...B.parameters?.docs?.source}}},V.parameters={...V.parameters,docs:{...V.parameters?.docs,source:{originalSource:`{
  name: "Suspend open, own Keycloak",
  args: {
    ownKeycloak: true
  },
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", {
      name: "Suspend"
    }));
    await expect(canvas.getByText(/sign in at the Keycloak of this plane/)).toBeVisible();
  }
}`,...V.parameters?.docs?.source}}},H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
  name: "Claim being deleted",
  args: {
    claim: {
      ...claim(),
      metadata: {
        ...claim().metadata,
        deletionTimestamp: "2026-11-15T11:00:00Z"
      }
    } as Resource<"CamundaManagementClusterClaim">
  }
}`,...H.parameters?.docs?.source}}},U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
  name: "Write mode not known",
  args: {
    writeMode: undefined
  }
}`,...U.parameters?.docs?.source}}}})))()}G();export{U as ChangesOff,H as ClaimDeleting,V as KeycloakSuspendOpen,B as NotRead,L as Running,R as SuspendOpen,z as Suspended,W as __namedExportsOrder,I as default};