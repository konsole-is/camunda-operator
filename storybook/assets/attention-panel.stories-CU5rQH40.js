import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{i as t,n,r,t as i}from"./attention-panel-CYum3VKI.js";import{d as a,g as o,h as s,m as c,n as l,s as u}from"./model-r69xCmQS.js";var d,f,p,m,h;function g(){return(g=e((()=>{t(),n(),s(),u(),d=o.map(e=>({...e,ready:{status:`False`,reason:`Degraded`,message:`The Keycloak pod is not ready`}})),f={kind:`PluginRegistration`,metadata:{name:`aws-s3-bucket`},spec:{label:`Bucket on Amazon S3`},status:{conditions:[{type:`Ready`,status:`False`,reason:`ControllerNotRunning`,message:`No controller pod of the plugin is ready`}]}},p={title:`fleet-view/AttentionPanel`,component:i},m={name:`Mixed`,args:{items:r({rows:l(c,a),backups:c.backups,registrations:[f],planes:d,now:a})}},h=[`Mixed`],m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "Mixed",
  args: {
    items: attentionItems({
      rows: clusterRows(fleet, NOW),
      backups: fleet.backups,
      registrations: [failingRegistration],
      planes: downPlanes,
      now: NOW
    })
  }
}`,...m.parameters?.docs?.source}}}})))()}g();export{m as Mixed,h as __namedExportsOrder,p as default};