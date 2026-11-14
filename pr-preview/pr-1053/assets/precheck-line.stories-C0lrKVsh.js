import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{r as n,t as r}from"./session-Dcps0m7c.js";import{n as i,t as a}from"./status-chip-DAXEX2Rs.js";function o({claim:e}){let t=n(e);return t?(0,s.jsxs)(`p`,{role:`status`,className:`flex flex-wrap items-center gap-2 text-sm`,children:[(0,s.jsx)(a,{condition:t}),(0,s.jsx)(`span`,{className:`text-text-2`,children:t.message})]}):null}var s;function c(){return(c=e((()=>{s=t(),r(),i(),o.__docgenInfo={description:`PrecheckLine shows why the claim controller refused the values of a
claim: the precheck reason of its Ready condition and the message. It
shows nothing when precheckOf finds no such condition. A flow without a
Claim tab, such as the create progress, shows it after a write.`,methods:[],displayName:`PrecheckLine`,props:{claim:{required:!0,tsType:{name:`Parameters[0]`,raw:`Parameters<typeof precheckOf>[0]`},description:``}}}})))()}var l,u,d;function f(){return(f=e((()=>{c(),l={title:`fleet/PrecheckLine`,component:o},u={name:`SlotConflict`,args:{claim:{status:{conditions:[{type:`Ready`,status:`False`,reason:`SlotConflict`,message:`spec.cluster.externalUrl is filled by section "access", so the claim cannot set it`}]}}}},d=[`SlotConflict`],u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "SlotConflict",
  args: {
    claim: {
      status: {
        conditions: [{
          type: "Ready",
          status: "False",
          reason: "SlotConflict",
          message: 'spec.cluster.externalUrl is filled by section "access", so the claim cannot set it'
        }]
      }
    }
  }
}`,...u.parameters?.docs?.source}}}})))()}f();export{u as SlotConflict,d as __namedExportsOrder,l as default};