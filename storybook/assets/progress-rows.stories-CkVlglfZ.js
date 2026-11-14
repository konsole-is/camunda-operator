import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./link-DOLMGIR2.js";import{n as i,t as a}from"./progress-rows-CCf7OJUK.js";var o,s,c,l,u;function d(){return(d=e((()=>{o=t(),r(),i(),s={title:`fleet/ProgressRows`,component:a},c={name:`Waiting for outputs`,args:{ready:{status:`False`,reason:`WaitingForOutputs`,message:`Waiting for Backup storage and Secondary storage`},rows:[{id:`backup`,label:`Backup storage`,detail:`New S3 bucket`,summary:`Creating the bucket orders-backups`,checks:[{label:`Outputs`,condition:{status:`False`,reason:`Provisioning`}},{label:`Ready`,condition:{status:`False`,reason:`Provisioning`}}]},{id:`storage`,label:`Secondary storage`,detail:`New Elasticsearch`,checks:[{label:`Outputs`,condition:{status:`True`}},{label:`Ready`,condition:{status:`Unknown`,reason:`Converging`}}]},{id:`cluster`,label:`Cluster`,checks:[{label:`RequestsReady`,condition:{status:`False`,reason:`Waiting`}},{label:`ClusterReady`}]}]}},l={name:`Ready`,args:{ready:{status:`True`,reason:`Healthy`},rows:[{id:`cluster`,label:`Cluster`,checks:[{label:`RequestsReady`,condition:{status:`True`}},{label:`ClusterReady`,condition:{status:`True`}}]}],done:(0,o.jsx)(n,{href:`/fleet/clusters/team-a/orders`,className:`text-sm text-info`,children:`Open the cluster orders`})}},u=[`WaitingForOutputs`,`Ready`],c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Waiting for outputs",
  args: {
    ready: {
      status: "False",
      reason: "WaitingForOutputs",
      message: "Waiting for Backup storage and Secondary storage"
    },
    rows: [{
      id: "backup",
      label: "Backup storage",
      detail: "New S3 bucket",
      summary: "Creating the bucket orders-backups",
      checks: [{
        label: "Outputs",
        condition: {
          status: "False",
          reason: "Provisioning"
        }
      }, {
        label: "Ready",
        condition: {
          status: "False",
          reason: "Provisioning"
        }
      }]
    }, {
      id: "storage",
      label: "Secondary storage",
      detail: "New Elasticsearch",
      checks: [{
        label: "Outputs",
        condition: {
          status: "True"
        }
      }, {
        label: "Ready",
        condition: {
          status: "Unknown",
          reason: "Converging"
        }
      }]
    }, {
      id: "cluster",
      label: "Cluster",
      checks: [{
        label: "RequestsReady",
        condition: {
          status: "False",
          reason: "Waiting"
        }
      }, {
        label: "ClusterReady"
      }]
    }]
  }
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Ready",
  args: {
    ready: {
      status: "True",
      reason: "Healthy"
    },
    rows: [{
      id: "cluster",
      label: "Cluster",
      checks: [{
        label: "RequestsReady",
        condition: {
          status: "True"
        }
      }, {
        label: "ClusterReady",
        condition: {
          status: "True"
        }
      }]
    }],
    done: <Link href="/fleet/clusters/team-a/orders" className="text-sm text-info">
        Open the cluster orders
      </Link>
  }
}`,...l.parameters?.docs?.source}}}})))()}d();export{l as Ready,c as WaitingForOutputs,u as __namedExportsOrder,s as default};