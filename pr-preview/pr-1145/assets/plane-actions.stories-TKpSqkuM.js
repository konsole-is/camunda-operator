import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{x as r,y as i}from"./labels-Be1Q9KU6.js";import{c as a,s as o}from"./iframe-DXUW18vg.js";import{G as s,Q as c,t as l}from"./session-DbnaU5F3.js";import{n as u}from"./refs-CNug01Rr.js";import{n as d,t as f}from"./link-DOLMGIR2.js";import{n as p,r as m,t as h}from"./button-CvkcydkV.js";import{n as g,t as _}from"./inline-confirm-DambzyQn.js";import{n as v,r as y,t as b}from"./toast-J0QGHROO.js";import{n as x,t as S}from"./git-waiting-line-z5pGZ9JE.js";function C(e,t){let{session:n}=c({claim:e,pending:new Map,locks:()=>void 0},[{field:`spec`},{field:`suspend`}],t),r=e.metadata.name??``;return s(n,`${t?`Suspend`:`Resume`} the management plane ${r}`)}function w(){return(w=e((()=>{l()})))()}function T({claim:e,served:t,apply:n,deleteHref:r,unread:o=[],now:s}){let{toast:c,show:l,dismiss:f}=y(),{pushes:m}=a(),{namespace:g,name:v=``,resourceVersion:x}=e.metadata??{},w=u({kind:`CamundaManagementClusterClaim`,namespace:g,name:v}),T=m.find(e=>e.key===w&&!(`deleted`in e)),O=e.metadata?.annotations?.[i],k=T!==void 0&&T.revision!==O,[A,j]=(0,D.useState)(!1),[M,N]=(0,D.useState)(),P=o.length>0||A||k||M!==void 0&&M===x,F=async t=>{if(P)return;j(!0);let r=await n(C(e,t)).catch(e=>({ok:!1,errors:[{code:`Unavailable`,message:String(e)}]}));j(!1),r.ok&&N(x),l(r,`${t?`Suspended`:`Resumed`} the management plane ${v}.`)},I=e.spec.suspend===!0;return(0,E.jsxs)(`div`,{className:`flex max-w-xl flex-col items-end gap-2`,children:[(0,E.jsxs)(`div`,{className:`flex flex-wrap items-start justify-end gap-2`,children:[I||P?(0,E.jsx)(h,{variant:`outline`,disabled:P,onClick:()=>void F(!1),children:I?`Resume`:`Suspend`}):(0,E.jsx)(_,{triggerLabel:`Suspend`,confirmLabel:`Suspend the management plane`,goes:[`Every workload of the plane stops. Sign-in to Console, Web Modeler, and Optimize stops for every cluster that this plane serves (${t}).`],stays:[`The served clusters keep running, and the databases keep the data of the plane.`],onConfirm:()=>void F(!0)}),P?(0,E.jsx)(h,{variant:`outline`,disabled:!0,children:`Delete`}):(0,E.jsx)(d,{href:r,className:p({variant:`outline`}),children:`Delete`})]}),o.length>0&&(0,E.jsxs)(`p`,{className:`text-right text-sm text-muted`,children:[`Suspend and Delete are possible when the fleet manager can read`,` `,o.join(`, `),`.`]}),(0,E.jsx)(S,{pushed:T,liveRevision:O,now:s}),c&&(0,E.jsx)(b,{result:c,onDismiss:f})]})}var E,D;function O(){return(O=e((()=>{E=n(),f(),D=t(),o(),x(),g(),v(),m(),r(),w(),T.__docgenInfo={description:`PlaneActions holds Suspend or Resume, and Delete, of a plane that a
claim made. Suspend asks in place and says that sign-in stops for every
cluster that the plane serves; served is that number. Resume writes at
once. Each write sets spec.suspend through apply, with the
resourceVersion of claim as its precondition. Delete links to deleteHref.
Both stay off while unread names a kind that the page could not read,
while a write runs, until the claim shows a newer version after a write,
and while a pushed change of the claim waits for GitOps; an open confirm
then closes. It needs a GitPushesProvider and a QueryClientProvider
above it.`,methods:[],displayName:`PlaneActions`,props:{claim:{required:!0,tsType:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaManagementClusterClaim"`}],raw:`Resource<"CamundaManagementClusterClaim">`},description:``},served:{required:!0,tsType:{name:`number`},description:``},apply:{required:!0,tsType:{name:`ClaimActions["apply"]`,raw:`ClaimActions["apply"]`},description:``},deleteHref:{required:!0,tsType:{name:`string`},description:``},unread:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}var k,A,j,M,N,P,F,I,L,R,z;function B(){return(B=e((()=>{O(),{expect:k,fn:A,userEvent:j,within:M}=__STORYBOOK_MODULE_TEST__,N=e=>({apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaManagementClusterClaim`,metadata:{namespace:`platform`,name:`plane-a`,resourceVersion:`7`},spec:{environments:[`platform`],...e&&{suspend:e}}}),P={title:`planes/PlaneActions`,component:T,args:{claim:N(),served:3,apply:A(async()=>({ok:!0,mode:`direct`})),deleteHref:`/fleet/planes/platform/plane-a/delete`,now:`2026-11-15T12:00:00Z`}},F={name:`Running plane`},I={name:`Suspend open`,play:async({canvasElement:e})=>{let t=M(e);await j.click(t.getByRole(`button`,{name:`Suspend`})),await k(t.getByRole(`button`,{name:`Suspend the management plane`})).toBeVisible()}},L={name:`Suspended, with Resume`,args:{claim:N(!0)}},R={name:`Plane not read`,args:{unread:[`CamundaManagementCluster`]}},z=[`Running`,`SuspendOpen`,`Suspended`,`NotRead`],F.parameters={...F.parameters,docs:{...F.parameters?.docs,source:{originalSource:`{
  name: "Running plane"
}`,...F.parameters?.docs?.source}}},I.parameters={...I.parameters,docs:{...I.parameters?.docs,source:{originalSource:`{
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
}`,...I.parameters?.docs?.source}}},L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
  name: "Suspended, with Resume",
  args: {
    claim: claim(true)
  }
}`,...L.parameters?.docs?.source}}},R.parameters={...R.parameters,docs:{...R.parameters?.docs,source:{originalSource:`{
  name: "Plane not read",
  args: {
    unread: ["CamundaManagementCluster"]
  }
}`,...R.parameters?.docs?.source}}}})))()}B();export{R as NotRead,F as Running,I as SuspendOpen,L as Suspended,z as __namedExportsOrder,P as default};