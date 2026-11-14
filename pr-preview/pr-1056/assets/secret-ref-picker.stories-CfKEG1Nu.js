import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./secret-ref-picker-euOpgowN.js";var r,i,a,o,s;function c(){return(c=e((()=>{t(),{fn:r}=__STORYBOOK_MODULE_TEST__,i={title:`fleet/SecretRefPicker`,component:n,args:{label:`Client secret`,secrets:[],onChange:r()}},a={name:`Picked`,args:{secrets:[{name:`orders-db`,keys:[`password`,`username`]},{name:`smtp`,keys:[`token`]}],defaultValue:{name:`smtp`,key:`token`}}},o={name:`No Secrets`},s=[`Picked`,`NoSecrets`],a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Picked",
  args: {
    secrets: [{
      name: "orders-db",
      keys: ["password", "username"]
    }, {
      name: "smtp",
      keys: ["token"]
    }],
    defaultValue: {
      name: "smtp",
      key: "token"
    }
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "No Secrets"
}`,...o.parameters?.docs?.source}}}})))()}c();export{o as NoSecrets,a as Picked,s as __namedExportsOrder,i as default};