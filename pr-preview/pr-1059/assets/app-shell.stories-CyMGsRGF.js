import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{i as n,n as r,r as i,t as a}from"./git-banner-i9D3eLKt.js";import{a as o,i as s,n as c,r as l,t as u}from"./fixtures-DU7FxC4G.js";import{n as d,t as f}from"./git-waiting-line-DwhutHJb.js";import{n as p,t as m}from"./setup-breadcrumb-1Kfc77rc.js";import{n as h,t as g}from"./setup-nav-DZ1zfooN.js";var _,v,y,b,x,S,C,w;function T(){return(T=e((()=>{_=t(),n(),l(),r(),d(),p(),h(),v=`/setup/releases/camunda-8-9-long-term-support-with-extended-security-patches-eu-north-1`,y={title:`fleet/AppShell`,component:i,args:{pathname:`/fleet`,children:c(`Fleet`)}},b={name:`Fleet page`},x={name:`Setup detail page`,args:{pathname:`/setup/presets/gold`,sideNav:(0,_.jsx)(g,{pathname:`/setup/presets/gold`}),children:(0,_.jsxs)(_.Fragment,{children:[(0,_.jsx)(m,{pathname:`/setup/presets/gold`}),c(`gold`)]})}},S={name:`Setup detail with a long name`,args:{pathname:`/setup/releases/x`,sideNav:(0,_.jsx)(g,{pathname:`/setup/releases/x`}),children:(0,_.jsxs)(_.Fragment,{children:[(0,_.jsx)(m,{pathname:v}),c(`Release`)]})}},C={name:`With Git waiting banner`,args:{banner:(0,_.jsxs)(a,{count:2,children:[(0,_.jsx)(f,{pushed:u,now:o}),(0,_.jsx)(f,{pushed:s,now:o})]})}},w=[`FleetPage`,`SetupDetailPage`,`SetupDetailWithALongName`,`WithGitWaitingBanner`],b.parameters={...b.parameters,docs:{...b.parameters?.docs,source:{originalSource:`{
  name: "Fleet page"
}`,...b.parameters?.docs?.source}}},x.parameters={...x.parameters,docs:{...x.parameters?.docs,source:{originalSource:`{
  name: "Setup detail page",
  args: {
    pathname: "/setup/presets/gold",
    sideNav: <SetupNav pathname="/setup/presets/gold" />,
    children: <>
        <SetupBreadcrumb pathname="/setup/presets/gold" />
        {heading("gold")}
      </>
  }
}`,...x.parameters?.docs?.source}}},S.parameters={...S.parameters,docs:{...S.parameters?.docs,source:{originalSource:`{
  name: "Setup detail with a long name",
  args: {
    pathname: "/setup/releases/x",
    sideNav: <SetupNav pathname="/setup/releases/x" />,
    children: <>
        <SetupBreadcrumb pathname={longRelease} />
        {heading("Release")}
      </>
  }
}`,...S.parameters?.docs?.source}}},C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
  name: "With Git waiting banner",
  args: {
    banner: <GitBannerView count={2}>
        <GitWaitingLine pushed={claimPush} now={now} />
        <GitWaitingLine pushed={namespacePush} now={now} />
      </GitBannerView>
  }
}`,...C.parameters?.docs?.source}}}})))()}T();export{b as FleetPage,x as SetupDetailPage,S as SetupDetailWithALongName,C as WithGitWaitingBanner,w as __namedExportsOrder,y as default};