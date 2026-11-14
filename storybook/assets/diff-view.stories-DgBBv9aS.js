import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./diff-BLIgMNCc.js";import{o as r,r as i,s as a}from"./fixtures-DU7FxC4G.js";import{n as o,t as s}from"./diff-view-C5YmqhpV.js";var c,l,u,d,f;function p(){return(p=e((()=>{n(),o(),i(),c={title:`fleet/DiffView`,component:s,args:{label:`Claim orders`,lines:a}},l={name:`Changed field`},u={name:`New object`,args:{lines:t(void 0,r)}},d={name:`No changes`,args:{lines:t(r,r)}},f=[`ChangedField`,`NewObject`,`NoChanges`],l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
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