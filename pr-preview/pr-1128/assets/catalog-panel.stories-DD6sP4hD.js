import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{i as t,n,r,t as i}from"./catalog-panel-CGqkv9Gx.js";import{i as a,n as o}from"./fixtures-BSCp3F5s.js";var s,c,l,u,d,f,p;function m(){return(m=e((()=>{r(),n(),o(),s=a().sections,c={title:`setup/plugins/CatalogPanel`,component:i,args:{charts:t.charts,sections:s}},l={name:`Seven charts`},u={name:`One chart with two sections`,args:{charts:[{name:`aws-identity`,description:`Creates one AWS IAM role for each cluster and one for its Connectors.`,sections:[`cloud-identity`,`connectors-permissions`],chart:`oci://ghcr.io/konsole-is/camunda-operator-plugins/charts/aws-identity`,version:`0.1.0`,repository:`https://github.com/konsole-is/camunda-operator-plugins`,installCommand:`helm install aws-identity oci://ghcr.io/konsole-is/camunda-operator-plugins/charts/aws-identity --version 0.1.0 --namespace camunda-operator-plugins --create-namespace`}]}},d={name:`The catalog cannot be read`,args:{error:`reading catalog /app/catalog/official-plugins.json: ENOENT: no such file or directory`}},f={name:`Which charts are installed is not known`,args:{installedUnknown:!0}},p=[`SevenCharts`,`OneChartWithTwoSections`,`CatalogNotRead`,`InstalledNotKnown`],l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Seven charts"
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "One chart with two sections",
  args: {
    charts: [{
      name: "aws-identity",
      description: "Creates one AWS IAM role for each cluster and one for its Connectors.",
      sections: ["cloud-identity", "connectors-permissions"],
      chart: "oci://ghcr.io/konsole-is/camunda-operator-plugins/charts/aws-identity",
      version: "0.1.0",
      repository: "https://github.com/konsole-is/camunda-operator-plugins",
      installCommand: "helm install aws-identity oci://ghcr.io/konsole-is/camunda-operator-plugins/charts/aws-identity --version 0.1.0 --namespace camunda-operator-plugins --create-namespace"
    }]
  }
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "The catalog cannot be read",
  args: {
    error: "reading catalog /app/catalog/official-plugins.json: ENOENT: no such file or directory"
  }
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Which charts are installed is not known",
  args: {
    installedUnknown: true
  }
}`,...f.parameters?.docs?.source}}}})))()}m();export{d as CatalogNotRead,f as InstalledNotKnown,u as OneChartWithTwoSections,l as SevenCharts,p as __namedExportsOrder,c as default};