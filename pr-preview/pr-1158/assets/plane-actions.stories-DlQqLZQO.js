import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{x as r,y as i}from"./labels-Be1Q9KU6.js";import{c as a,s as o}from"./iframe-B2DxHM4S.js";import{n as ee}from"./refs-CNug01Rr.js";import{rt as s,t as c,ut as l}from"./session-BmEcT8yX.js";import{a as u,n as d}from"./ownership-CAEiO32v.js";import{n as f,t as p}from"./link-DOLMGIR2.js";import{n as m,r as h,t as g}from"./button-Ci8lqj97.js";import{n as _,t as v}from"./inline-confirm-9ZznqnEs.js";import{i as y,n as b,r as x,t as S}from"./take-over-choice-C__T6GR5.js";import{n as C,r as te,t as w}from"./toast-CW321st2.js";import{r as T}from"./claim-edit-DB1St4nX.js";import{n as E,t as D}from"./git-waiting-line-z5pGZ9JE.js";function O(e,t){let{session:n}=l({claim:e,pending:new Map,locks:()=>void 0},[{field:`spec`},{field:`suspend`}],t),r=e.metadata.name??``;return s(n,`${t?`Suspend`:`Resume`} the management plane ${r}`)}function k(){return(k=e((()=>{c()})))()}function A({claim:e,served:t,ownKeycloak:n=!1,apply:r,preview:o,deleteHref:s,writeMode:c,unread:l=[],now:d}){let{toast:p,show:h,dismiss:_}=te(),{pushes:b}=a(),{namespace:C,name:T=``,resourceVersion:E}=e.metadata??{},k=ee({kind:`CamundaManagementClusterClaim`,namespace:C,name:T}),A=b.find(e=>e.key===k),F=e.metadata?.annotations?.[i],I=A!==void 0&&(`deleted`in A||A.revision!==F),[L,R]=(0,N.useState)(!1),[z,B]=(0,N.useState)(),V=l.length>0||L||I||z!==void 0&&z===E,H=e.metadata?.deletionTimestamp!=null,U=V||H||c===void 0,[W,G]=(0,N.useState)(),K=W?.version===E?W:void 0,q=(0,N.useRef)(null),J=(0,N.useRef)(!1);(0,N.useEffect)(()=>{J.current&&(J.current=!1,q.current?.querySelector(`button`)?.focus())});let Y=async(t,n=O(e,t))=>{if(U)return;R(!0),G(void 0);let i=await r(n).catch(e=>({ok:!1,errors:[j(e)]})),a;if(!i.ok&&c===`direct`&&n.takeOver===void 0&&i.errors.some(e=>e.code===`Conflict`)){let e=await o(n).catch(e=>({objects:[],errors:[j(e)]})),r=e.ownership??[];r.length>0&&(a={suspend:t,change:n,ownership:r,errors:y(e.errors,r),chosen:!1,version:E})}R(!1),a?G(a):(i.ok&&B(E),h(i,`${t?`Suspended`:`Resumed`} the management plane ${T}.`))},X=e.spec.suspend===!0,Z=W!==void 0&&W.version!==E?W.suspend:void 0,Q=[`Every workload of the plane stops. Sign-in to Console, Web Modeler, and Optimize stops for every cluster that this plane serves (${t}).`,...n?[P]:[]],$=[`The served clusters keep running, and the databases keep the data of the plane.`];return(0,M.jsxs)(`div`,{className:`flex max-w-xl flex-col items-end gap-2`,children:[(0,M.jsxs)(`div`,{ref:q,className:`flex flex-wrap items-start justify-end gap-2`,children:[K&&!U?(0,M.jsxs)(v,{defaultOpen:!0,danger:K.suspend,triggerLabel:K.suspend?`Suspend`:`Resume`,confirmLabel:`${K.suspend?`Suspend`:`Resume`} the management plane`,goes:K.suspend?Q:[],stays:K.suspend?$:[],confirmDisabled:K.errors.length>0||x(K.ownership,K.chosen),onCancel:()=>{J.current=!0,G(void 0)},onConfirm:()=>void Y(K.suspend,u(K.change,K.ownership)),children:[(0,M.jsx)(S,{ownership:K.ownership,chosen:K.chosen,onChoose:e=>G({...K,chosen:e})}),K.errors.length>0&&(0,M.jsx)(`ul`,{role:`alert`,className:`flex flex-col gap-1 text-bad`,children:K.errors.map((e,t)=>(0,M.jsx)(`li`,{children:e.message},t))})]},`take-over-${K.suspend}`):X||U?(0,M.jsx)(g,{variant:`outline`,disabled:U,onClick:()=>void Y(!1),children:X?`Resume`:`Suspend`}):(0,M.jsx)(v,{triggerLabel:`Suspend`,confirmLabel:`Suspend the management plane`,goes:Q,stays:$,onConfirm:()=>void Y(!0)}),V?(0,M.jsx)(g,{variant:`outline`,disabled:!0,children:`Delete`}):(0,M.jsx)(f,{href:s,className:m({variant:`outline`}),children:`Delete`})]}),Z!==void 0&&(0,M.jsxs)(`p`,{className:`text-right text-sm text-muted`,children:[`The plane changed.`,Z!==X&&` Select ${Z?`Suspend`:`Resume`} again.`]}),c===void 0&&(0,M.jsx)(`p`,{className:`text-right text-sm text-muted`,children:`Changes are off: the fleet manager could not read its settings.`}),H&&(0,M.jsx)(`p`,{className:`text-right text-sm text-muted`,children:`Suspend and Resume are off while the claim is being deleted.`}),l.length>0&&(0,M.jsxs)(`p`,{className:`text-right text-sm text-muted`,children:[`Suspend and Delete are possible when the fleet manager can read`,` `,l.join(`, `),`.`]}),(0,M.jsx)(D,{pushed:A,liveRevision:F,now:d}),p&&(0,M.jsx)(w,{result:p,onDismiss:_})]})}function j(e){return{code:`Unavailable`,message:String(e)}}var M,N,P;function F(){return(F=e((()=>{M=n(),p(),N=t(),T(),o(),E(),_(),b(),C(),h(),r(),k(),d(),P=`Each cluster that uses the Keycloak of this plane as its OIDC issuer loses sign-in: its users and API clients cannot sign in or get tokens until the plane resumes.`,A.__docgenInfo={description:`PlaneActions holds Suspend or Resume, and Delete, of a plane that a
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
GitPushesProvider and a QueryClientProvider above it.

In direct mode, a write that fails with a Conflict previews the same
change. When the preview names fields of other tools, a confirm shows
them in TakeOverChoice, and its confirm writes the change with
withTakeOver once takeOverOpen is false. The choice starts off for each
write. Otherwise the refusal shows as before.`,methods:[],displayName:`PlaneActions`,props:{claim:{required:!0,tsType:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaManagementClusterClaim"`}],raw:`Resource<"CamundaManagementClusterClaim">`},description:``},served:{required:!0,tsType:{name:`number`},description:``},ownKeycloak:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`false`,computed:!1}},apply:{required:!0,tsType:{name:`ClaimActions["apply"]`,raw:`ClaimActions["apply"]`},description:``},preview:{required:!0,tsType:{name:`ClaimActions["preview"]`,raw:`ClaimActions["preview"]`},description:``},deleteHref:{required:!0,tsType:{name:`string`},description:``},writeMode:{required:!0,tsType:{name:`union`,raw:`WriteMode | undefined`,elements:[{name:`WriteMode`},{name:`undefined`}]},description:``},unread:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}var I,L,R,z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{F(),{expect:I,fn:L,userEvent:R,within:z}=__STORYBOOK_MODULE_TEST__,B=e=>({apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaManagementClusterClaim`,metadata:{namespace:`platform`,name:`plane-a`,resourceVersion:`7`},spec:{environments:[`platform`],...e&&{suspend:e}}}),V={title:`planes/PlaneActions`,component:A,args:{claim:B(),served:3,apply:L(async()=>({ok:!0,mode:`direct`})),preview:L(async()=>({objects:[],errors:[]})),deleteHref:`/fleet/planes/platform/plane-a/delete`,writeMode:`direct`,now:`2026-11-15T12:00:00Z`}},H={name:`Running plane`},U={name:`Suspend open`,play:async({canvasElement:e})=>{let t=z(e);await R.click(t.getByRole(`button`,{name:`Suspend`})),await I(t.getByRole(`button`,{name:`Suspend the management plane`})).toBeVisible()}},W={name:`Suspended, with Resume`,args:{claim:B(!0)}},G={name:`Plane not read`,args:{unread:[`CamundaManagementCluster`]}},K={name:`Suspend open, own Keycloak`,args:{ownKeycloak:!0},play:async({canvasElement:e})=>{let t=z(e);await R.click(t.getByRole(`button`,{name:`Suspend`})),await I(t.getByText(/uses the Keycloak of this plane as its OIDC issuer/)).toBeVisible()}},q={name:`Claim being deleted`,args:{claim:{...B(),metadata:{...B().metadata,deletionTimestamp:`2026-11-15T11:00:00Z`}}}},J={name:`Write mode not known`,args:{writeMode:void 0}},Y={apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaManagementClusterClaim`,namespace:`platform`,name:`plane-a`},X={code:`Conflict`,message:`conflict with "kubectl"`,path:`spec.suspend`,manager:`kubectl`,ref:Y},Z={name:`Suspend asks to take over`,args:{apply:L(async e=>e.takeOver?{ok:!0,mode:`direct`}:{ok:!1,errors:[X]}),preview:L(async()=>({objects:[],errors:[X],ownership:[{ref:Y,changed:[{path:`spec.suspend`,steps:[{field:`spec`},{field:`suspend`}],managers:[{name:`kubectl`,operation:`Apply`,kind:`other`,known:!0}]}],cleared:[]}]}))},play:async({canvasElement:e})=>{let t=z(e);await R.click(t.getByRole(`button`,{name:`Suspend`})),await R.click(t.getByRole(`button`,{name:`Suspend the management plane`})),await I(await t.findByRole(`group`,{name:`Fields that another tool set`})).toBeVisible(),await I(t.getByRole(`button`,{name:`Suspend the management plane`})).toBeDisabled()}},Q=[`Running`,`SuspendOpen`,`Suspended`,`NotRead`,`KeycloakSuspendOpen`,`ClaimDeleting`,`ChangesOff`,`SuspendAsksToTakeOver`],H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
  name: "Running plane"
}`,...H.parameters?.docs?.source}}},U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
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
}`,...U.parameters?.docs?.source}}},W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "Suspended, with Resume",
  args: {
    claim: claim(true)
  }
}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "Plane not read",
  args: {
    unread: ["CamundaManagementCluster"]
  }
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
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
    await expect(canvas.getByText(/uses the Keycloak of this plane as its OIDC issuer/)).toBeVisible();
  }
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
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
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Write mode not known",
  args: {
    writeMode: undefined
  }
}`,...J.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Suspend asks to take over",
  args: {
    // kubectl set spec.suspend, so a write without the take-over conflicts.
    apply: fn(async (change: Change): Promise<WriteResult> => change.takeOver ? {
      ok: true,
      mode: "direct"
    } : {
      ok: false,
      errors: [conflict]
    }),
    preview: fn(async (): Promise<Preview> => ({
      objects: [],
      errors: [conflict],
      ownership: [{
        ref,
        changed: [{
          path: "spec.suspend",
          steps: [{
            field: "spec"
          }, {
            field: "suspend"
          }],
          managers: [{
            name: "kubectl",
            operation: "Apply",
            kind: "other" as const,
            known: true
          }]
        }],
        cleared: []
      }]
    }))
  },
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", {
      name: "Suspend"
    }));
    await userEvent.click(canvas.getByRole("button", {
      name: "Suspend the management plane"
    }));
    await expect(await canvas.findByRole("group", {
      name: "Fields that another tool set"
    })).toBeVisible();
    await expect(canvas.getByRole("button", {
      name: "Suspend the management plane"
    })).toBeDisabled();
  }
}`,...Z.parameters?.docs?.source}}}})))()}$();export{J as ChangesOff,q as ClaimDeleting,K as KeycloakSuspendOpen,G as NotRead,H as Running,Z as SuspendAsksToTakeOver,U as SuspendOpen,W as Suspended,Q as __namedExportsOrder,V as default};