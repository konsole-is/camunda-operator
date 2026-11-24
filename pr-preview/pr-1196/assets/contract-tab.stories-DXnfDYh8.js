import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{i as t,l as n,o as r}from"./fixtures-kgATjJaf.js";import{i,n as a,r as o,t as s}from"./contract-tab-KA7rmk10.js";var c,l,u,d,f,p,m;function h(){return(h=e((()=>{i(),a(),t(),c=n(),l=c.registrations.find(e=>e.metadata.name===`elasticsearch`),u={title:`setup/plugins/ContractTab`,component:s,args:{contract:o(l,c.sections,[])}},d={name:`Built-in elasticsearch`},f={name:`Required input with formats`,args:{contract:o(r(`aws-s3`,{sections:[`backup-storage`],inputs:[{name:`kmsKeyArn`,from:`encryption-key/keyArn`,formats:[`aws-kms-arn`],required:!0},{name:`principal`,from:`cloud-identity/roleArn`}],outputs:[{key:`objectStorageConfig`,description:`The name of the ObjectStorageConfig of the bucket.`},{key:`bucketArn`,format:`aws-s3-arn`}]}),c.sections,[])}},p={name:`Delete only`,args:{contract:o(r(`rustfs`,{sections:[`backup-storage`,`removed-section`],deletionPolicies:[`Delete`]}),c.sections,[])}},m=[`BuiltInElasticsearch`,`RequiredInput`,`DeleteOnly`],d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Built-in elasticsearch"
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Required input with formats",
  args: {
    contract: contractOf(pluginRegistration("aws-s3", {
      sections: ["backup-storage"],
      inputs: [{
        name: "kmsKeyArn",
        from: "encryption-key/keyArn",
        formats: ["aws-kms-arn"],
        required: true
      }, {
        name: "principal",
        from: "cloud-identity/roleArn"
      }],
      outputs: [{
        key: "objectStorageConfig",
        description: "The name of the ObjectStorageConfig of the bucket."
      }, {
        key: "bucketArn",
        format: "aws-s3-arn"
      }]
    }), sources.sections, [])
  }
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Delete only",
  args: {
    contract: contractOf(pluginRegistration("rustfs", {
      sections: ["backup-storage", "removed-section"],
      deletionPolicies: ["Delete"]
    }), sources.sections, [])
  }
}`,...p.parameters?.docs?.source}}}})))()}h();export{d as BuiltInElasticsearch,p as DeleteOnly,f as RequiredInput,m as __namedExportsOrder,u as default};