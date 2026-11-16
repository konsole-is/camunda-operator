import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r,r as i}from"./settings-w8S2mPUB.js";import{n as a,t as o}from"./option-settings-Do66ZUy7.js";import{n as s,r as c}from"./fixtures-CmgSa7d5.js";function l({sources:e,environment:t,registration:n,onSpec:r,onTextErrors:a}){let[s,c]=(0,d.useState)(t.spec);return(0,u.jsx)(o,{settings:i(e.registrations,s,e.sections).filter(e=>e.registration===n),sources:e,environment:{...t,spec:s},errors:[],onSpec:e=>{r(e),c(e)},onTextErrors:a})}var u,d,f,p,m,h,g,_,v,y;function b(){return(b=e((()=>{u=n(),d=t(),s(),a(),r(),{fn:f}=__STORYBOOK_MODULE_TEST__,p=c(),m={title:`setup/environments/OptionSettings`,component:l,args:{sources:p,environment:p.environments[0],onSpec:f(),onTextErrors:f()},render:e=>(0,u.jsx)(l,{...e},JSON.stringify(e.environment))},h={name:`Connectors role with allowed values`,args:{registration:`aws-connectors-role`}},g=c(),_=g.registrations.find(e=>e.metadata.name===`rustfs`),_.spec.parameters={type:`object`,required:[`bucket`],properties:{bucket:{type:`string`,minLength:1,title:`Bucket`},region:{type:`string`,title:`Region`}}},v={name:`Required parameter without a value`,args:{sources:g,environment:g.environments[0],registration:`rustfs`}},y=[`ConnectorsRoleWithAllowedValues`,`RequiredParameterWithoutValue`],h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "Connectors role with allowed values",
  args: {
    registration: "aws-connectors-role"
  }
}`,...h.parameters?.docs?.source}}},v.parameters={...v.parameters,docs:{...v.parameters?.docs,source:{originalSource:`{
  name: "Required parameter without a value",
  args: {
    sources: requiredBucket,
    environment: requiredBucket.environments[0],
    registration: "rustfs"
  }
}`,...v.parameters?.docs?.source}}}})))()}b();export{h as ConnectorsRoleWithAllowedValues,v as RequiredParameterWithoutValue,y as __namedExportsOrder,m as default};