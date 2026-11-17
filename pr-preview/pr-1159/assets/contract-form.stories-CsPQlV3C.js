import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{i as n,n as r,r as i}from"./namespaced-secret-ref-picker-B6q0kxsO.js";import{n as a,t as o}from"./fixtures-PmE5ozkW.js";import{a as s,c,i as l,s as u}from"./draft-KlHejKyE.js";function d(e){return{..._,spec:e}}function f(e){return{kind:e.kind,namespace:e.metadata.namespace,draft:l(e)}}var p,m,h,g,_,v,y,b,x,S,C,w,T,E,D,O,k;function A(){return(A=e((()=>{p=t(),i(),c(),s(),a(),{fn:m}=__STORYBOOK_MODULE_TEST__,h=n({"team-a":[{name:`shared-es-user`,keys:[`user`,`pass`]},{name:`shared-es-ca`,keys:[`ca.crt`]},{name:`s3-keys`,keys:[`access-key-id`,`secret-access-key`]},{name:`gcs-key`,keys:[`key.json`]},{name:`postgres-admin`,keys:[`username`,`password`]},{name:`orders-db`,keys:[`user`,`pass`]},{name:`orders-db-backup`,keys:[`user`,`pass`]}],platform:[{name:`plane-a-optimize-client`,keys:[`client-secret`]}]}),g=o(),_=g.objectStorageConfigs[0],v={title:`setup/connections/ContractForm`,component:u,args:{...f(_),errors:[],namespaces:[`camunda-system`,`platform`,`team-a`],databaseConfigs:[`orders-db`,`billing-db`],databaseServerConfigs:[`postgres`],onChange:m()},decorators:[e=>(0,p.jsx)(r.Provider,{value:h,children:(0,p.jsx)(`div`,{className:`max-w-3xl`,children:(0,p.jsx)(e,{})})})]},y={name:`S3 with workload identity`},b={name:`S3 with a key pair`,args:f(d({type:`S3`,s3:{bucketName:`acme-backups`,endpoint:`https://minio.team-a:9000`,forcePathStyle:!0,auth:{type:`credentials`,credentials:{secretRef:{name:`s3-keys`,accessKeyIdKey:`access-key-id`,secretAccessKeyKey:`secret-access-key`}}}}}))},x={name:`GCS with a key`,args:f(d({type:`GCS`,gcs:{bucketName:`acme-backups`,basePath:`orders`,auth:{type:`credentials`,credentials:{secretRef:{name:`gcs-key`,key:`key.json`}}}}}))},S={name:`Azure with workload identity`,args:f(d({type:`AzureBlob`,azureBlob:{accountName:`acmebackups`,container:`orders`,auth:{type:`workloadIdentity`,workloadIdentity:{clientId:`8f1c2a90-3c41-4f4e-9d1b-1f0b3e7c2d55`}}}}))},C={name:`Elasticsearch with a CA certificate`,args:f(g.secondaryStorageConfigs[1])},w={name:`RDBMS`,args:f({...g.secondaryStorageConfigs[1],spec:{type:`rdbms`,rdbms:{databaseConfigRef:`orders-db`}}})},T={name:`Database server`,args:f(g.databaseServerConfigs[0])},E={name:`Database with backup credentials`,args:f({...g.databaseConfigs[0],spec:{...g.databaseConfigs[0].spec,backupCredentialsSecretRef:{name:`orders-db-backup`}}})},D={name:`Management Identity`,args:f(g.managementAuthConfigs[0])},O={name:`Draft errors`,args:{...f(d({type:`S3`,s3:{bucketName:``}})),errors:[{path:`spec.s3.bucketName`,message:`Enter the bucket name.`},{path:`spec.s3.region`,message:`Enter the region, or an endpoint.`}]}},k=[`S3WithWorkloadIdentity`,`S3WithAKeyPair`,`GcsWithAKey`,`AzureWithWorkloadIdentity`,`ElasticsearchWithACaCertificate`,`Rdbms`,`DatabaseServer`,`DatabaseWithBackupCredentials`,`ManagementIdentity`,`DraftErrors`],y.parameters={...y.parameters,docs:{...y.parameters?.docs,source:{originalSource:`{
  name: "S3 with workload identity"
}`,...y.parameters?.docs?.source}}},b.parameters={...b.parameters,docs:{...b.parameters?.docs,source:{originalSource:`{
  name: "S3 with a key pair",
  args: argsOf(objectStorage({
    type: "S3",
    s3: {
      bucketName: "acme-backups",
      endpoint: "https://minio.team-a:9000",
      forcePathStyle: true,
      auth: {
        type: "credentials",
        credentials: {
          secretRef: {
            name: "s3-keys",
            accessKeyIdKey: "access-key-id",
            secretAccessKeyKey: "secret-access-key"
          }
        }
      }
    }
  }))
}`,...b.parameters?.docs?.source}}},x.parameters={...x.parameters,docs:{...x.parameters?.docs,source:{originalSource:`{
  name: "GCS with a key",
  args: argsOf(objectStorage({
    type: "GCS",
    gcs: {
      bucketName: "acme-backups",
      basePath: "orders",
      auth: {
        type: "credentials",
        credentials: {
          secretRef: {
            name: "gcs-key",
            key: "key.json"
          }
        }
      }
    }
  }))
}`,...x.parameters?.docs?.source}}},S.parameters={...S.parameters,docs:{...S.parameters?.docs,source:{originalSource:`{
  name: "Azure with workload identity",
  args: argsOf(objectStorage({
    type: "AzureBlob",
    azureBlob: {
      accountName: "acmebackups",
      container: "orders",
      auth: {
        type: "workloadIdentity",
        workloadIdentity: {
          clientId: "8f1c2a90-3c41-4f4e-9d1b-1f0b3e7c2d55"
        }
      }
    }
  }))
}`,...S.parameters?.docs?.source}}},C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
  name: "Elasticsearch with a CA certificate",
  args: argsOf(sources.secondaryStorageConfigs[1])
}`,...C.parameters?.docs?.source}}},w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
  name: "RDBMS",
  args: argsOf({
    ...sources.secondaryStorageConfigs[1],
    spec: {
      type: "rdbms",
      rdbms: {
        databaseConfigRef: "orders-db"
      }
    }
  })
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
  name: "Database server",
  args: argsOf(sources.databaseServerConfigs[0])
}`,...T.parameters?.docs?.source}}},E.parameters={...E.parameters,docs:{...E.parameters?.docs,source:{originalSource:`{
  name: "Database with backup credentials",
  args: argsOf({
    ...sources.databaseConfigs[0],
    spec: {
      ...sources.databaseConfigs[0].spec,
      backupCredentialsSecretRef: {
        name: "orders-db-backup"
      }
    }
  })
}`,...E.parameters?.docs?.source}}},D.parameters={...D.parameters,docs:{...D.parameters?.docs,source:{originalSource:`{
  name: "Management Identity",
  args: argsOf(sources.managementAuthConfigs[0])
}`,...D.parameters?.docs?.source}}},O.parameters={...O.parameters,docs:{...O.parameters?.docs,source:{originalSource:`{
  name: "Draft errors",
  args: {
    ...argsOf(objectStorage({
      type: "S3",
      s3: {
        bucketName: ""
      }
    })),
    errors: [{
      path: "spec.s3.bucketName",
      message: "Enter the bucket name."
    }, {
      path: "spec.s3.region",
      message: "Enter the region, or an endpoint."
    }]
  }
}`,...O.parameters?.docs?.source}}}})))()}A();export{S as AzureWithWorkloadIdentity,T as DatabaseServer,E as DatabaseWithBackupCredentials,O as DraftErrors,C as ElasticsearchWithACaCertificate,x as GcsWithAKey,D as ManagementIdentity,w as Rdbms,b as S3WithAKeyPair,y as S3WithWorkloadIdentity,k as __namedExportsOrder,v as default};