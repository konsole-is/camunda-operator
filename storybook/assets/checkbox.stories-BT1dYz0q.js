import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./checkbox-X75DKHBY.js";var i,a,o,s,c,l;function u(){return(u=e((()=>{i=t(),n(),{fn:a}=__STORYBOOK_MODULE_TEST__,o={title:`ui/Checkbox`,component:r,args:{onCheckedChange:a()}},s={name:`Checked`,args:{defaultChecked:!0},render:e=>(0,i.jsxs)(`label`,{className:`flex items-center gap-2`,children:[(0,i.jsx)(r,{...e}),` Keep the resources`]})},c={name:`Unchecked`,render:e=>(0,i.jsxs)(`label`,{className:`flex items-center gap-2`,children:[(0,i.jsx)(r,{...e}),` Delete the backups`]})},l=[`Checked`,`Unchecked`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Checked",
  args: {
    defaultChecked: true
  },
  render: args => <label className="flex items-center gap-2">
      <Checkbox {...args} /> Keep the resources
    </label>
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Unchecked",
  render: args => <label className="flex items-center gap-2">
      <Checkbox {...args} /> Delete the backups
    </label>
}`,...c.parameters?.docs?.source}}}})))()}u();export{s as Checked,c as Unchecked,l as __namedExportsOrder,o as default};