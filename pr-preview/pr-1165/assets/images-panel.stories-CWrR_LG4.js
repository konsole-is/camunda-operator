import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./fixtures-LAdMoSZh.js";import{i as r,n as i,r as a,t as o}from"./images-panel-DOj8lmZ7.js";var s,c,l,u,d,f;function p(){return(p=e((()=>{t(),a(),i(),{fn:s}=__STORYBOOK_MODULE_TEST__,c={title:`setup/releases/ImagesPanel`,component:o,args:{rows:r({version:`8.9.0`,connectors:{version:`8.9.7`}},n),minor:`8.9`,onPin:s()}},l={name:`Defaults for 8.9`},u={name:`Pinned images`,args:{pinned:{camunda:`registry.example.com/camunda/camunda:8.9.0-patched`,connectors:`registry.example.com/camunda/connectors-bundle:8.9.7`}}},d={name:`Minor not in the table`,args:{rows:void 0,minor:`8.10`}},f=[`DefaultsFor89`,`PinnedImages`,`MinorNotInTheTable`],l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Defaults for 8.9"
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Pinned images",
  args: {
    pinned: {
      camunda: "registry.example.com/camunda/camunda:8.9.0-patched",
      connectors: "registry.example.com/camunda/connectors-bundle:8.9.7"
    }
  }
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Minor not in the table",
  args: {
    rows: undefined,
    minor: "8.10"
  }
}`,...d.parameters?.docs?.source}}}})))()}p();export{l as DefaultsFor89,d as MinorNotInTheTable,u as PinnedImages,f as __namedExportsOrder,c as default};