import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{l as r,o as i}from"./iframe-BqIb-xGg.js";import{n as a,t as o}from"./circle-alert-DfT8pSui.js";import{n as s,r as c,t as l}from"./toast-MHa2aAEv.js";import{n as u,t as d}from"./review-panel-nx3Fdix3.js";import{D as f,t as p}from"./policy-EofxYXuk.js";import{s as m,t as h}from"./fields-xUNWSDV-.js";import{n as g,t as _}from"./diff-BLIgMNCc.js";import{o as v,r as y}from"./fixtures-DU7FxC4G.js";import{n as b,t as x}from"./diff-view-C5YmqhpV.js";import{c as S,d as C,f as w,i as T,m as E,o as D,s as O,t as k,u as A}from"./session-CZCGo5UT.js";function j({session:e,mode:t,intent:n,impacts:r,caution:i,confirm:a,mergedLines:s,preview:c,apply:l,show:u,onWritten:f,onRefused:p,onCancel:m}){let[h,_]=(0,L.useState)(),[v,y]=(0,L.useState)(),[b,C]=(0,L.useState)(!1),w=(0,L.useRef)(!1),T=(0,L.useRef)(c),E=(0,L.useRef)(p);(0,L.useEffect)(()=>{T.current=c,E.current=p}),(0,L.useEffect)(()=>{let t=!0;return T.current(O(e,n)).catch(e=>({objects:[],errors:[{code:`Unavailable`,message:String(e)}]})).then(r=>{if(!t)return;_({session:e,intent:n,result:r});let i=D(r,e);!i.ok&&i.fields.length>0&&E.current?.(i.fields)}),()=>{t=!1}},[e,n]);let k=h?.session===e&&h.intent===n?h.result:void 0,A=k&&D(k,e),j=v?.session===e?v.outcome:void 0,P=!A||!A.ok||j!==void 0&&!j.ok&&j.stale,F=[...M(A),...M(j),...k?N(e,k):[]];async function R(){if(!(w.current||P)){w.current=!0,C(!0);try{let t=await l(O(e,n)).catch(e=>({ok:!1,errors:[{code:`Unavailable`,message:String(e)}]}));if(t.ok){u(t,`The claim ${e.claim.metadata.name} is saved.`),f?.();return}let r=D(t,e);y({session:e,outcome:r}),!r.ok&&r.fields.length>0&&p?.(r.fields)}finally{w.current=!1,C(!1)}}}return(0,I.jsx)(d,{lines:g(e.claim,S(e)),details:s&&(0,I.jsx)(x,{label:`Changes to the cluster`,lines:s}),effects:r,caution:i,notices:F.length>0&&(0,I.jsxs)(`div`,{role:`alert`,className:`flex gap-3 rounded-md border border-bad/40 bg-bad/10 p-3 text-sm text-text`,children:[(0,I.jsx)(o,{"aria-hidden":!0,className:`mt-0.5 size-4 shrink-0 text-bad`}),(0,I.jsx)(`ul`,{className:`flex flex-col gap-1`,children:F.map((e,t)=>(0,I.jsx)(`li`,{children:e},t))})]}),mode:t,confirmLabel:a?.label??`Save`,gitConfirmLabel:a?.gitLabel,danger:a?.danger,typed:a?.typed,pending:b||P,onConfirm:R,onCancel:m})}function M(e){return!e||e.ok?[]:[...e.messages,...e.fields.map(e=>e.path?`${e.path}: ${e.message}`:e.message)]}function N(e,t){let{kind:n,metadata:r}=e.claim,i=t.objects.find(e=>e.ref.kind===n&&e.ref.namespace===r.namespace&&e.ref.name===r.name)?.after;if(!i)return[];let a=S(e);return[...e.pending.values()].filter(t=>{if(!(`reset`in t))return!1;let n=P(e.claim,t.steps);if(n)return E(i,n)!==void 0;let r=t.steps.slice(0,F(e.claim,t.steps)),o=P(e.claim,r);return o!==void 0&&!f(E(i,o),E(a,o))}).map(e=>`Another field manager keeps ${e.path}, so the reset does not remove it.`)}function P(e,t){let n=[],r=e;for(let e of t){let t=`index`in e&&Array.isArray(r)?m(r[e.index],e.index):e;if(`index`in t)return;n.push(t),r=E(r,[e])}return n}function F(e,t){let n=0;for(;n<t.length&&P(e,t.slice(0,n+1));)n++;return n}var I,L;function R(){return(R=e((()=>{I=n(),a(),L=t(),_(),p(),h(),k(),b(),u(),j.__docgenInfo={description:`ClaimReview is the review of the pending changes of a session: the diff
of the claim, the merged diff of the cluster when mergedLines is given,
the impacts, the caution, and the confirm. It previews the change of the session once
for each session and intent, and the confirm stays disabled until that
preview is clean. A confirm sends one write. An ok write goes to show.
Then it calls onWritten, and the parent closes the session. A refused
preview or write shows its errors and gives the errors of pending fields
to onRefused. It never changes the session.`,methods:[],displayName:`ClaimReview`,props:{session:{required:!0,tsType:{name:`Session`,elements:[{name:`C`}],raw:`Session<C>`},description:``},mode:{required:!0,tsType:{name:`WriteMode`},description:``},intent:{required:!0,tsType:{name:`string`},description:`intent is the commit subject in Git mode.`},impacts:{required:!0,tsType:{name:`Array`,elements:[{name:`string`}],raw:`string[]`},description:``},caution:{required:!1,tsType:{name:`ReactNode`},description:`caution is a warning about the change, such as a backup that it makes fail.`},confirm:{required:!1,tsType:{name:`signature`,type:`object`,raw:`{
  label?: string;
  gitLabel?: string;
  danger?: boolean;
  typed?: string;
}`,signature:{properties:[{key:`label`,value:{name:`string`,required:!1}},{key:`gitLabel`,value:{name:`string`,required:!1}},{key:`danger`,value:{name:`boolean`,required:!1}},{key:`typed`,value:{name:`string`,required:!1}}]}},description:``},mergedLines:{required:!1,tsType:{name:`Array`,elements:[{name:`DiffLine`}],raw:`DiffLine[]`},description:``},preview:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}}},description:``},apply:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}}},description:``},show:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(result: WriteResult, message: string) => void`,signature:{arguments:[{type:{name:`WriteResult`},name:`result`},{type:{name:`string`},name:`message`}],return:{name:`void`}}},description:``},onWritten:{required:!1,tsType:{name:`signature`,type:`function`,raw:`() => void`,signature:{arguments:[],return:{name:`void`}}},description:``},onRefused:{required:!1,tsType:{name:`signature`,type:`function`,raw:`(fields: FieldError[]) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`FieldError`}],raw:`FieldError[]`},name:`fields`}],return:{name:`void`}}},description:``},onCancel:{required:!1,tsType:{name:`signature`,type:`function`,raw:`() => void`,signature:{arguments:[],return:{name:`void`}}},description:``}}}})))()}function z({claim:e,changes:t,mode:n,impacts:r=[],caution:a,confirm:o,mergedLines:s,preview:u={objects:[],errors:[]},result:d={ok:!0,mode:`direct`}}){let f=(0,V.useMemo)(()=>{let n=A({claim:e,resource:{metadata:{labels:{[i]:e.metadata.name}}},locks:()=>void 0});for(let{steps:e,value:r}of t){let t=r===void 0?C(n,e):w(n,e,r);`session`in t&&(n=t.session)}return n},[e,t]),{toast:p,show:m,dismiss:h}=c();return(0,B.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,B.jsx)(j,{session:f,mode:n,intent:`Change the claim ${e.metadata.name}`,impacts:r,caution:a,confirm:o,mergedLines:s,preview:async()=>u,apply:async()=>d,show:m}),p&&(0,B.jsx)(l,{result:p,onDismiss:h})]})}var B,V;function H(){return(H=e((()=>{B=n(),V=t(),r(),k(),R(),s(),z.__docgenInfo={description:`ClaimReviewStory shows ClaimReview in the gallery. A story passes no
event handler, so this wrapper opens the session over claim with
changes, where an undefined value is a reset, and answers the preview
and the write with the scripted preview and result.`,methods:[],displayName:`ClaimReviewStory`,props:{claim:{required:!0,tsType:{name:`ClaimObject`},description:``},changes:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ steps: Step[]; value?: unknown }`,signature:{properties:[{key:`steps`,value:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`,required:!0}},{key:`value`,value:{name:`unknown`,required:!1}}]}}],raw:`{ steps: Step[]; value?: unknown }[]`},description:``},mode:{required:!0,tsType:{name:`WriteMode`},description:``},impacts:{required:!1,tsType:{name:`Array`,elements:[{name:`string`}],raw:`string[]`},description:``,defaultValue:{value:`[]`,computed:!1}},caution:{required:!1,tsType:{name:`ReactNode`},description:``},confirm:{required:!1,tsType:{name:`signature`,type:`object`,raw:`{
  label?: string;
  gitLabel?: string;
  danger?: boolean;
  typed?: string;
}`,signature:{properties:[{key:`label`,value:{name:`string`,required:!1}},{key:`gitLabel`,value:{name:`string`,required:!1}},{key:`danger`,value:{name:`boolean`,required:!1}},{key:`typed`,value:{name:`string`,required:!1}}]}},description:``},mergedLines:{required:!1,tsType:{name:`Array`,elements:[{name:`DiffLine`}],raw:`DiffLine[]`},description:``},preview:{required:!1,tsType:{name:`Preview`},description:``,defaultValue:{value:`{ objects: [], errors: [] }`,computed:!1}},result:{required:!1,tsType:{name:`WriteResult`},description:``,defaultValue:{value:`{ ok: true, mode: "direct" }`,computed:!1}}}}})))()}var U,W,G,K,q,J,Y,X;function Z(){return(Z=e((()=>{k(),H(),y(),U=[{field:`spec`},{field:`cluster`},{field:`zeebe`},{field:`replicas`}],W={title:`fleet/ClaimReview`,component:z,args:{claim:{...v,metadata:{...v.metadata,resourceVersion:`41`}},changes:[{steps:U,value:5}],mode:`direct`,impacts:[`Broker pods restart.`]}},G={name:`Direct`},K={name:`Git`,args:{mode:`git`}},q={name:`With caution`,args:{caution:`A backup of orders runs now. The rollout makes it fail.`}},J={name:`Preview refused`,args:{changes:[{steps:U,value:0}],impacts:[],preview:{objects:[],errors:[{code:`Invalid`,message:`Invalid value: 0: must be greater than or equal to 1`,path:`spec.cluster.zeebe.replicas`}]}}},Y={name:`Stale`,args:{preview:{objects:[],errors:[{code:`Stale`,message:T}]}}},X=[`Direct`,`Git`,`WithCaution`,`PreviewRefused`,`Stale`],G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "Direct"
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "Git",
  args: {
    mode: "git"
  }
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "With caution",
  args: {
    caution: "A backup of orders runs now. The rollout makes it fail."
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Preview refused",
  args: {
    changes: [{
      steps: replicas,
      value: 0
    }],
    impacts: [],
    preview: {
      objects: [],
      errors: [{
        code: "Invalid",
        message: "Invalid value: 0: must be greater than or equal to 1",
        path: "spec.cluster.zeebe.replicas"
      }]
    }
  }
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Stale",
  args: {
    preview: {
      objects: [],
      errors: [{
        code: "Stale",
        message: STALE_CLAIM_MESSAGE
      }]
    }
  }
}`,...Y.parameters?.docs?.source}}}})))()}Z();export{G as Direct,K as Git,J as PreviewRefused,Y as Stale,q as WithCaution,X as __namedExportsOrder,W as default};