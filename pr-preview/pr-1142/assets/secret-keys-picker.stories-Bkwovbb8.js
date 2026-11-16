import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{i as n,n as r,r as i}from"./namespaced-secret-ref-picker-uZupkYhV.js";import{a,i as o,r as s,t as c}from"./secret-keys-picker-Du-SJCzN.js";var l,u,d,f,p,m,h,g,_;function v(){return(v=e((()=>{l=t(),i(),a(),{fn:u}=__STORYBOOK_MODULE_TEST__,d=n({"team-a":[{name:`shared-es-user`,keys:[`user`,`pass`]},{name:`s3-keys`,keys:[`access-key-id`,`secret-access-key`]}],empty:[]}),f={title:`fleet/SecretKeysPicker`,component:o,args:{label:`Credentials`,namespace:`team-a`,slots:c,onChange:u()},decorators:[e=>(0,l.jsx)(r.Provider,{value:d,children:(0,l.jsx)(`div`,{className:`max-w-2xl`,children:(0,l.jsx)(e,{})})})],render:e=>(0,l.jsx)(o,{...e},JSON.stringify(e.value))},p={name:`Credentials with default keys`,args:{value:{name:`shared-es-user`,passwordKey:`pass`}}},m={name:`S3 key pair`,args:{label:`Access key`,slots:s,value:{name:`s3-keys`,accessKeyIdKey:`access-key-id`,secretAccessKeyKey:`secret-access-key`}}},h={name:`A Secret that is gone`,args:{value:{name:`old-es-user`,usernameKey:`user`}}},g={name:`No Secret`,args:{namespace:`empty`}},_=[`CredentialsWithDefaultKeys`,`S3KeyPair`,`ASecretThatIsGone`,`NoSecret`],p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Credentials with default keys",
  args: {
    value: {
      name: "shared-es-user",
      passwordKey: "pass"
    }
  }
}`,...p.parameters?.docs?.source}}},m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "S3 key pair",
  args: {
    label: "Access key",
    slots: S3_CREDENTIALS_SLOTS,
    value: {
      name: "s3-keys",
      accessKeyIdKey: "access-key-id",
      secretAccessKeyKey: "secret-access-key"
    }
  }
}`,...m.parameters?.docs?.source}}},h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "A Secret that is gone",
  args: {
    value: {
      name: "old-es-user",
      usernameKey: "user"
    }
  }
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "No Secret",
  args: {
    namespace: "empty"
  }
}`,...g.parameters?.docs?.source}}}})))()}v();export{h as ASecretThatIsGone,p as CredentialsWithDefaultKeys,g as NoSecret,m as S3KeyPair,_ as __namedExportsOrder,f as default};