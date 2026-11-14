import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{r as n,t as r}from"./button-q0ZjCprT.js";var i,a,o,s,c,l,u;function d(){return(d=e((()=>{i=t(),n(),{fn:a}=__STORYBOOK_MODULE_TEST__,o={title:`ui/Button`,component:r,args:{children:`Create cluster`,onClick:a()}},s={name:`Variants`,render:e=>(0,i.jsxs)(`div`,{className:`flex flex-wrap gap-3`,children:[(0,i.jsx)(r,{...e,children:`Create cluster`}),(0,i.jsx)(r,{...e,variant:`secondary`,children:`Cancel`}),(0,i.jsx)(r,{...e,variant:`outline`,children:`Edit`}),(0,i.jsx)(r,{...e,variant:`ghost`,children:`More`}),(0,i.jsx)(r,{...e,variant:`destructive`,children:`Delete`}),(0,i.jsx)(r,{...e,variant:`link`,children:`View`})]})},c={name:`Disabled`,args:{children:`Saving`,disabled:!0}},l={name:`Sizes`,render:e=>(0,i.jsxs)(`div`,{className:`flex flex-wrap items-center gap-3`,children:[(0,i.jsx)(r,{...e,size:`xs`,children:`Extra small`}),(0,i.jsx)(r,{...e,size:`sm`,children:`Small`}),(0,i.jsx)(r,{...e,children:`Default`}),(0,i.jsx)(r,{...e,size:`lg`,children:`Large`})]})},u=[`Variants`,`Disabled`,`Sizes`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Variants",
  render: args => <div className="flex flex-wrap gap-3">
      <Button {...args}>Create cluster</Button>
      <Button {...args} variant="secondary">
        Cancel
      </Button>
      <Button {...args} variant="outline">
        Edit
      </Button>
      <Button {...args} variant="ghost">
        More
      </Button>
      <Button {...args} variant="destructive">
        Delete
      </Button>
      <Button {...args} variant="link">
        View
      </Button>
    </div>
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Disabled",
  args: {
    children: "Saving",
    disabled: true
  }
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Sizes",
  render: args => <div className="flex flex-wrap items-center gap-3">
      <Button {...args} size="xs">
        Extra small
      </Button>
      <Button {...args} size="sm">
        Small
      </Button>
      <Button {...args}>Default</Button>
      <Button {...args} size="lg">
        Large
      </Button>
    </div>
}`,...l.parameters?.docs?.source}}}})))()}d();export{c as Disabled,l as Sizes,s as Variants,u as __namedExportsOrder,o as default};