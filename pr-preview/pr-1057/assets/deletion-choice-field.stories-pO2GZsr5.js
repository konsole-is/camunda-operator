import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{i as r,n as i}from"./deletion-choice-BKTd8n93.js";function a(e){let[t,n]=(0,s.useState)(e.choice.policy);return(0,o.jsx)(i,{...e,choice:{...e.choice,policy:t},onChange:t=>{e.onChange(t),n(t)}})}var o,s,c,l,u,d;function f(){return(f=e((()=>{o=n(),s=t(),r(),{fn:c}=__STORYBOOK_MODULE_TEST__,l={title:`fleet/DeletionChoiceField`,component:i,args:{subject:`cluster`,onChange:c()},decorators:[e=>(0,o.jsx)(`div`,{className:`max-w-3xl`,children:(0,o.jsx)(e,{})})],render:e=>(0,o.jsx)(a,{...e},JSON.stringify(e.choice))},u={name:`Deletion choice with Keep refused`,args:{choice:{defaultPolicy:`Orphan`,keepAllowed:!1,alwaysDeleted:[{section:`backup-storage`,label:`Archive bucket (Backup storage)`},{section:`access`,label:`Gateway (Access)`}],policy:`Delete`}}},d=[`DeletionChoiceWithKeepRefused`],u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Deletion choice with Keep refused",
  args: {
    choice: {
      defaultPolicy: "Orphan",
      keepAllowed: false,
      alwaysDeleted: [{
        section: "backup-storage",
        label: "Archive bucket (Backup storage)"
      }, {
        section: "access",
        label: "Gateway (Access)"
      }],
      policy: "Delete"
    }
  }
}`,...u.parameters?.docs?.source}}}})))()}f();export{u as DeletionChoiceWithKeepRefused,d as __namedExportsOrder,l as default};