import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./input-C5dG51LH.js";var i,a,o,s,c,l;function u(){return(u=e((()=>{i=t(),n(),{fn:a}=__STORYBOOK_MODULE_TEST__,o={title:`ui/Input`,component:r,args:{onChange:a()},decorators:[e=>(0,i.jsx)(`div`,{className:`flex max-w-sm flex-col gap-2`,children:(0,i.jsx)(e,{})})]},s={name:`Enabled`,args:{id:`story-name`,defaultValue:`orders`},render:e=>(0,i.jsxs)(i.Fragment,{children:[(0,i.jsx)(`label`,{htmlFor:e.id,className:`text-sm font-medium text-muted`,children:`Name`}),(0,i.jsx)(r,{...e})]})},c={name:`Disabled`,args:{id:`story-ns`,defaultValue:`team-orders`,disabled:!0},render:e=>(0,i.jsxs)(i.Fragment,{children:[(0,i.jsx)(`label`,{htmlFor:e.id,className:`text-sm font-medium text-muted`,children:`Namespace`}),(0,i.jsx)(r,{...e})]})},l=[`Enabled`,`Disabled`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Enabled",
  args: {
    id: "story-name",
    defaultValue: "orders"
  },
  render: args => <>
      <label htmlFor={args.id} className="text-sm font-medium text-muted">
        Name
      </label>
      <Input {...args} />
    </>
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Disabled",
  args: {
    id: "story-ns",
    defaultValue: "team-orders",
    disabled: true
  },
  render: args => <>
      <label htmlFor={args.id} className="text-sm font-medium text-muted">
        Namespace
      </label>
      <Input {...args} />
    </>
}`,...c.parameters?.docs?.source}}}})))()}u();export{c as Disabled,s as Enabled,l as __namedExportsOrder,o as default};