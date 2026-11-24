import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{i as t,o as n,r}from"./fixtures-kgATjJaf.js";import{i,n as a,r as o,t as s}from"./parameters-tab-C6rDx_Cg.js";var c,l,u,d,f,p,m;function h(){return(h=e((()=>{t(),o(),a(),c=n(`aws-kms`,{parameters:{type:`object`,properties:{deletionWindowDays:{type:`integer`,minimum:7,maximum:30,description:`The days between the delete of a key and its removal.`},multiRegion:{type:`boolean`,description:`Makes a key that other regions can use.`},keySpec:{type:`string`,enum:[`SYMMETRIC_DEFAULT`,`RSA_2048`]}}},parameterValues:{deletionWindowDays:30,multiRegion:!1},claimEditable:[`multiRegion`]}),l=[r(`production`,{displayName:`Production`,parameters:{"aws-kms":{deletionWindowDays:{value:14},keySpec:{claimEditable:!0}}}}),r(`staging`,{displayName:`Staging`})],u={title:`setup/plugins/ParametersTab`,component:s,args:{rows:i(c,l,[]),unread:[]}},d={name:`Three parameters and two environments`},f={name:`No parameters`,args:{rows:i(n(`rustfs`,{}),[],[])}},p={name:`Parameter values that are not an object`,args:{rows:i(n(`aws-kms`,{parameterValues:[30]}),[],[])}},m=[`ThreeParameters`,`NoParameters`,`BadValues`],d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Three parameters and two environments"
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "No parameters",
  args: {
    rows: parameterRows(pluginRegistration("rustfs", {}), [], [])
  }
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Parameter values that are not an object",
  args: {
    rows: parameterRows(pluginRegistration("aws-kms", {
      parameterValues: [30]
    }), [], [])
  }
}`,...p.parameters?.docs?.source}}}})))()}h();export{p as BadValues,f as NoParameters,d as ThreeParameters,m as __namedExportsOrder,u as default};