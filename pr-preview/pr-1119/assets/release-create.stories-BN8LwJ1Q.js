import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,r,t as i}from"./fixtures-BwQmGF8f.js";import{n as a,t as o}from"./release-settings-Cdk7hnXE.js";import{t as s}from"./sources-CF_bR0iS.js";function c({sources:e,imagesTable:t,from:n,mode:r,actions:i}){return(0,l.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,l.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em]`,children:`New release`}),(0,l.jsx)(o,{sources:e,copyFrom:n,imagesTable:t,secrets:{},mode:r,actions:i})]})}var l;function u(){return(u=e((()=>{l=t(),a(),s(),c.__docgenInfo={description:`ReleaseCreateView is the create page of a release on sources.`,methods:[],displayName:`ReleaseCreateView`,props:{sources:{required:!0,tsType:{name:`intersection`,raw:`ReleasesData & { errors: ReadError[] } & SourceReads`,elements:[{name:`signature`,type:`object`,raw:`{
  releases: CamundaRelease[];
  environments: Environment[];
  namespaces: V1Namespace[];
  clusters: CamundaCluster[];
  elasticsearchClusters: ElasticsearchCluster[];
  databaseServers: DatabaseServer[];
  claims: CamundaClusterClaim[];
}`,signature:{properties:[{key:`releases`,value:{name:`Array`,elements:[{name:`CamundaRelease`}],raw:`CamundaRelease[]`,required:!0}},{key:`environments`,value:{name:`Array`,elements:[{name:`Environment`}],raw:`Environment[]`,required:!0}},{key:`namespaces`,value:{name:`Array`,elements:[{name:`V1Namespace`}],raw:`V1Namespace[]`,required:!0}},{key:`clusters`,value:{name:`Array`,elements:[{name:`CamundaCluster`}],raw:`CamundaCluster[]`,required:!0}},{key:`elasticsearchClusters`,value:{name:`Array`,elements:[{name:`ElasticsearchCluster`}],raw:`ElasticsearchCluster[]`,required:!0}},{key:`databaseServers`,value:{name:`Array`,elements:[{name:`DatabaseServer`}],raw:`DatabaseServer[]`,required:!0}},{key:`claims`,value:{name:`Array`,elements:[{name:`CamundaClusterClaim`}],raw:`CamundaClusterClaim[]`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{ errors: ReadError[] }`,signature:{properties:[{key:`errors`,value:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{
  errors?: readonly ReadError[];
  stale?: readonly string[];
}`,signature:{properties:[{key:`errors`,value:{name:`unknown`,required:!1}},{key:`stale`,value:{name:`unknown`,required:!1}}]}}]},description:``},imagesTable:{required:!0,tsType:{name:`ImagesTable`},description:``},from:{required:!1,tsType:{name:`string`},description:``},mode:{required:!1,tsType:{name:`WriteMode`},description:``},actions:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  previewChange: (change: Change) => Promise<Preview>;
  applyChange: (change: Change) => Promise<WriteResult>;
  previewMove: (change: Change) => Promise<Preview>;
  applyMove: (change: Change) => Promise<WriteResult>;
}`,signature:{properties:[{key:`previewChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`applyChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}},{key:`previewMove`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`applyMove`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}}]}},description:``}}}})))()}var d,f,p,m,h,g;function _(){return(_=e((()=>{n(),u(),{fn:d}=__STORYBOOK_MODULE_TEST__,f={title:`setup/releases/ReleaseCreateView`,component:c,args:{sources:{...r(),errors:[]},imagesTable:i,mode:`direct`,actions:{previewChange:d(async()=>({objects:[],errors:[]})),applyChange:d(async()=>({ok:!0,mode:`direct`})),previewMove:d(async()=>({objects:[],errors:[]})),applyMove:d(async()=>({ok:!0,mode:`direct`}))}}},p={name:`Empty release`},m={name:`Copy of a release`,args:{from:`camunda-8-9-0`}},h={name:`Changes off`,args:{mode:void 0}},g=[`EmptyRelease`,`CopyOfARelease`,`ChangesOff`],p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Empty release"
}`,...p.parameters?.docs?.source}}},m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "Copy of a release",
  args: {
    from: "camunda-8-9-0"
  }
}`,...m.parameters?.docs?.source}}},h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "Changes off",
  args: {
    mode: undefined
  }
}`,...h.parameters?.docs?.source}}}})))()}_();export{h as ChangesOff,m as CopyOfARelease,p as EmptyRelease,g as __namedExportsOrder,f as default};