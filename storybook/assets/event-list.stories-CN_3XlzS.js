import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./dist-BTjrb5Vg.js";import{n as i,t as a}from"./age-f6c5OgJq.js";function o({events:e,now:t,showObject:n=!1}){if(e.length===0)return(0,s.jsx)(`p`,{className:`text-sm text-muted`,children:`No events`});let i=e=>e.series?.lastObservedTime??e.deprecatedLastTimestamp??e.eventTime??e.deprecatedFirstTimestamp??e.metadata?.creationTimestamp,o=[...e].sort((e,t)=>new Date(i(t)??0).getTime()-new Date(i(e)??0).getTime());return(0,s.jsx)(`ul`,{className:`flex flex-col divide-y divide-divider`,children:o.map((e,o)=>{let c=e.type===`Warning`,l=e.series?.count??e.deprecatedCount;return(0,s.jsxs)(`li`,{className:`flex flex-col gap-0.5 py-2 text-sm sm:flex-row sm:gap-4`,children:[(0,s.jsx)(`span`,{className:`w-12 shrink-0 text-muted`,children:a(i(e),t)}),(0,s.jsxs)(`span`,{className:r(`w-40 shrink-0 font-medium`,c?`text-warn`:`text-text`),children:[e.reason??`-`,c&&(0,s.jsx)(`span`,{className:`ml-2 rounded-sm bg-warn/15 px-1.5 py-0.5 text-xs font-normal text-warn`,children:`Warning`})]}),(0,s.jsxs)(`span`,{className:`min-w-0 flex-1 text-text-2`,children:[n&&e.regarding&&(0,s.jsx)(`span`,{className:`mr-2 font-mono text-xs break-all text-muted`,children:`${e.regarding.kind}/${e.regarding.name}`}),e.note,l&&l>1?(0,s.jsx)(`span`,{className:`text-muted`,children:` (${l} times)`}):null]})]},e.metadata?.uid??o)})})}var s;function c(){return(c=e((()=>{s=t(),n(),i(),o.__docgenInfo={description:`EventList lists the given events.k8s.io/v1 Events, newest first, with
their age. With showObject, an event names its regarding object. An
empty list says "No events".`,methods:[],displayName:`EventList`,props:{events:{required:!0,tsType:{name:`Array`,elements:[{name:`Resource`,elements:[{name:`literal`,value:`"Event"`}],raw:`Resource<"Event">`}],raw:`Resource<"Event">[]`},description:``},now:{required:!0,tsType:{name:`string`},description:``},showObject:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`false`,computed:!1}}}}})))()}var l,u,d,f;function p(){return(p=e((()=>{c(),l={title:`fleet/EventList`,component:o,args:{now:`2026-10-15T12:00:00Z`,events:[]}},u={name:`Events`,args:{events:[{type:`Normal`,reason:`Created`,note:`Created the StatefulSet orders-zeebe`,eventTime:`2026-10-15T09:00:00Z`},{type:`Warning`,reason:`BackOff`,note:`Back-off restarting the failed container`,eventTime:`2026-10-15T11:50:00Z`,series:{count:3,lastObservedTime:`2026-10-15T11:59:00Z`}}]}},d={name:`Empty`},f=[`Events`,`Empty`],u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Events",
  args: {
    events: [{
      type: "Normal",
      reason: "Created",
      note: "Created the StatefulSet orders-zeebe",
      eventTime: "2026-10-15T09:00:00Z"
    }, {
      type: "Warning",
      reason: "BackOff",
      note: "Back-off restarting the failed container",
      eventTime: "2026-10-15T11:50:00Z",
      series: {
        count: 3,
        lastObservedTime: "2026-10-15T11:59:00Z"
      }
    }]
  }
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Empty"
}`,...d.parameters?.docs?.source}}}})))()}p();export{d as Empty,u as Events,f as __namedExportsOrder,l as default};