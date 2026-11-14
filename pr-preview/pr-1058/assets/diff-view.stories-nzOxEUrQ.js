import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./diff-BLIgMNCc.js";import{n as r,t as i}from"./diff-view-DXPLH9-N.js";import{o as a,r as o,s}from"./fixtures-DU7FxC4G.js";var c,l,u,d,f;function p(){return(p=e((()=>{n(),r(),o(),c={title:`fleet/DiffView`,component:i,args:{label:`Claim orders`,lines:s}},l={name:`Changed field`},u={name:`New object`,args:{lines:t(void 0,a)}},d={name:`No changes`,args:{lines:t(a,a)}},f=[`ChangedField`,`NewObject`,`NoChanges`],l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
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