import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{a as t,i as n,o as r,r as i}from"./review-panel-D95fXXb-.js";import{c as a,i as o,s}from"./fixtures-C-my-adP.js";var c,l,u,d,f;function p(){return(p=e((()=>{t(),n(),o(),c={title:`fleet/DiffView`,component:i,args:{label:`Claim orders`,lines:a}},l={name:`Changed field`},u={name:`New object`,args:{lines:r(void 0,s)}},d={name:`No changes`,args:{lines:r(s,s)}},f=[`ChangedField`,`NewObject`,`NoChanges`],l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Changed field"
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "New object",
  args: {
    lines: objectDiff(undefined, ordersClaim)
  }
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "No changes",
  args: {
    lines: objectDiff(ordersClaim, ordersClaim)
  }
}`,...d.parameters?.docs?.source}}}})))()}p();export{l as ChangedField,u as NewObject,d as NoChanges,f as __namedExportsOrder,c as default};