import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,n as r,r as i,t as a}from"./field-help-DAQjHzC5.js";var o,s,c,l,u,d;function f(){return(f=e((()=>{o=t(),n(),r(),s={title:`fleet/FieldHelp`,component:a,args:{label:`Partitions`,doc:i(`core.camunda.io`,`CamundaCluster`,`spec.zeebe.partitions`),operatorVersion:`0.2.3`,defaultOpen:!1},render:e=>(0,o.jsxs)(`div`,{className:`flex items-center gap-1 text-sm font-medium text-muted`,children:[(0,o.jsx)(`span`,{children:`Partitions`}),(0,o.jsx)(a,{...e})]})},c={name:`Closed`},l={name:`Open`,args:{defaultOpen:!0},decorators:[e=>(0,o.jsx)(`div`,{className:`pb-48`,children:(0,o.jsx)(e,{})})]},u={name:`Open at the right edge`,args:{defaultOpen:!0},decorators:[e=>(0,o.jsx)(`div`,{className:`flex justify-end pb-48`,children:(0,o.jsx)(e,{})})]},d=[`Closed`,`Open`,`OpenAtTheRightEdge`],c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Closed"
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Open",
  args: {
    defaultOpen: true
  },
  decorators: [Story => <div className="pb-48">
        <Story />
      </div>]
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Open at the right edge",
  args: {
    defaultOpen: true
  },
  decorators: [Story => <div className="flex justify-end pb-48">
        <Story />
      </div>]
}`,...u.parameters?.docs?.source}}}})))()}f();export{c as Closed,l as Open,u as OpenAtTheRightEdge,d as __namedExportsOrder,s as default};