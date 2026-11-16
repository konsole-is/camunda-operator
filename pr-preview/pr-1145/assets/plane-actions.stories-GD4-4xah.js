import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{x as r,y as i}from"./labels-Be1Q9KU6.js";import{c as a,s as o}from"./iframe-DCNVgSfG.js";import{G as s,Q as c,t as l}from"./session-DbnaU5F3.js";import{n as u}from"./refs-CNug01Rr.js";import{n as d,t as f}from"./link-DOLMGIR2.js";import{n as p,r as m,t as h}from"./button-C3j-aT_T.js";import{n as g,t as _}from"./inline-confirm-DS-r0kUe.js";import{n as v,r as y,t as b}from"./toast-DN0teXkR.js";import{n as x,t as S}from"./git-waiting-line-z5pGZ9JE.js";function C(e,t){let{session:n}=c({claim:e,pending:new Map,locks:()=>void 0},[{field:`spec`},{field:`suspend`}],t),r=e.metadata.name??``;return s(n,`${t?`Suspend`:`Resume`} the management plane ${r}`)}function w(){return(w=e((()=>{l()})))()}function T({claim:e,served:t,ownKeycloak:n=!1,apply:r,deleteHref:o,unread:s=[],now:c}){let{toast:l,show:f,dismiss:m}=y(),{pushes:g}=a(),{namespace:v,name:x=``,resourceVersion:w}=e.metadata??{},T=u({kind:`CamundaManagementClusterClaim`,namespace:v,name:x}),k=g.find(e=>e.key===T&&!(`deleted`in e)),A=e.metadata?.annotations?.[i],j=k!==void 0&&k.revision!==A,[M,N]=(0,D.useState)(!1),[P,F]=(0,D.useState)(),I=s.length>0||M||j||P!==void 0&&P===w,L=e.metadata?.deletionTimestamp!=null,R=I||L,z=async t=>{if(R)return;N(!0);let n=await r(C(e,t)).catch(e=>({ok:!1,errors:[{code:`Unavailable`,message:String(e)}]}));N(!1),n.ok&&F(w),f(n,`${t?`Suspended`:`Resumed`} the management plane ${x}.`)},B=e.spec.suspend===!0;return(0,E.jsxs)(`div`,{className:`flex max-w-xl flex-col items-end gap-2`,children:[(0,E.jsxs)(`div`,{className:`flex flex-wrap items-start justify-end gap-2`,children:[B||R?(0,E.jsx)(h,{variant:`outline`,disabled:R,onClick:()=>void z(!1),children:B?`Resume`:`Suspend`}):(0,E.jsx)(_,{triggerLabel:`Suspend`,confirmLabel:`Suspend the management plane`,goes:[`Every workload of the plane stops. Sign-in to Console, Web Modeler, and Optimize stops for every cluster that this plane serves (${t}).`,...n&&t>0?[O]:[]],stays:[`The served clusters keep running, and the databases keep the data of the plane.`],onConfirm:()=>void z(!0)}),I?(0,E.jsx)(h,{variant:`outline`,disabled:!0,children:`Delete`}):(0,E.jsx)(d,{href:o,className:p({variant:`outline`}),children:`Delete`})]}),L&&(0,E.jsx)(`p`,{className:`text-right text-sm text-muted`,children:`Suspend and Resume are off while the claim is being deleted.`}),s.length>0&&(0,E.jsxs)(`p`,{className:`text-right text-sm text-muted`,children:[`Suspend and Delete are possible when the fleet manager can read`,` `,s.join(`, `),`.`]}),(0,E.jsx)(S,{pushed:k,liveRevision:A,now:c}),l&&(0,E.jsx)(b,{result:l,onDismiss:m})]})}var E,D,O;function k(){return(k=e((()=>{E=n(),f(),D=t(),o(),x(),g(),v(),m(),r(),w(),O=`The served clusters that use OIDC sign in at the Keycloak of this plane. Their users and API clients cannot sign in or get tokens until the plane resumes.`,T.__docgenInfo={description:`PlaneActions holds Suspend or Resume, and Delete, of a plane that a
claim made. Suspend asks in place and says that sign-in stops for every
cluster that the plane serves; served is that number. ownKeycloak says
that the plane runs the Keycloak that its OIDC clusters sign in at.
Resume writes at once. Each write sets spec.suspend through apply, with
the resourceVersion of claim as its precondition. Delete links to
deleteHref. Both stay off while unread names a kind that the page could
not read, while a write runs, until the claim shows a newer version after
a write, and while a pushed change of the claim waits for GitOps; an open
confirm then closes. Suspend and Resume are off while the claim is being
deleted. It needs a GitPushesProvider and a QueryClientProvider above it.`,methods:[],displayName:`PlaneActions`,props:{claim:{required:!0,tsType:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaManagementClusterClaim"`}],raw:`Resource<"CamundaManagementClusterClaim">`},description:``},served:{required:!0,tsType:{name:`number`},description:``},ownKeycloak:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`false`,computed:!1}},apply:{required:!0,tsType:{name:`ClaimActions["apply"]`,raw:`ClaimActions["apply"]`},description:``},deleteHref:{required:!0,tsType:{name:`string`},description:``},unread:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}var A,j,M,N,P,F,I,L,R,z,B,V,H;function U(){return(U=e((()=>{k(),{expect:A,fn:j,userEvent:M,within:N}=__STORYBOOK_MODULE_TEST__,P=e=>({apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaManagementClusterClaim`,metadata:{namespace:`platform`,name:`plane-a`,resourceVersion:`7`},spec:{environments:[`platform`],...e&&{suspend:e}}}),F={title:`planes/PlaneActions`,component:T,args:{claim:P(),served:3,apply:j(async()=>({ok:!0,mode:`direct`})),deleteHref:`/fleet/planes/platform/plane-a/delete`,now:`2026-11-15T12:00:00Z`}},I={name:`Running plane`},L={name:`Suspend open`,play:async({canvasElement:e})=>{let t=N(e);await M.click(t.getByRole(`button`,{name:`Suspend`})),await A(t.getByRole(`button`,{name:`Suspend the management plane`})).toBeVisible()}},R={name:`Suspended, with Resume`,args:{claim:P(!0)}},z={name:`Plane not read`,args:{unread:[`CamundaManagementCluster`]}},B={name:`Suspend open, own Keycloak`,args:{ownKeycloak:!0},play:async({canvasElement:e})=>{let t=N(e);await M.click(t.getByRole(`button`,{name:`Suspend`})),await A(t.getByText(/sign in at the Keycloak of this plane/)).toBeVisible()}},V={name:`Claim being deleted`,args:{claim:{...P(),metadata:{...P().metadata,deletionTimestamp:`2026-11-15T11:00:00Z`}}}},H=[`Running`,`SuspendOpen`,`Suspended`,`NotRead`,`KeycloakSuspendOpen`,`ClaimDeleting`],I.parameters={...I.parameters,docs:{...I.parameters?.docs,source:{originalSource:`{
  name: "Running plane"
}`,...I.parameters?.docs?.source}}},L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
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
}`,...L.parameters?.docs?.source}}},R.parameters={...R.parameters,docs:{...R.parameters?.docs,source:{originalSource:`{
  name: "Suspended, with Resume",
  args: {
    claim: claim(true)
  }
}`,...R.parameters?.docs?.source}}},z.parameters={...z.parameters,docs:{...z.parameters?.docs,source:{originalSource:`{
  name: "Plane not read",
  args: {
    unread: ["CamundaManagementCluster"]
  }
}`,...z.parameters?.docs?.source}}},B.parameters={...B.parameters,docs:{...B.parameters?.docs,source:{originalSource:`{
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
}`,...B.parameters?.docs?.source}}},V.parameters={...V.parameters,docs:{...V.parameters?.docs,source:{originalSource:`{
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
}`,...V.parameters?.docs?.source}}}})))()}U();export{V as ClaimDeleting,B as KeycloakSuspendOpen,z as NotRead,I as Running,L as SuspendOpen,R as Suspended,H as __namedExportsOrder,F as default};