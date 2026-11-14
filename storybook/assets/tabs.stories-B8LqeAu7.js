import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,i as r,n as i,r as a,t as o}from"./tabs-BJxJS6z2.js";var s,c,l,u,d,f;function p(){return(p=e((()=>{s=t(),n(),{fn:c}=__STORYBOOK_MODULE_TEST__,l={title:`ui/Tabs`,component:o,args:{defaultValue:`overview`,onValueChange:c()}},u={name:`Default`,render:e=>(0,s.jsxs)(o,{...e,children:[(0,s.jsxs)(a,{children:[(0,s.jsx)(r,{value:`overview`,children:`Overview`}),(0,s.jsx)(r,{value:`logs`,children:`Logs`})]}),(0,s.jsx)(i,{value:`overview`,children:`Overview of the cluster`}),(0,s.jsx)(i,{value:`logs`,children:`Logs of the cluster`})]})},d={name:`Line`,render:e=>(0,s.jsxs)(o,{...e,children:[(0,s.jsxs)(a,{variant:`line`,children:[(0,s.jsx)(r,{value:`overview`,children:`Overview`}),(0,s.jsx)(r,{value:`logs`,children:`Logs`})]}),(0,s.jsx)(i,{value:`overview`,children:`Overview of the cluster`}),(0,s.jsx)(i,{value:`logs`,children:`Logs of the cluster`})]})},f=[`Default`,`Line`],u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Default",
  render: args => <Tabs {...args}>
      <TabsList>
        <TabsTrigger value="overview">Overview</TabsTrigger>
        <TabsTrigger value="logs">Logs</TabsTrigger>
      </TabsList>
      <TabsContent value="overview">Overview of the cluster</TabsContent>
      <TabsContent value="logs">Logs of the cluster</TabsContent>
    </Tabs>
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Line",
  render: args => <Tabs {...args}>
      <TabsList variant="line">
        <TabsTrigger value="overview">Overview</TabsTrigger>
        <TabsTrigger value="logs">Logs</TabsTrigger>
      </TabsList>
      <TabsContent value="overview">Overview of the cluster</TabsContent>
      <TabsContent value="logs">Logs of the cluster</TabsContent>
    </Tabs>
}`,...d.parameters?.docs?.source}}}})))()}p();export{u as Default,d as Line,f as __namedExportsOrder,l as default};