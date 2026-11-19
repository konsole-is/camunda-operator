import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,r as n,t as r}from"./fixtures-LAdMoSZh.js";import{n as i,t as a}from"./release-settings-C0pkMNeQ.js";var o,s,c,l,u,d;function f(){return(f=e((()=>{t(),i(),{fn:o}=__STORYBOOK_MODULE_TEST__,s={title:`setup/releases/ReleaseSettings`,component:a,args:{sources:{...n(),errors:[]},name:`camunda-8-9-0`,imagesTable:r,secrets:{"team-orders":[{name:`orders-db`,keys:[`password`]}],"team-billing":[{name:`billing-db`,keys:[`password`]}]},mode:`direct`,actions:{previewChange:o(async()=>({objects:[],errors:[]})),applyChange:o(async()=>({ok:!0,mode:`direct`})),previewMove:o(async()=>({objects:[],errors:[]})),applyMove:o(async()=>({ok:!0,mode:`direct`}))}}},c={name:`Existing release`},l={name:`Release that can be deleted`,args:{name:`camunda-8-9-1`,secrets:{}}},u={name:`New release from a copy`,args:{name:void 0,copyFrom:`camunda-8-9-0`,secrets:{}}},d=[`ExistingRelease`,`ReleaseThatCanBeDeleted`,`NewReleaseFromACopy`],c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Existing release"
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Release that can be deleted",
  args: {
    name: "camunda-8-9-1",
    secrets: {}
  }
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "New release from a copy",
  args: {
    name: undefined,
    copyFrom: "camunda-8-9-0",
    secrets: {}
  }
}`,...u.parameters?.docs?.source}}}})))()}f();export{c as ExistingRelease,u as NewReleaseFromACopy,l as ReleaseThatCanBeDeleted,d as __namedExportsOrder,s as default};