import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./log-viewer-DoFfpaer.js";var r,i,a,o,s;function c(){return(c=e((()=>{t(),r=[{raw:``,time:`2026-10-15T11:58:01Z`,level:`INFO`,message:`Partition 1 became leader`,logger:`io.camunda.zeebe.broker`,fields:{partitionId:1,term:4}},{raw:``,time:`2026-10-15T11:58:05Z`,level:`WARN`,message:`Exporter is behind by 1200 records`,fields:{}},{raw:``,time:`2026-10-15T11:58:09Z`,level:`ERROR`,message:`Failed to export a batch to Elasticsearch: connection refused`,fields:{exporter:`camundaexporter`}}].map(e=>({...e,raw:`${e.level} ${e.message}`})),i={title:`fleet/LogViewer`,component:n,args:{lines:[]}},a={name:`Following`,args:{lines:r}},o={name:`No lines`},s=[`Following`,`NoLines`],a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Following",
  args: {
    lines
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "No lines"
}`,...o.parameters?.docs?.source}}}})))()}c();export{a as Following,o as NoLines,s as __namedExportsOrder,i as default};