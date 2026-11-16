import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{a as t,n,r,t as i}from"./images-panel-jds8XXYB.js";import{n as a,t as o}from"./fixtures-CXhqxw46.js";var s,c,l,u,d,f;function p(){return(p=e((()=>{a(),r(),n(),{fn:s}=__STORYBOOK_MODULE_TEST__,c={title:`setup/releases/ImagesPanel`,component:i,args:{rows:t({version:`8.9.0`,connectors:{version:`8.9.7`}},o),minor:`8.9`,onPin:s()}},l={name:`Defaults for 8.9`},u={name:`Pinned images`,args:{pinned:{camunda:`registry.example.com/camunda/camunda:8.9.0-patched`,connectors:`registry.example.com/camunda/connectors-bundle:8.9.7`}}},d={name:`Minor not in the table`,args:{rows:void 0,minor:`8.10`}},f=[`DefaultsFor89`,`PinnedImages`,`MinorNotInTheTable`],l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
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