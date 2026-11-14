import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./event-list-CfuH6S9H.js";import{i as r,r as i}from"./fixtures-Dju11n-g.js";var a,o,s,c;function l(){return(l=e((()=>{t(),i(),a=(e,t,n,i,a=`Normal`)=>({metadata:{name:`${n.name}.${e}`,uid:`${n.name}/${e}`},type:a,reason:e,note:t,eventTime:new Date(Date.parse(r)-i*6e4).toISOString(),regarding:{...n,namespace:`team-a`}}),o={title:`cluster/DetailEvents`,component:n,args:{now:r,showObject:!0}},s={name:`Events of the brokers`,args:{events:[a(`BackOff`,`Back-off restarting failed container zeebe`,{kind:`Pod`,name:`orders-zeebe-2`},3,`Warning`),a(`SuccessfulCreate`,`create Pod orders-zeebe-2 in StatefulSet orders-zeebe successful`,{kind:`StatefulSet`,name:`orders-zeebe`},14),a(`CreatedStatefulSet`,`Created StatefulSet 'orders-zeebe'`,{kind:`CamundaCluster`,name:`orders`},62)]}},c=[`EventsOfTheBrokers`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Events of the brokers",
  args: {
    events: [event("BackOff", "Back-off restarting failed container zeebe", {
      kind: "Pod",
      name: "orders-zeebe-2"
    }, 3, "Warning"), event("SuccessfulCreate", "create Pod orders-zeebe-2 in StatefulSet orders-zeebe successful", {
      kind: "StatefulSet",
      name: "orders-zeebe"
    }, 14), event("CreatedStatefulSet", "Created StatefulSet 'orders-zeebe'", {
      kind: "CamundaCluster",
      name: "orders"
    }, 62)]
  }
}`,...s.parameters?.docs?.source}}}})))()}l();export{s as EventsOfTheBrokers,c as __namedExportsOrder,o as default};