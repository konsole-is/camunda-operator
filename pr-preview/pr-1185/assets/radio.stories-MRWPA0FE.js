import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./radio-DhXW10xq.js";var i,a,o,s,c,l;function u(){return(u=e((()=>{i=t(),n(),{fn:a}=__STORYBOOK_MODULE_TEST__,o={title:`ui/Radio`,component:r,args:{onChange:a(),name:`deletion`}},s={name:`Group`,render:e=>(0,i.jsxs)(`fieldset`,{className:`flex flex-col gap-2`,children:[(0,i.jsx)(`legend`,{className:`mb-1 text-sm font-medium`,children:`When the cluster is deleted`}),(0,i.jsxs)(`label`,{className:`flex items-center gap-2`,children:[(0,i.jsx)(r,{...e,value:`keep`,defaultChecked:!0}),` Keep the resources`]}),(0,i.jsxs)(`label`,{className:`flex items-center gap-2`,children:[(0,i.jsx)(r,{...e,value:`delete`}),` Delete the resources`]})]})},c={name:`Disabled`,render:e=>(0,i.jsxs)(`label`,{className:`flex items-center gap-2`,children:[(0,i.jsx)(r,{...e,disabled:!0}),` Keep the resources`]})},l=[`Group`,`Disabled`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Group",
  render: args => <fieldset className="flex flex-col gap-2">
      <legend className="mb-1 text-sm font-medium">
        When the cluster is deleted
      </legend>
      <label className="flex items-center gap-2">
        <Radio {...args} value="keep" defaultChecked /> Keep the resources
      </label>
      <label className="flex items-center gap-2">
        <Radio {...args} value="delete" /> Delete the resources
      </label>
    </fieldset>
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Disabled",
  render: args => <label className="flex items-center gap-2">
      <Radio {...args} disabled /> Keep the resources
    </label>
}`,...c.parameters?.docs?.source}}}})))()}u();export{c as Disabled,s as Group,l as __namedExportsOrder,o as default};