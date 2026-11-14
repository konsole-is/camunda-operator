import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{i as r,n as i,o as a,r as o,t as s}from"./sections-table-DE8PhbhT.js";import{n as c,r as l,t as u}from"./fixtures-ODd7kMxO.js";function d({sources:e,spec:t,sections:n,onPolicy:i}){let[o,c]=(0,p.useState)(t),l=r(e.sections,e.registrations,o).map(e=>({...e,rows:e.rows.filter(e=>!n||n.includes(e.name))}));return(0,f.jsx)(s,{groups:l,errors:[],onPolicy:(e,t)=>{i(e,t),c(n=>a(n,e,t))}})}var f,p,m,h,g,_,v,y,b;function x(){return(x=e((()=>{f=n(),p=t(),c(),o(),i(),{fn:m}=__STORYBOOK_MODULE_TEST__,h=u(),g=l(),_={title:`setup/environments/SectionsTable`,component:d,args:{sources:h,spec:h.environments[2].spec,onPolicy:m()},render:e=>(0,f.jsx)(d,{...e},JSON.stringify(e.spec))},v={name:`Platform environment`},y={name:`Notes`,args:{sources:g,spec:{sections:{...g.environments[0].spec.sections,"encryption-key":{allowed:[`aws-kms`,`vault-transit`]}}},sections:[`encryption-key`,`volume-encryption`]}},b=[`PlatformEnvironment`,`Notes`],v.parameters={...v.parameters,docs:{...v.parameters?.docs,source:{originalSource:`{
  name: "Platform environment"
}`,...v.parameters?.docs?.source}}},y.parameters={...y.parameters,docs:{...y.parameters?.docs,source:{originalSource:`{
  name: "Notes",
  args: {
    sources: withSections,
    spec: {
      sections: {
        ...withSections.environments[0].spec.sections,
        "encryption-key": {
          allowed: ["aws-kms", "vault-transit"]
        }
      }
    },
    sections: ["encryption-key", "volume-encryption"]
  }
}`,...y.parameters?.docs?.source}}}})))()}x();export{y as Notes,v as PlatformEnvironment,b as __namedExportsOrder,_ as default};