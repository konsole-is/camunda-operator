import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,r as n}from"./fixtures-LAdMoSZh.js";import{n as r,t as i}from"./release-environments-BHny5T19.js";function a(){let{environments:e}=n();return[...e,{...e[1],metadata:{name:`sandbox`},spec:{displayName:`Sandbox`}}]}var o,s,c,l;function u(){return(u=e((()=>{t(),r(),o={title:`setup/releases/ReleaseEnvironments`,component:i,args:{release:`camunda-8-9-0`,environments:a()}},s={name:`Default, allowed, not allowed`},c={name:`No environment`,args:{environments:[]}},l=[`DefaultAllowedNotAllowed`,`NoEnvironment`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Default, allowed, not allowed"
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "No environment",
  args: {
    environments: []
  }
}`,...c.parameters?.docs?.source}}}})))()}u();export{s as DefaultAllowedNotAllowed,c as NoEnvironment,l as __namedExportsOrder,o as default};