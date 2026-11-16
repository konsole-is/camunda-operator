import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{i as n,n as r,r as i,t as a}from"./namespaced-secret-ref-picker-lo8RhUPv.js";var o,s,c,l,u,d,f,p;function m(){return(m=e((()=>{o=t(),i(),{fn:s}=__STORYBOOK_MODULE_TEST__,c=n({"camunda-system":[{name:`oidc-credentials`,keys:[`client-secret`,`client-id`]},{name:`smtp`,keys:[`token`]}],"team-a":[{name:`orders-oidc`,keys:[`secret`]}]}),l={title:`fleet/NamespacedSecretRefPicker`,component:a,args:{label:`Client secret`,namespaces:[`camunda-system`,`team-a`],onChange:s()},decorators:[e=>(0,o.jsx)(r.Provider,{value:c,children:(0,o.jsx)(`div`,{className:`max-w-2xl`,children:(0,o.jsx)(e,{})})})],render:e=>(0,o.jsx)(a,{...e},JSON.stringify(e.value))},u={name:`Picked`,args:{value:{namespace:`camunda-system`,name:`oidc-credentials`,key:`client-secret`}}},d={name:`Missing Secret`,args:{label:`License key`,value:{namespace:`camunda-system`,name:`camunda-license`,key:`license-key`}}},f={name:`Nothing picked`},p=[`Picked`,`MissingSecret`,`NothingPicked`],u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Picked",
  args: {
    value: {
      namespace: "camunda-system",
      name: "oidc-credentials",
      key: "client-secret"
    }
  }
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Missing Secret",
  args: {
    label: "License key",
    value: {
      namespace: "camunda-system",
      name: "camunda-license",
      key: "license-key"
    }
  }
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Nothing picked"
}`,...f.parameters?.docs?.source}}}})))()}m();export{d as MissingSecret,f as NothingPicked,u as Picked,p as __namedExportsOrder,l as default};