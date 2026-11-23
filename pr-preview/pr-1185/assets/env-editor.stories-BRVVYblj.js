import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{r as t,t as n}from"./env-editor-C_Vpliq7.js";var r,i,a,o,s,c;function l(){return(l=e((()=>{t(),{fn:r}=__STORYBOOK_MODULE_TEST__,i={title:`fleet/EnvEditor`,component:n,args:{label:`Environment variables`,secrets:[{name:`orders-db`,keys:[`password`,`username`]},{name:`smtp`,keys:[`token`]}],onChange:r()}},a={name:`Values and Secrets`,args:{defaultValue:[{name:`JAVA_TOOL_OPTIONS`,value:`-Xmx2g`},{name:`DB_PASSWORD`,valueFrom:{secretKeyRef:{name:`orders-db`,key:`password`}}},{name:`MODE`,valueFrom:{configMapKeyRef:{name:`settings`,key:`mode`}}}]}},o={name:`Empty`},s={name:`Secret missing in a namespace`,args:{defaultValue:[{name:`DB_PASSWORD`,valueFrom:{secretKeyRef:{name:`orders-db`,key:`password`}}},{name:`SMTP_TOKEN`,valueFrom:{secretKeyRef:{name:`smtp`,key:`token`}}}],missingIn:e=>e===`orders-db`?[`team-billing`]:[]}},c=[`ValuesAndSecrets`,`Empty`,`SecretMissingInANamespace`],a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
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
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Secret missing in a namespace",
  args: {
    defaultValue: [{
      name: "DB_PASSWORD",
      valueFrom: {
        secretKeyRef: {
          name: "orders-db",
          key: "password"
        }
      }
    }, {
      name: "SMTP_TOKEN",
      valueFrom: {
        secretKeyRef: {
          name: "smtp",
          key: "token"
        }
      }
    }],
    missingIn: (name: string) => name === "orders-db" ? ["team-billing"] : []
  }
}`,...s.parameters?.docs?.source}}}})))()}l();export{o as Empty,s as SecretMissingInANamespace,a as ValuesAndSecrets,c as __namedExportsOrder,i as default};