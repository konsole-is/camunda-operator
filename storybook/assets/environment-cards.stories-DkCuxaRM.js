import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r,t as i}from"./environment-cards-CJiZWRD0.js";import{n as a,o,r as s,t as c}from"./fixtures-BTwPClnx.js";function l(e){let[t,n]=(0,d.useState)(()=>[e.value??[]].flat());if(e.multiple){let{onChange:r}=e;return(0,u.jsx)(i,{...e,value:t,onChange:e=>{r?.(e),n(e)}})}let{onChange:r}=e;return(0,u.jsx)(i,{...e,value:t[0],onChange:e=>{r?.(e),n([e])}})}var u,d,f,p,m,h,g,_;function v(){return(v=e((()=>{u=n(),d=t(),r(),a(),{fn:f}=__STORYBOOK_MODULE_TEST__,p={title:`create/EnvironmentCards`,component:i,args:{environments:[o,s],counts:{prod:{namespaces:2,clusters:3},dev:{namespaces:1,clusters:0},legacy:{namespaces:0,clusters:0}},onChange:f()},render:e=>(0,u.jsx)(l,{...e},JSON.stringify(e.value))},m={name:`Two environments, one picked`,args:{environments:[o,c],value:`prod`}},h={name:`Not ready`},g={name:`Several picked`,args:{multiple:!0,environments:[o,c,s],value:[`dev`,`prod`]}},_=[`TwoEnvironmentsOnePicked`,`NotReady`,`SeveralPicked`],m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "Two environments, one picked",
  args: {
    environments: [production, development],
    value: "prod"
  }
}`,...m.parameters?.docs?.source}}},h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "Not ready"
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "Several picked",
  args: {
    multiple: true,
    environments: [production, development, legacy],
    value: ["dev", "prod"]
  }
}`,...g.parameters?.docs?.source}}}})))()}v();export{h as NotReady,g as SeveralPicked,m as TwoEnvironmentsOnePicked,_ as __namedExportsOrder,p as default};