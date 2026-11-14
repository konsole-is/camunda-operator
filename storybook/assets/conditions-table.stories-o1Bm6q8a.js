import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./dist-BTjrb5Vg.js";import{i,n as a,o}from"./status-CBDUVTbC.js";import{n as s,t as c}from"./age-f6c5OgJq.js";import{a as l,i as u,n as d,o as f,r as p,s as m,t as h}from"./table-DGxvrdAu.js";function g({conditions:e,groups:t=[],now:n,label:r=`Conditions`}){return e.length===0&&t.every(e=>e.conditions.length===0)?(0,v.jsx)(`p`,{className:`text-sm text-muted`,children:`No conditions`}):(0,v.jsxs)(h,{label:r,children:[(0,v.jsx)(l,{children:(0,v.jsxs)(f,{children:[(0,v.jsx)(u,{children:`Type`}),(0,v.jsx)(u,{children:`Status`}),(0,v.jsx)(u,{children:`Reason`}),(0,v.jsx)(u,{children:`Message`}),(0,v.jsx)(u,{children:`Age`})]})}),e.length>0&&(0,v.jsx)(d,{children:e.map(e=>(0,v.jsx)(_,{condition:e,now:n},e.type))}),t.map(e=>(0,v.jsxs)(d,{children:[(0,v.jsx)(f,{children:(0,v.jsx)(`th`,{scope:`rowgroup`,colSpan:5,className:`px-2 pt-4 pb-1 text-left font-heading text-sm font-semibold text-text`,children:e.label})}),e.conditions.map(e=>(0,v.jsx)(_,{condition:e,now:n},e.type))]},e.label))]})}function _({condition:e,now:t}){return(0,v.jsxs)(f,{children:[(0,v.jsx)(`th`,{scope:`row`,className:`px-2 py-2 text-left font-medium whitespace-nowrap text-text`,children:e.type}),(0,v.jsx)(p,{className:r(`font-medium`,i[a[e.status]]),children:e.status}),(0,v.jsx)(p,{className:`text-text-2`,children:e.reason??`-`}),(0,v.jsx)(p,{className:`min-w-48 whitespace-normal text-text-2`,children:e.message||`-`}),(0,v.jsx)(p,{className:`text-muted`,children:c(e.lastTransitionTime,t)})]})}var v;function y(){return(y=e((()=>{v=t(),m(),n(),s(),o(),g.__docgenInfo={description:`ConditionsTable lists conditions with their type, status, reason, message,
and age. The conditions without a group come first, then each group under
its label, in the order given. now is the time that the ages count to.`,methods:[],displayName:`ConditionsTable`,props:{conditions:{required:!0,tsType:{name:`Array`,elements:[{name:`intersection`,raw:`ConditionLike & {
  type: string;
  lastTransitionTime?: string | Date;
}`,elements:[{name:`signature`,type:`object`,raw:`{
  status: ConditionStatus;
  reason?: string;
  message?: string;
}`,signature:{properties:[{key:`status`,value:{name:`union`,raw:`"True" | "False" | "Unknown"`,elements:[{name:`literal`,value:`"True"`},{name:`literal`,value:`"False"`},{name:`literal`,value:`"Unknown"`}],required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]}},{name:`signature`,type:`object`,raw:`{
  type: string;
  lastTransitionTime?: string | Date;
}`,signature:{properties:[{key:`type`,value:{name:`string`,required:!0}},{key:`lastTransitionTime`,value:{name:`union`,raw:`string | Date`,elements:[{name:`string`},{name:`Date`}],required:!1}}]}}]}],raw:`ConditionRow[]`},description:``},groups:{required:!1,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ label: string; conditions: ConditionRow[] }`,signature:{properties:[{key:`label`,value:{name:`string`,required:!0}},{key:`conditions`,value:{name:`Array`,elements:[{name:`intersection`,raw:`ConditionLike & {
  type: string;
  lastTransitionTime?: string | Date;
}`,elements:[{name:`signature`,type:`object`,raw:`{
  status: ConditionStatus;
  reason?: string;
  message?: string;
}`,signature:{properties:[{key:`status`,value:{name:`union`,raw:`"True" | "False" | "Unknown"`,elements:[{name:`literal`,value:`"True"`},{name:`literal`,value:`"False"`},{name:`literal`,value:`"Unknown"`}],required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]}},{name:`signature`,type:`object`,raw:`{
  type: string;
  lastTransitionTime?: string | Date;
}`,signature:{properties:[{key:`type`,value:{name:`string`,required:!0}},{key:`lastTransitionTime`,value:{name:`union`,raw:`string | Date`,elements:[{name:`string`},{name:`Date`}],required:!1}}]}}]}],raw:`ConditionRow[]`,required:!0}}]}}],raw:`{ label: string; conditions: ConditionRow[] }[]`},description:``,defaultValue:{value:`[]`,computed:!1}},now:{required:!0,tsType:{name:`string`},description:``},label:{required:!1,tsType:{name:`string`},description:``,defaultValue:{value:`"Conditions"`,computed:!1}}}}})))()}var b,x,S,C,w;function T(){return(T=e((()=>{y(),b={title:`fleet/ConditionsTable`,component:g,args:{now:`2026-10-15T12:00:00Z`,conditions:[]}},x={name:`Not ready condition`,args:{conditions:[{type:`AdminSecretReady`,status:`False`,reason:`SecretMissing`,message:`The Secret orders-admin does not exist.`,lastTransitionTime:`2026-10-15T11:48:00Z`},{type:`MirroredSecretsReady`,status:`True`,reason:`Healthy`,message:``,lastTransitionTime:`2026-10-14T09:00:00Z`}]}},S={name:`Groups`,args:{conditions:[{type:`MirroredSecretsReady`,status:`True`,reason:`Healthy`,lastTransitionTime:`2026-10-14T09:00:00Z`}],groups:[{label:`Claim`,conditions:[{type:`RequestsReady`,status:`True`,reason:`Healthy`,lastTransitionTime:`2026-10-15T11:00:00Z`},{type:`ClusterReady`,status:`False`,reason:`Converging`,message:`Zeebe has 1 of 3 brokers ready.`,lastTransitionTime:`2026-10-15T11:58:00Z`},{type:`AttachmentsReady`,status:`Unknown`,reason:`Waiting`,lastTransitionTime:`2026-10-15T11:58:00Z`}]}]}},C={name:`Empty`},w=[`NotReadyCondition`,`Groups`,`Empty`],x.parameters={...x.parameters,docs:{...x.parameters?.docs,source:{originalSource:`{
  name: "Not ready condition",
  args: {
    conditions: [{
      type: "AdminSecretReady",
      status: "False",
      reason: "SecretMissing",
      message: "The Secret orders-admin does not exist.",
      lastTransitionTime: "2026-10-15T11:48:00Z"
    }, {
      type: "MirroredSecretsReady",
      status: "True",
      reason: "Healthy",
      message: "",
      lastTransitionTime: "2026-10-14T09:00:00Z"
    }]
  }
}`,...x.parameters?.docs?.source}}},S.parameters={...S.parameters,docs:{...S.parameters?.docs,source:{originalSource:`{
  name: "Groups",
  args: {
    conditions: [{
      type: "MirroredSecretsReady",
      status: "True",
      reason: "Healthy",
      lastTransitionTime: "2026-10-14T09:00:00Z"
    }],
    groups: [{
      label: "Claim",
      conditions: [{
        type: "RequestsReady",
        status: "True",
        reason: "Healthy",
        lastTransitionTime: "2026-10-15T11:00:00Z"
      }, {
        type: "ClusterReady",
        status: "False",
        reason: "Converging",
        message: "Zeebe has 1 of 3 brokers ready.",
        lastTransitionTime: "2026-10-15T11:58:00Z"
      }, {
        type: "AttachmentsReady",
        status: "Unknown",
        reason: "Waiting",
        lastTransitionTime: "2026-10-15T11:58:00Z"
      }]
    }]
  }
}`,...S.parameters?.docs?.source}}},C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
  name: "Empty"
}`,...C.parameters?.docs?.source}}}})))()}T();export{C as Empty,S as Groups,x as NotReadyCondition,w as __namedExportsOrder,b as default};