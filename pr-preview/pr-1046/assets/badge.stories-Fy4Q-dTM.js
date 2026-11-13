import{n as e}from"./rolldown-runtime-2s6bX4vc.js";import{a as t}from"./iframe-C-WtZ9Oh.js";import{n,t as r}from"./dist-C5rfoaAs.js";import{n as i,t as a}from"./dist-DHGg81cT.js";import{i as o,t as s}from"./dist-OOzhz3sF.js";function c({className:e,variant:t=`default`,asChild:n=!1,...i}){let a=n?s:`span`;return(0,l.jsx)(a,{"data-slot":`badge`,"data-variant":t,className:r(u({variant:t}),e),...i})}var l,u;function d(){return(d=e((()=>{l=t(),i(),n(),o(),u=a(`group/badge inline-flex h-5 w-fit shrink-0 items-center justify-center gap-1 overflow-hidden rounded-4xl border border-transparent px-2 py-0.5 text-xs font-medium whitespace-nowrap transition-all focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&>svg]:pointer-events-none [&>svg]:size-3!`,{variants:{variant:{default:`bg-primary text-primary-foreground [a]:hover:bg-accent-hover`,secondary:`bg-secondary text-secondary-foreground [a]:hover:bg-secondary/80`,destructive:`bg-destructive/10 text-destructive focus-visible:ring-destructive/20 dark:focus-visible:ring-destructive/40 [a]:hover:bg-destructive/15`,outline:`border-border text-foreground [a]:hover:bg-panel-2 [a]:hover:text-muted-foreground`,ghost:`hover:bg-panel-2 hover:text-muted-foreground dark:hover:bg-panel-2/50`,link:`text-primary underline-offset-4 hover:underline`}},defaultVariants:{variant:`default`}}),c.__docgenInfo={description:``,methods:[],displayName:`Badge`,props:{asChild:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`false`,computed:!1}},variant:{defaultValue:{value:`"default"`,computed:!1},required:!1}}}})))()}var f,p,m,h;function g(){return(g=e((()=>{f=t(),d(),p={title:`ui/Badge`,component:c},m={name:`Variants`,render:e=>(0,f.jsxs)(`div`,{className:`flex flex-wrap gap-2`,children:[(0,f.jsx)(c,{...e,children:`Default`}),(0,f.jsx)(c,{...e,variant:`secondary`,children:`Optional`}),(0,f.jsx)(c,{...e,variant:`outline`,children:`Required`}),(0,f.jsx)(c,{...e,variant:`destructive`,children:`Refused`}),(0,f.jsx)(c,{...e,variant:`ghost`,children:`Draft`}),(0,f.jsx)(c,{...e,variant:`link`,children:`Details`})]})},h=[`Variants`],m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "Variants",
  render: args => <div className="flex flex-wrap gap-2">
      <Badge {...args}>Default</Badge>
      <Badge {...args} variant="secondary">
        Optional
      </Badge>
      <Badge {...args} variant="outline">
        Required
      </Badge>
      <Badge {...args} variant="destructive">
        Refused
      </Badge>
      <Badge {...args} variant="ghost">
        Draft
      </Badge>
      <Badge {...args} variant="link">
        Details
      </Badge>
    </div>
}`,...m.parameters?.docs?.source}}}})))()}g();export{m as Variants,h as __namedExportsOrder,p as default};