import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{a as t,i as n,l as r,s as i,t as a}from"./fixtures-kgATjJaf.js";import{i as o,n as s,r as c,t as l}from"./used-by-tab-BHg3-fkq.js";var u,d,f,p,m,h,g,_,v,y;function b(){return(b=e((()=>{n(),c(),s(),u=[a(`OutputsReady`,`True`,`Healthy`),a(`Ready`,`True`,`Healthy`)],d=o(`elasticsearch`,r({requests:[i(`team-orders`,`orders-secondary-storage`,{registration:`elasticsearch`,section:`secondary-storage`,claimRef:{kind:`CamundaClusterClaim`,name:`orders`},target:{kind:`CamundaCluster`,name:`orders`},deletionPolicy:`Orphan`},{conditions:u,summary:[{label:`Health`,value:`green`,severity:`Info`},{label:`Nodes`,value:`3`}],resources:[{kind:`ElasticsearchCluster`,name:`orders-es`,role:`Search and secondary storage`,onOrphan:`Kept`},{kind:`SecondaryStorageConfig`,name:`orders-es`,onOrphan:`Kept`}]})]}),[]),f=o(`aws-s3`,r({requests:[i(`team-orders`,`orders-backup-storage`,{registration:`aws-s3`,claimRef:{kind:`CamundaClusterClaim`,name:`orders`},target:{kind:`CamundaCluster`,name:`orders`}},{conditions:[a(`OutputsReady`,`True`,`Healthy`),a(`Ready`,`False`,`GrantRejected`,`Grant orders-bucket-access is refused: OverCeiling`)],summary:[{label:`Bucket`,value:`cpo-orders-backups`,severity:`Info`}],resources:[{kind:`S3Bucket`,name:`cpo-orders-backups`,location:`eu-west-1`,onOrphan:`Kept`}]})]}),[t(`team-orders`,`orders-bucket-access`,{template:`bucket-access`},{"plugins.camunda.io/registration":`aws-s3`,"plugins.camunda.io/request":`orders-backup-storage`},[a(`Ready`,`False`,`OverCeiling`)])]),p=o(`management-database-server`,r({requests:[i(`team-orders`,`ops-management-database`,{registration:`management-database-server`,claimRef:{kind:`CamundaManagementClusterClaim`,name:`ops`},target:{kind:`CamundaManagementCluster`,name:`ops`}},{conditions:u,summary:[{label:`Databases`,value:`3 of 3 ready`}]})]}),[]),m={title:`setup/plugins/UsedByTab`,component:l,args:{rows:d,unread:[]}},h={name:`Built-in elasticsearch with its resources`},g={name:`A request with a refused Grant`,args:{rows:f}},_={name:`A management plane request`,args:{rows:p}},v={name:`No request`,args:{rows:[]}},y=[`BuiltInElasticsearch`,`RefusedGrant`,`ManagementPlane`,`NoRequest`],h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "Built-in elasticsearch with its resources"
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "A request with a refused Grant",
  args: {
    rows: s3Rows
  }
}`,...g.parameters?.docs?.source}}},_.parameters={..._.parameters,docs:{..._.parameters?.docs,source:{originalSource:`{
  name: "A management plane request",
  args: {
    rows: planeRows
  }
}`,..._.parameters?.docs?.source}}},v.parameters={...v.parameters,docs:{...v.parameters?.docs,source:{originalSource:`{
  name: "No request",
  args: {
    rows: []
  }
}`,...v.parameters?.docs?.source}}}})))()}b();export{h as BuiltInElasticsearch,_ as ManagementPlane,v as NoRequest,g as RefusedGrant,y as __namedExportsOrder,m as default};