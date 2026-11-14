import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./data-table-DceIS0m9.js";import{n as i,t as a}from"./status-chip-DikExibM.js";var o,s,c,l,u,d,f,p,m,h,g,_;function v(){return(v=e((()=>{o=t(),n(),i(),{fn:s}=__STORYBOOK_MODULE_TEST__,c=[{key:`name`,label:`Name`,sortable:!0},{key:`environment`,label:`Environment`},{key:`version`,label:`Version`,sortable:!0},{key:`status`,label:`Status`}],l=[[`orders`,`Production`,`8.9.20`,!0],[`billing`,`Production`,`8.9.23`,!0],[`payments`,`Development`,`8.9.21`,!1],[`search`,`Development`,`8.9.19`,!0]].map(([e,t,n,r])=>({id:e,href:`/fleet/clusters/${e}`,cells:{name:e,environment:t,version:n,status:(0,o.jsx)(a,{condition:r?{status:`True`}:{status:`False`,reason:`Converging`}})},search:e,sort:{name:e,version:n},filters:{environment:t},applicable:n!==`8.9.23`})),u=[{key:`environment`,label:`Environment`,options:[{value:`Production`,label:`Production`},{value:`Development`,label:`Development`}]}],d={applicableLabel:`clusters that can upgrade`,onChange:s()},f={title:`fleet/DataTable`,component:r,args:{label:`Clusters`,columns:c,rows:l}},p={name:`Nothing selected`,args:{filters:u,selection:d}},m={name:`Rows that are not applicable`,args:{rows:l.map(e=>({...e,applicable:e.id===`orders`})),selection:d}},h={name:`Pages`,args:{pageSize:2}},g={name:`Empty`,args:{rows:[],emptyText:`No clusters yet. Create one to see it here.`}},_=[`NothingSelected`,`RowsThatAreNotApplicable`,`Pages`,`Empty`],p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Nothing selected",
  args: {
    filters,
    selection
  }
}`,...p.parameters?.docs?.source}}},m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "Rows that are not applicable",
  args: {
    rows: rows.map(r => ({
      ...r,
      applicable: r.id === "orders"
    })),
    selection
  }
}`,...m.parameters?.docs?.source}}},h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "Pages",
  args: {
    pageSize: 2
  }
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "Empty",
  args: {
    rows: [],
    emptyText: "No clusters yet. Create one to see it here."
  }
}`,...g.parameters?.docs?.source}}}})))()}v();export{g as Empty,p as NothingSelected,h as Pages,m as RowsThatAreNotApplicable,_ as __namedExportsOrder,f as default};