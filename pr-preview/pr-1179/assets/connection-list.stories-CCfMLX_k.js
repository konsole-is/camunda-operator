import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./fixtures-kch9eRRI.js";import{a as r,c as i,n as a,r as o,t as s}from"./connection-list-Dz2EKfqK.js";var c,l,u,d,f,p,m,h;function g(){return(g=e((()=>{o(),t(),i(),c=e=>Object.fromEntries(r.map(t=>[t.value,a(e,t.kind)])),l={title:`setup/connections/ConnectionList`,component:s,args:{tab:`object-storage`,rows:c(n()),errors:[],onTab:()=>{}}},u={name:`Object storage with every writer`},d={name:`Secondary storage with an owner`,args:{tab:`secondary-storage`}},f={name:`Management Identity written by a plane`,args:{tab:`management-identity`}},p={name:`An empty tab`,args:{tab:`databases`,rows:c({...n(),databaseConfigs:[]})}},m={name:`A read error`,args:{errors:[{kind:`PluginRequest`,message:`pluginrequests.plugins.camunda.io is forbidden: User "system:serviceaccount:camunda-system:fleet-manager" cannot list resource "pluginrequests"`}]}},h=[`ObjectStorageWithEveryWriter`,`SecondaryStorageWithAnOwner`,`ManagementIdentityWrittenByAPlane`,`AnEmptyTab`,`AReadError`],u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Object storage with every writer"
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Secondary storage with an owner",
  args: {
    tab: "secondary-storage"
  }
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Management Identity written by a plane",
  args: {
    tab: "management-identity"
  }
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "An empty tab",
  args: {
    tab: "databases",
    rows: rowsOf({
      ...connectionFixtures(),
      databaseConfigs: []
    })
  }
}`,...p.parameters?.docs?.source}}},m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "A read error",
  args: {
    errors: [{
      kind: "PluginRequest",
      message: 'pluginrequests.plugins.camunda.io is forbidden: User "system:serviceaccount:camunda-system:fleet-manager" cannot list resource "pluginrequests"'
    }]
  }
}`,...m.parameters?.docs?.source}}}})))()}g();export{m as AReadError,p as AnEmptyTab,f as ManagementIdentityWrittenByAPlane,u as ObjectStorageWithEveryWriter,d as SecondaryStorageWithAnOwner,h as __namedExportsOrder,l as default};