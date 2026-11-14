import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./namespace-picker-Ca00lMNl.js";var r,i,a,o,s,c;function l(){return(l=e((()=>{t(),{fn:r}=__STORYBOOK_MODULE_TEST__,i={title:`fleet/NamespacePicker`,component:n,args:{namespaces:[{name:`team-payments`,environment:`prod`},{name:`team-orders`,environment:`prod`}],environment:{name:`prod`,displayName:`Production`,namespaceCreation:{allowed:!0,labels:{"cost-center":`4711`},requiredLabelKeys:[`team`]}},onChange:r()}},a={name:`Existing namespace`,args:{defaultValue:{kind:`existing`,name:`team-orders`}}},o={name:`New namespace`,args:{defaultValue:{kind:`new`,name:`team-search`,labels:{team:`search`}}}},s={name:`Creation not allowed`,args:{environment:{name:`prod`,displayName:`Production`}}},c=[`ExistingNamespace`,`NewNamespace`,`CreationNotAllowed`],a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Existing namespace",
  args: {
    defaultValue: {
      kind: "existing",
      name: "team-orders"
    }
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "New namespace",
  args: {
    defaultValue: {
      kind: "new",
      name: "team-search",
      labels: {
        team: "search"
      }
    }
  }
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Creation not allowed",
  args: {
    environment: {
      name: "prod",
      displayName: "Production"
    }
  }
}`,...s.parameters?.docs?.source}}}})))()}l();export{s as CreationNotAllowed,a as ExistingNamespace,o as NewNamespace,c as __namedExportsOrder,i as default};