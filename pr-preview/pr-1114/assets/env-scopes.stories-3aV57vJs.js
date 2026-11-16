import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./env-scopes-CjF20gJv.js";import{n as r,r as i}from"./fixtures-CXhqxw46.js";var a,o,s,c,l,u,d;function f(){return(f=e((()=>{t(),r(),{fn:a}=__STORYBOOK_MODULE_TEST__,o=()=>[],s={title:`setup/releases/EnvScopes`,component:n,args:{spec:{version:`8.9.0`},secrets:[{name:`orders-db`,keys:[`password`]}],missingIn:o,onChange:a()}},c={name:`No entries`},l={name:`Entries in two scopes`,args:{spec:i().releases[0].spec}},u={name:`Secret missing in a namespace`,args:{spec:i().releases[0].spec,missingIn:e=>e===`orders-db`?[`team-billing`]:[]}},d=[`NoEntries`,`EntriesInTwoScopes`,`SecretMissingInANamespace`],c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "No entries"
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Entries in two scopes",
  args: {
    spec: releaseFixtures().releases[0].spec
  }
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Secret missing in a namespace",
  args: {
    spec: releaseFixtures().releases[0].spec,
    missingIn: (name: string) => name === "orders-db" ? ["team-billing"] : []
  }
}`,...u.parameters?.docs?.source}}}})))()}f();export{l as EntriesInTwoScopes,c as NoEntries,u as SecretMissingInANamespace,d as __namedExportsOrder,s as default};