import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./toast-B0lmg6jC.js";var r,i,a,o,s,c;function l(){return(l=e((()=>{t(),r={title:`fleet/Toast`,component:n},i={name:`Saved`,args:{result:{kind:`saved`,message:`Saved the claim orders`}}},a={name:`Waiting for GitOps`,args:{result:{kind:`waiting`,commit:`4f1c3a7e9b2d8c6f0a5e1b3d`}}},o={name:`Applied`,args:{result:{kind:`applied`,message:`GitOps applied the claim orders`,commit:`4f1c3a7e`}}},s={name:`Error`,args:{result:{kind:`error`,message:`The claim changed since you opened it. Reload it to see the change.`}}},c=[`Saved`,`WaitingForGitOps`,`Applied`,`Error`],i.parameters={...i.parameters,docs:{...i.parameters?.docs,source:{originalSource:`{
  name: "Saved",
  args: {
    result: {
      kind: "saved",
      message: "Saved the claim orders"
    }
  }
}`,...i.parameters?.docs?.source}}},a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Waiting for GitOps",
  args: {
    result: {
      kind: "waiting",
      commit: "4f1c3a7e9b2d8c6f0a5e1b3d"
    }
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Applied",
  args: {
    result: {
      kind: "applied",
      message: "GitOps applied the claim orders",
      commit: "4f1c3a7e"
    }
  }
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Error",
  args: {
    result: {
      kind: "error",
      message: "The claim changed since you opened it. Reload it to see the change."
    }
  }
}`,...s.parameters?.docs?.source}}}})))()}l();export{o as Applied,s as Error,i as Saved,a as WaitingForGitOps,c as __namedExportsOrder,r as default};