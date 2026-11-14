import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r,r as i}from"./detail-metrics-CNXytxYX.js";import{a,r as o,s,t as c}from"./fixtures-Dju11n-g.js";function l(e,t){return c.map((n,r)=>({name:n.metadata.name,points:Array.from({length:30},(n,i)=>[m+i*12e4,e*(1+r*.15)+t*Math.sin((i+r*3)/4)])}))}function u(e){let[t,n]=(0,f.useState)(e.range);return(0,d.jsx)(r,{...e,range:t,onRange:t=>{e.onRange?.(t),n(t)}})}var d,f,p,m,h,g,_,v,y,b,x,S,C,w;function T(){return(T=e((()=>{d=n(),f=t(),i(),o(),{fn:p}=__STORYBOOK_MODULE_TEST__,m=Date.parse(`2026-10-30T11:00:00Z`),h=1024**3,g=[s(`orders-zeebe-0`,.8,2.6),s(`orders-zeebe-1`,1.1,3.1),s(`orders-zeebe-2`,.4,1.4)].map(e=>({...e,cpu:{...e.cpu,requestCores:1},memory:{...e.memory,requestBytes:2*h}})),_=[{label:`CPU`,unit:`cores`,source:``,series:l(.8,.2)},{label:`Disk used`,unit:`bytes`,source:``,series:l(12*h,.4*h).map(e=>({...e,name:`data-${e.name}`}))},{label:`Process instances activated`,unit:`instances/s`,source:``,series:l(40,12)}],v={title:`cluster/DetailMetrics`,component:r,args:{pods:c,usage:g,range:`1h`,onRange:p()},render:e=>(0,d.jsx)(u,{...e},e.range)},y={name:`Charts of the brokers`,args:{charts:{prometheus:`answered`,charts:_,errors:[]}}},b={name:`Prometheus not configured`,args:{charts:{prometheus:`notConfigured`}}},x={name:`No Camunda charts for the version`,args:{charts:{prometheus:`answered`,charts:_.slice(0,2),errors:[],hiddenForVersion:{version:`8.10.0`}}}},S={name:`No metrics server`,args:{usage:`unavailable`,charts:{prometheus:`answered`,charts:_.slice(0,1),errors:[]},range:`6h`}},C={name:`A request query that does not render`,args:{pods:[a(`rustfs-0`,`pool-a-1`)],usage:[s(`rustfs-0`,.2,.6)],charts:{prometheus:`answered`,charts:[{label:`Requests`,unit:`req/s`,source:`rustfs-bucket`,declaredBy:`Bucket on RustFS`,series:l(120,30).slice(0,1).map(e=>({...e,name:`rustfs-0`}))}],errors:[{label:`Latency`,declaredBy:`Bucket on RustFS`,error:`output "bucket" has no value`}]}}},w=[`ChartsOfTheBrokers`,`PrometheusNotConfigured`,`NoCamundaChartsForTheVersion`,`NoMetricsServer`,`ARequestQueryThatDoesNotRender`],y.parameters={...y.parameters,docs:{...y.parameters?.docs,source:{originalSource:`{
  name: "Charts of the brokers",
  args: {
    charts: {
      prometheus: "answered",
      charts: brokerCharts,
      errors: []
    }
  }
}`,...y.parameters?.docs?.source}}},b.parameters={...b.parameters,docs:{...b.parameters?.docs,source:{originalSource:`{
  name: "Prometheus not configured",
  args: {
    charts: {
      prometheus: "notConfigured"
    }
  }
}`,...b.parameters?.docs?.source}}},x.parameters={...x.parameters,docs:{...x.parameters?.docs,source:{originalSource:`{
  name: "No Camunda charts for the version",
  args: {
    charts: {
      prometheus: "answered",
      charts: brokerCharts.slice(0, 2),
      errors: [],
      hiddenForVersion: {
        version: "8.10.0"
      }
    }
  }
}`,...x.parameters?.docs?.source}}},S.parameters={...S.parameters,docs:{...S.parameters?.docs,source:{originalSource:`{
  name: "No metrics server",
  args: {
    usage: "unavailable",
    charts: {
      prometheus: "answered",
      charts: brokerCharts.slice(0, 1),
      errors: []
    },
    range: "6h"
  }
}`,...S.parameters?.docs?.source}}},C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
  name: "A request query that does not render",
  args: {
    pods: [pod("rustfs-0", "pool-a-1")],
    usage: [usage("rustfs-0", 0.2, 0.6)],
    charts: {
      prometheus: "answered",
      charts: [{
        label: "Requests",
        unit: "req/s",
        source: "rustfs-bucket",
        declaredBy: "Bucket on RustFS",
        series: sine(120, 30).slice(0, 1).map(s => ({
          ...s,
          name: "rustfs-0"
        }))
      }],
      errors: [{
        label: "Latency",
        declaredBy: "Bucket on RustFS",
        error: 'output "bucket" has no value'
      }]
    }
  }
}`,...C.parameters?.docs?.source}}}})))()}T();export{C as ARequestQueryThatDoesNotRender,y as ChartsOfTheBrokers,x as NoCamundaChartsForTheVersion,S as NoMetricsServer,b as PrometheusNotConfigured,w as __namedExportsOrder,v as default};