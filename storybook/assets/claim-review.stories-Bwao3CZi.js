import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{a as r,h as i}from"./labels-a4ptf10d.js";import{n as a,r as o,t as s}from"./toast-YCUlGkv4.js";import{o as c,r as l}from"./fixtures-DU7FxC4G.js";import{_ as u,g as d,i as f,m as p,t as m}from"./session-BrIL8yoW.js";import{n as h,t as g}from"./claim-review-C8k85Sh6.js";function _({claim:e,changes:t,mode:n,impacts:i=[],caution:a,confirm:c,mergedLines:l,preview:f={objects:[],errors:[]},result:m={ok:!0,mode:`direct`}}){let h=(0,y.useMemo)(()=>{let n=p({claim:e,resource:{metadata:{labels:{[r]:e.metadata.name}}},locks:()=>void 0});for(let{steps:e,value:r}of t){let t=r===void 0?d(n,e):u(n,e,r);`session`in t&&(n=t.session)}return n},[e,t]),{toast:_,show:b,dismiss:x}=o();return(0,v.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,v.jsx)(g,{session:h,mode:n,intent:`Change the claim ${e.metadata.name}`,impacts:i,caution:a,confirm:c,mergedLines:l,preview:async()=>f,apply:async()=>m,show:b}),_&&(0,v.jsx)(s,{result:_,onDismiss:x})]})}var v,y;function b(){return(b=e((()=>{v=n(),y=t(),i(),m(),h(),a(),_.__docgenInfo={description:`ClaimReviewStory shows ClaimReview in the gallery. A story passes no
event handler, so this wrapper opens the session over claim with
changes, where an undefined value is a reset, and answers the preview
and the write with the scripted preview and result.`,methods:[],displayName:`ClaimReviewStory`,props:{claim:{required:!0,tsType:{name:`ClaimObject`},description:``},changes:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ steps: Step[]; value?: unknown }`,signature:{properties:[{key:`steps`,value:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`,required:!0}},{key:`value`,value:{name:`unknown`,required:!1}}]}}],raw:`{ steps: Step[]; value?: unknown }[]`},description:``},mode:{required:!0,tsType:{name:`WriteMode`},description:``},impacts:{required:!1,tsType:{name:`Array`,elements:[{name:`string`}],raw:`string[]`},description:``,defaultValue:{value:`[]`,computed:!1}},caution:{required:!1,tsType:{name:`ReactNode`},description:``},confirm:{required:!1,tsType:{name:`signature`,type:`object`,raw:`{
  label?: string;
  gitLabel?: string;
  danger?: boolean;
  typed?: string;
}`,signature:{properties:[{key:`label`,value:{name:`string`,required:!1}},{key:`gitLabel`,value:{name:`string`,required:!1}},{key:`danger`,value:{name:`boolean`,required:!1}},{key:`typed`,value:{name:`string`,required:!1}}]}},description:``},mergedLines:{required:!1,tsType:{name:`Array`,elements:[{name:`DiffLine`}],raw:`DiffLine[]`},description:``},preview:{required:!1,tsType:{name:`Preview`},description:``,defaultValue:{value:`{ objects: [], errors: [] }`,computed:!1}},result:{required:!1,tsType:{name:`WriteResult`},description:``,defaultValue:{value:`{ ok: true, mode: "direct" }`,computed:!1}}}}})))()}var x,S,C,w,T,E,D,O;function k(){return(k=e((()=>{m(),b(),l(),x=[{field:`spec`},{field:`cluster`},{field:`zeebe`},{field:`replicas`}],S={title:`fleet/ClaimReview`,component:_,args:{claim:{...c,metadata:{...c.metadata,resourceVersion:`41`}},changes:[{steps:x,value:5}],mode:`direct`,impacts:[`Broker pods restart.`]}},C={name:`Direct`},w={name:`Git`,args:{mode:`git`}},T={name:`With caution`,args:{caution:`A backup of orders runs now. The rollout makes it fail.`}},E={name:`Preview refused`,args:{changes:[{steps:x,value:0}],impacts:[],preview:{objects:[],errors:[{code:`Invalid`,message:`Invalid value: 0: must be greater than or equal to 1`,path:`spec.cluster.zeebe.replicas`}]}}},D={name:`Stale`,args:{preview:{objects:[],errors:[{code:`Stale`,message:f}]}}},O=[`Direct`,`Git`,`WithCaution`,`PreviewRefused`,`Stale`],C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
  name: "Direct"
}`,...C.parameters?.docs?.source}}},w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
  name: "Git",
  args: {
    mode: "git"
  }
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
  name: "With caution",
  args: {
    caution: "A backup of orders runs now. The rollout makes it fail."
  }
}`,...T.parameters?.docs?.source}}},E.parameters={...E.parameters,docs:{...E.parameters?.docs,source:{originalSource:`{
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
}`,...E.parameters?.docs?.source}}},D.parameters={...D.parameters,docs:{...D.parameters?.docs,source:{originalSource:`{
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
}`,...D.parameters?.docs?.source}}}})))()}k();export{C as Direct,w as Git,E as PreviewRefused,D as Stale,T as WithCaution,O as __namedExportsOrder,S as default};