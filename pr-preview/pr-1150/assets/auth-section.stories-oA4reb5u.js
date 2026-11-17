import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{i as n,n as r,r as i}from"./namespaced-secret-ref-picker-D7C23PyK.js";import{a,n as o,o as s,t as c}from"./auth-section-DtXqYK6R.js";import{n as l,t as u}from"./fixtures-C7LiHu3T.js";var d,f,p,m,h,g,_,v;function y(){return(y=e((()=>{d=t(),i(),o(),s(),u(),{fn:f}=__STORYBOOK_MODULE_TEST__,p=n({"camunda-system":[{name:`oidc-credentials`,keys:[`client-secret`,`identity-secret`]},{name:`camunda-license`,keys:[`license-key`]}]}),m=e=>a(l().platformConfigs.find(t=>t.metadata.name===e)),h={title:`setup/platform/AuthSection`,component:c,args:{draft:m(`oidc-prod`),namespaces:[`camunda-system`,`team-a`],errors:[],onChange:f()},decorators:[e=>(0,d.jsx)(r.Provider,{value:p,children:(0,d.jsx)(e,{})})]},g={name:`OIDC`},_={name:`Basic`,args:{draft:m(`basic-lab`)}},v=[`Oidc`,`Basic`],g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "OIDC"
}`,...g.parameters?.docs?.source}}},_.parameters={..._.parameters,docs:{..._.parameters?.docs,source:{originalSource:`{
  name: "Basic",
  args: {
    draft: draftNamed("basic-lab")
  }
}`,..._.parameters?.docs?.source}}}})))()}y();export{_ as Basic,g as Oidc,v as __namedExportsOrder,h as default};