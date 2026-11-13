import{n as e}from"./rolldown-runtime-2s6bX4vc.js";import{a as t}from"./iframe-C-WtZ9Oh.js";import{n,t as r}from"./dist-C5rfoaAs.js";function i({className:e,type:t,...n}){return(0,a.jsx)(`input`,{type:t,"data-slot":`input`,className:r(`h-8 w-full min-w-0 rounded-lg border border-control-edge bg-transparent px-2.5 py-1 text-base transition-colors outline-none file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/30 dark:disabled:bg-input/80 dark:aria-invalid:ring-destructive/40`,e),...n})}var a;function o(){return(o=e((()=>{a=t(),n(),i.__docgenInfo={description:``,methods:[],displayName:`Input`}})))()}var s,c,l,u,d,f;function p(){return(p=e((()=>{s=t(),o(),{fn:c}=__STORYBOOK_MODULE_TEST__,l={title:`ui/Input`,component:i,args:{onChange:c()},decorators:[e=>(0,s.jsx)(`div`,{className:`flex max-w-sm flex-col gap-2`,children:(0,s.jsx)(e,{})})]},u={name:`Enabled`,args:{id:`story-name`,defaultValue:`orders`},render:e=>(0,s.jsxs)(s.Fragment,{children:[(0,s.jsx)(`label`,{htmlFor:e.id,className:`text-sm font-medium text-muted`,children:`Name`}),(0,s.jsx)(i,{...e})]})},d={name:`Disabled`,args:{id:`story-ns`,defaultValue:`team-orders`,disabled:!0},render:e=>(0,s.jsxs)(s.Fragment,{children:[(0,s.jsx)(`label`,{htmlFor:e.id,className:`text-sm font-medium text-muted`,children:`Namespace`}),(0,s.jsx)(i,{...e})]})},f=[`Enabled`,`Disabled`],u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
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
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
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
}`,...d.parameters?.docs?.source}}}})))()}p();export{d as Disabled,u as Enabled,f as __namedExportsOrder,l as default};