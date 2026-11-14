import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{r as t,t as n}from"./banners-DJhvcCY0.js";var r,i,a,o,s;function c(){return(c=e((()=>{t(),{fn:r}=__STORYBOOK_MODULE_TEST__,i={title:`cluster/ActivityBanner`,component:n,args:{backupsHref:`?tab=backups`,onOpenBackups:r()}},a={name:`Backup running`,args:{backup:{kind:`LogicalBackupRDBMS`,namespace:`team-a`,name:`nightly-1`}}},o={name:`Point-in-time restore running`,args:{restore:{kind:`PointInTimeRestore`,namespace:`team-a`,name:`before-incident`}}},s=[`BackupRunning`,`PointInTimeRestoreRunning`],a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Backup running",
  args: {
    backup: {
      kind: "LogicalBackupRDBMS",
      namespace: "team-a",
      name: "nightly-1"
    }
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Point-in-time restore running",
  args: {
    restore: {
      kind: "PointInTimeRestore",
      namespace: "team-a",
      name: "before-incident"
    }
  }
}`,...o.parameters?.docs?.source}}}})))()}c();export{a as BackupRunning,o as PointInTimeRestoreRunning,s as __namedExportsOrder,i as default};