import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./users-table-Df3drAJW.js";var r,i,a,o,s,c;function l(){return(l=e((()=>{t(),r={title:`setup/connections/UsersTable`,component:n,args:{users:[{kind:`CamundaCluster`,namespace:`team-a`,name:`billing`,fields:[`spec.storageRef`],ready:{status:`False`,reason:`StorageAlreadyAttached`,message:`Another cluster holds this storage backend: team-a/orders`},environment:`production`},{kind:`CamundaCluster`,namespace:`team-a`,name:`orders`,fields:[`spec.backupStorageRef`,`spec.storageRef`],ready:{status:`True`,reason:`Healthy`,message:`All checks passed`},environment:`production`}]}},i={name:`Users with a refused cluster`},a={name:`A reference from a preset`,args:{users:[{kind:`ElasticsearchCluster`,namespace:`team-a`,name:`orders-es`,fields:[`spec.snapshotStorageRef`],preset:`es-small`,ready:{status:`True`,reason:`Healthy`,message:`All checks passed`}},{kind:`DatabaseServer`,namespace:`team-a`,name:`orders-pg`,fields:[`spec.archive.objectStorageRef`,`status.archive.history.objectStorageRef`]}]}},o={name:`No user`,args:{users:[]}},s={name:`A kind that cannot be read`,args:{users:[],unread:[`CamundaCluster`,`DatabaseServerPreset`]}},c=[`UsersWithARefusedCluster`,`AReferenceFromAPreset`,`NoUser`,`AKindThatCannotBeRead`],i.parameters={...i.parameters,docs:{...i.parameters?.docs,source:{originalSource:`{
  name: "Users with a refused cluster"
}`,...i.parameters?.docs?.source}}},a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "A reference from a preset",
  args: {
    users: [{
      kind: "ElasticsearchCluster",
      namespace: "team-a",
      name: "orders-es",
      fields: ["spec.snapshotStorageRef"],
      preset: "es-small",
      ready: {
        status: "True",
        reason: "Healthy",
        message: "All checks passed"
      }
    }, {
      kind: "DatabaseServer",
      namespace: "team-a",
      name: "orders-pg",
      fields: ["spec.archive.objectStorageRef", "status.archive.history.objectStorageRef"]
    }]
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "No user",
  args: {
    users: []
  }
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "A kind that cannot be read",
  args: {
    users: [],
    unread: ["CamundaCluster", "DatabaseServerPreset"]
  }
}`,...s.parameters?.docs?.source}}}})))()}l();export{s as AKindThatCannotBeRead,a as AReferenceFromAPreset,o as NoUser,i as UsersWithARefusedCluster,c as __namedExportsOrder,r as default};