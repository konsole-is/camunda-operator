import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{d as t,t as n}from"./merge-CyPKz5z-.js";import{r,t as i}from"./fixtures-C7LiHu3T.js";import{n as a,t as o}from"./images-section-DdWvWown.js";var s,c,l,u,d,f,p;function m(){return(m=e((()=>{n(),i(),a(),{fn:s}=__STORYBOOK_MODULE_TEST__,c={title:`setup/platform/ImagesSection`,component:o,args:{images:{camunda:`registry.example.com/camunda/camunda`},defaults:t(r,[`8.9`]),errors:[],onChange:s()}},l={name:`One minor`},u={name:`Two minors`,args:{defaults:t(r,[`8.9`,`8.10`])}},d={name:`A minor without defaults`,args:{defaults:t(r,[`8.8`,`8.9`])}},f={name:`Minors that can be incomplete`,args:{unreadMinorKinds:[`CamundaCluster`]}},p=[`OneMinor`,`TwoMinors`,`MinorWithoutDefaults`,`MinorsIncomplete`],l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "One minor"
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Two minors",
  args: {
    defaults: defaultRepositories(twoMinorTable, ["8.9", "8.10"])
  }
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "A minor without defaults",
  args: {
    defaults: defaultRepositories(twoMinorTable, ["8.8", "8.9"])
  }
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Minors that can be incomplete",
  args: {
    unreadMinorKinds: ["CamundaCluster"]
  }
}`,...f.parameters?.docs?.source}}}})))()}m();export{d as MinorWithoutDefaults,f as MinorsIncomplete,l as OneMinor,u as TwoMinors,p as __namedExportsOrder,c as default};