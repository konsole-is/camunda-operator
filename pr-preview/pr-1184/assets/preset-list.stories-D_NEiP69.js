import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{i as t,n}from"./rows-Doe2eSyN.js";import{n as r,t as i}from"./fixtures-CF8JYqCq.js";import{n as a,r as o}from"./preset-list-DBIQND-S.js";var s,c,l,u,d,f,p,m;function h(){return(h=e((()=>{i(),o(),n(),{fn:s}=__STORYBOOK_MODULE_TEST__,c={title:`setup/presets/PresetList`,component:a,args:{rows:t(r(),[]),errors:[],mode:`direct`,actions:{previewChange:s(async()=>({objects:[],errors:[]})),applyChange:s(async()=>({ok:!0,mode:`direct`}))}}},l={name:`Three kinds, used and unused`},u={name:`No preset`,args:{rows:[]}},d=[{kind:`ElasticsearchCluster`,message:`forbidden`}],f={name:`A list that cannot be read`,args:{rows:t(r(),d),errors:d}},p={name:`Git mode`,args:{mode:`git`}},m=[`ThreeKindsUsedAndUnused`,`NoPreset`,`AListThatCannotBeRead`,`GitMode`],l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Three kinds, used and unused"
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "No preset",
  args: {
    rows: []
  }
}`,...u.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "A list that cannot be read",
  args: {
    rows: presetRows(presetFixtures(), unreadable),
    errors: unreadable
  }
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Git mode",
  args: {
    mode: "git"
  }
}`,...p.parameters?.docs?.source}}}})))()}h();export{f as AListThatCannotBeRead,p as GitMode,u as NoPreset,l as ThreeKindsUsedAndUnused,m as __namedExportsOrder,c as default};