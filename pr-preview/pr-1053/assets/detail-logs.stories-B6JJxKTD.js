import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{a as r,i,n as a,r as o}from"./detail-logs-BHadMX0m.js";import{r as s,t as c}from"./fixtures-Dju11n-g.js";function l(e,t,n,i,a){let o=`2026-10-30T11:58:${String(t).padStart(2,`0`)}.120Z`;return{...r(JSON.stringify({timestamp:o,severity:n,logger_name:i,message:a})),pod:e}}function u(e){let[t,n]=(0,f.useState)(e.picked);return(0,d.jsx)(a,{...e,picked:t,onPick:t=>{e.onPick?.(t),n(t)}})}var d,f,p,m,h,g,_;function v(){return(v=e((()=>{d=n(),f=t(),i(),o(),s(),{fn:p}=__STORYBOOK_MODULE_TEST__,m={title:`cluster/DetailLogs`,component:a,args:{picked:``,onPick:p()},render:e=>(0,d.jsx)(u,{...e},e.picked)},h={name:`All pods of the brokers`,args:{pods:c.map(e=>e.metadata.name),lines:[l(`orders-zeebe-0`,2,`INFO`,`io.camunda.zeebe.broker.system`,`Broker 0 is ready`),l(`orders-zeebe-1`,3,`INFO`,`io.camunda.zeebe.broker.system`,`Broker 1 is ready`),l(`orders-zeebe-0`,9,`INFO`,`io.camunda.zeebe.raft`,`Partition 1 transitioned to LEADER in term 4`),l(`orders-zeebe-2`,11,`WARN`,`io.camunda.zeebe.broker.system`,`Disk usage is above 80 percent of the data volume`),l(`orders-zeebe-2`,14,`ERROR`,`io.camunda.zeebe.journal`,`Could not append to the journal of partition 3, because the disk is full`),l(`orders-zeebe-1`,20,`INFO`,`io.camunda.zeebe.raft`,`Partition 2 transitioned to LEADER in term 7`)],errors:[]}},g={name:`A request with its controller`,args:{pods:[`rustfs-0`],controllerPods:[`rustfs-controller-6f9c-2xk4p`],lines:[l(`rustfs-0`,1,`INFO`,`rustfs::server`,`Bucket orders-backup-storage is ready`),{...r(JSON.stringify({ts:`2026-10-30T11:58:04.000Z`,level:`info`,logger:`plugin`,msg:`Reconciled the bucket`,request:{namespace:`team-a`,name:`orders-backup-storage`}})),pod:`rustfs-controller-6f9c-2xk4p`}],errors:[{pod:`rustfs-0`,error:`The connection to the log stream broke`}]}},_=[`AllPodsOfTheBrokers`,`ARequestWithItsController`],h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "All pods of the brokers",
  args: {
    pods: brokers.map(p => p.metadata.name),
    lines: [logLine("orders-zeebe-0", 2, "INFO", "io.camunda.zeebe.broker.system", "Broker 0 is ready"), logLine("orders-zeebe-1", 3, "INFO", "io.camunda.zeebe.broker.system", "Broker 1 is ready"), logLine("orders-zeebe-0", 9, "INFO", "io.camunda.zeebe.raft", "Partition 1 transitioned to LEADER in term 4"), logLine("orders-zeebe-2", 11, "WARN", "io.camunda.zeebe.broker.system", "Disk usage is above 80 percent of the data volume"), logLine("orders-zeebe-2", 14, "ERROR", "io.camunda.zeebe.journal", "Could not append to the journal of partition 3, because the disk is full"), logLine("orders-zeebe-1", 20, "INFO", "io.camunda.zeebe.raft", "Partition 2 transitioned to LEADER in term 7")],
    errors: []
  }
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "A request with its controller",
  args: {
    pods: ["rustfs-0"],
    controllerPods: ["rustfs-controller-6f9c-2xk4p"],
    lines: [logLine("rustfs-0", 1, "INFO", "rustfs::server", "Bucket orders-backup-storage is ready"), {
      ...parseLogLine(JSON.stringify({
        ts: "2026-10-30T11:58:04.000Z",
        level: "info",
        logger: "plugin",
        msg: "Reconciled the bucket",
        request: {
          namespace: "team-a",
          name: "orders-backup-storage"
        }
      })),
      pod: "rustfs-controller-6f9c-2xk4p"
    }],
    errors: [{
      pod: "rustfs-0",
      error: "The connection to the log stream broke"
    }]
  }
}`,...g.parameters?.docs?.source}}}})))()}v();export{g as ARequestWithItsController,h as AllPodsOfTheBrokers,_ as __namedExportsOrder,m as default};