import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./env-editor-Bi3PleIL.js";var r,i,a,o,s;function c(){return(c=e((()=>{t(),{fn:r}=__STORYBOOK_MODULE_TEST__,i={title:`fleet/EnvEditor`,component:n,args:{label:`Environment variables`,secrets:[{name:`orders-db`,keys:[`password`,`username`]},{name:`smtp`,keys:[`token`]}],onChange:r()}},a={name:`Values and Secrets`,args:{defaultValue:[{name:`JAVA_TOOL_OPTIONS`,value:`-Xmx2g`},{name:`DB_PASSWORD`,valueFrom:{secretKeyRef:{name:`orders-db`,key:`password`}}},{name:`MODE`,valueFrom:{configMapKeyRef:{name:`settings`,key:`mode`}}}]}},o={name:`Empty`},s=[`ValuesAndSecrets`,`Empty`],a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Values and Secrets",
  args: {
    defaultValue: [{
      name: "JAVA_TOOL_OPTIONS",
      value: "-Xmx2g"
    }, {
      name: "DB_PASSWORD",
      valueFrom: {
        secretKeyRef: {
          name: "orders-db",
          key: "password"
        }
      }
    }, {
      name: "MODE",
      valueFrom: {
        configMapKeyRef: {
          name: "settings",
          key: "mode"
        }
      }
    }]
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Empty"
}`,...o.parameters?.docs?.source}}}})))()}c();export{o as Empty,a as ValuesAndSecrets,s as __namedExportsOrder,i as default};