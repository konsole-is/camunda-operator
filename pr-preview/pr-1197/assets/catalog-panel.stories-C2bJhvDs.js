import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{i as t,l as n}from"./fixtures-kgATjJaf.js";import{i as r,n as i,r as a,t as o}from"./catalog-panel-BiK_fucx.js";var s,c,l,u,d,f;function p(){return(p=e((()=>{a(),i(),t(),s=n().sections,c={title:`setup/plugins/CatalogPanel`,component:o,args:{charts:r.charts,sections:s}},l={name:`Three charts`},u={name:`The catalog cannot be read`,args:{error:`reading catalog /app/catalog/official-plugins.json: ENOENT: no such file or directory`}},d={name:`Which charts are installed is not known`,args:{installedUnknown:!0}},f=[`ThreeCharts`,`CatalogNotRead`,`InstalledNotKnown`],l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Three charts"
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "The catalog cannot be read",
  args: {
    error: "reading catalog /app/catalog/official-plugins.json: ENOENT: no such file or directory"
  }
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Which charts are installed is not known",
  args: {
    installedUnknown: true
  }
}`,...d.parameters?.docs?.source}}}})))()}p();export{u as CatalogNotRead,d as InstalledNotKnown,l as ThreeCharts,f as __namedExportsOrder,c as default};