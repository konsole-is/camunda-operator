import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./namespace-picker-_WBY8CdP.js";var r,i,a,o,s,c,l;function u(){return(u=e((()=>{t(),{fn:r}=__STORYBOOK_MODULE_TEST__,i={title:`fleet/NamespacePicker`,component:n,args:{namespaces:[{name:`team-payments`,environment:`prod`},{name:`team-orders`,environment:`prod`}],environment:{name:`prod`,displayName:`Production`,namespaceCreation:{allowed:!0,labels:{"cost-center":`4711`},requiredLabelKeys:[`team`]}},onChange:r()}},a={name:`Existing namespace`,args:{defaultValue:{kind:`existing`,name:`team-orders`}}},o={name:`New namespace`,args:{defaultValue:{kind:`new`,name:`team-search`,labels:{team:`search`}}}},s={name:`Creation not allowed`,args:{environment:{name:`prod`,displayName:`Production`}}},c={name:`No namespace to pick`,args:{namespaces:[],environment:{name:`prod`,displayName:`Production`}}},l=[`ExistingNamespace`,`NewNamespace`,`CreationNotAllowed`,`NoNamespaceToPick`],a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
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
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "No namespace to pick",
  args: {
    namespaces: [],
    environment: {
      name: "prod",
      displayName: "Production"
    }
  }
}`,...c.parameters?.docs?.source}}}})))()}u();export{s as CreationNotAllowed,a as ExistingNamespace,o as NewNamespace,c as NoNamespaceToPick,l as __namedExportsOrder,i as default};