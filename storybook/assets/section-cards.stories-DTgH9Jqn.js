import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./section-cards-DGQ5U67x.js";var r,i,a,o,s,c,l,u,d,f,p,m,h;function g(){return(g=e((()=>{t(),{fn:r}=__STORYBOOK_MODULE_TEST__,i={name:`backup-storage`,label:`Backup storage`,description:`Where the backups of the cluster go.`,selection:`One`,emptyOptionLabel:`No backups`,required:!1,options:[{registration:`aws-s3`,label:`New S3 bucket`,description:`A bucket for this cluster, deleted with the claim.`,isDefault:!1,parameters:[{name:`bucketName`,label:`Bucket name`,claimEditable:!1,summary:`team-orders-orders-backups`,note:`Named <namespace>-<claim>-backups`},{name:`retentionDays`,label:`Retention in days`,schema:{type:`integer`,minimum:1,maximum:365},claimEditable:!0,value:30}]},{registration:`existing-object-storage`,label:`Existing bucket`,description:`An ObjectStorageConfig of the namespace.`,isDefault:!0,parameters:[]}]},a={name:`connectors-permissions`,label:`Connectors permissions`,selection:`One`,emptyOptionLabel:`No cloud permissions`,required:!1,options:[{registration:`aws-connectors-role`,label:`Role for each cluster`,isDefault:!0,parameters:[{name:`policies`,label:`Policies`,schema:{type:`array`,items:{type:`string`}},claimEditable:!0,allowed:[{name:`sqs`,label:`SQS access`},{name:`s3`,label:`S3 read`},{name:`sns`,label:`SNS publish`}],value:[`s3`]},{name:`boundaryPolicies`,label:`Admin policies`,claimEditable:!1,summary:`3 policies`}]}]},o={title:`fleet/SectionCards`,component:n,args:{section:i,onChange:r()}},s={name:`Default and empty option`},c={name:`Required section`,args:{section:{...i,required:!0}}},l={name:`Allowed checkboxes`,args:{section:a}},u={name:`Allowed select`,args:{section:{...a,options:[{registration:`gcp-pubsub`,label:`Pub/Sub topics`,isDefault:!0,parameters:[{name:`access`,label:`Topic access`,schema:{type:`string`},claimEditable:!0,allowed:[{name:`publish`,label:`Publish to the team topics`},{name:`subscribe`,label:`Subscribe to the team topics`}]}]}]}}},d={name:`Map parameter`,args:{section:{...i,options:[{registration:`aws-s3`,label:`New S3 bucket`,description:`A bucket for this cluster, deleted with the claim.`,isDefault:!0,parameters:[{name:`tags`,label:`Bucket tags`,schema:{type:`object`,additionalProperties:{type:`string`}},claimEditable:!0,value:{team:`orders`,"cost-center":`4711`}}]}]}}},f={name:`Option that needs an input`,args:{section:{name:`volume-encryption`,label:`Volume encryption`,selection:`One`,emptyOptionLabel:`Unencrypted volumes`,required:!1,options:[{registration:`aws-ebs`,label:`Encrypted volumes`,isDefault:!1,disabledReason:`Needs an encryption key`,parameters:[]}]}}},p={name:`Preset cards`,args:{section:{name:`secondary-storage`,label:`Secondary storage`,selection:`One`,required:!0,options:[{registration:`elasticsearch`,label:`New Elasticsearch`,isDefault:!0,parameters:[{name:`preset`,label:`Preset`,claimEditable:!0,value:`small`}],choices:{parameter:`preset`,items:[{name:`small`,specs:[{label:`Version`,value:`8.19`},{label:`Nodes`,value:`1`},{label:`Memory`,value:`2 GiB`}]},{name:`large`,specs:[{label:`Version`,value:`8.19`},{label:`Nodes`,value:`3`},{label:`Memory`,value:`8 GiB`}]}]}},{registration:`existing-secondary-storage`,label:`Existing storage`,isDefault:!1,parameters:[]}]}}},m={name:`Parameter rules`,args:{section:{name:`team-storage`,label:`Team storage`,selection:`One`,required:!0,options:[{registration:`team-bucket`,label:`Team bucket`,description:`A bucket that the team shares.`,isDefault:!0,schema:{type:`object`,"x-kubernetes-validations":[{rule:`self.minSizeGi <= self.maxSizeGi`,message:`The minimum size must not be above the maximum size`}]},parameters:[{name:`name`,label:`Bucket name`,schema:{type:`string`,"x-kubernetes-validations":[{rule:`self.startsWith('team-')`,message:`The name must start with team-`}]},claimEditable:!0,value:`team-orders`},{name:`maxSizeGi`,label:`Maximum size in GiB`,schema:{type:`integer`,"x-kubernetes-validations":[{rule:`self % 10 == 0`}]},claimEditable:!0,value:50},{name:`minSizeGi`,label:`Minimum size in GiB`,schema:{type:`integer`},claimEditable:!0,value:10}]}]}}},h=[`DefaultAndEmptyOption`,`RequiredSection`,`AllowedCheckboxes`,`AllowedSelect`,`MapParameter`,`OptionThatNeedsAnInput`,`PresetCards`,`ParameterRules`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Default and empty option"
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Required section",
  args: {
    section: {
      ...backupStorage,
      required: true
    }
  }
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Allowed checkboxes",
  args: {
    section: connectorsPermissions
  }
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Allowed select",
  args: {
    section: {
      ...connectorsPermissions,
      options: [{
        registration: "gcp-pubsub",
        label: "Pub/Sub topics",
        isDefault: true,
        parameters: [{
          name: "access",
          label: "Topic access",
          schema: {
            type: "string"
          },
          claimEditable: true,
          allowed: [{
            name: "publish",
            label: "Publish to the team topics"
          }, {
            name: "subscribe",
            label: "Subscribe to the team topics"
          }]
        }]
      }]
    }
  }
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Map parameter",
  args: {
    section: {
      ...backupStorage,
      options: [{
        registration: "aws-s3",
        label: "New S3 bucket",
        description: "A bucket for this cluster, deleted with the claim.",
        isDefault: true,
        parameters: [{
          name: "tags",
          label: "Bucket tags",
          schema: {
            type: "object",
            additionalProperties: {
              type: "string"
            }
          },
          claimEditable: true,
          value: {
            team: "orders",
            "cost-center": "4711"
          }
        }]
      }]
    }
  }
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Option that needs an input",
  args: {
    section: {
      name: "volume-encryption",
      label: "Volume encryption",
      selection: "One",
      emptyOptionLabel: "Unencrypted volumes",
      required: false,
      options: [{
        registration: "aws-ebs",
        label: "Encrypted volumes",
        isDefault: false,
        disabledReason: "Needs an encryption key",
        parameters: []
      }]
    }
  }
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Preset cards",
  args: {
    section: {
      name: "secondary-storage",
      label: "Secondary storage",
      selection: "One",
      required: true,
      options: [{
        registration: "elasticsearch",
        label: "New Elasticsearch",
        isDefault: true,
        parameters: [{
          name: "preset",
          label: "Preset",
          claimEditable: true,
          value: "small"
        }],
        choices: {
          parameter: "preset",
          items: [{
            name: "small",
            specs: [{
              label: "Version",
              value: "8.19"
            }, {
              label: "Nodes",
              value: "1"
            }, {
              label: "Memory",
              value: "2 GiB"
            }]
          }, {
            name: "large",
            specs: [{
              label: "Version",
              value: "8.19"
            }, {
              label: "Nodes",
              value: "3"
            }, {
              label: "Memory",
              value: "8 GiB"
            }]
          }]
        }
      }, {
        registration: "existing-secondary-storage",
        label: "Existing storage",
        isDefault: false,
        parameters: []
      }]
    }
  }
}`,...p.parameters?.docs?.source}}},m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "Parameter rules",
  args: {
    section: {
      name: "team-storage",
      label: "Team storage",
      selection: "One",
      required: true,
      options: [{
        registration: "team-bucket",
        label: "Team bucket",
        description: "A bucket that the team shares.",
        isDefault: true,
        schema: {
          type: "object",
          "x-kubernetes-validations": [{
            rule: "self.minSizeGi <= self.maxSizeGi",
            message: "The minimum size must not be above the maximum size"
          }]
        },
        parameters: [{
          name: "name",
          label: "Bucket name",
          schema: {
            type: "string",
            "x-kubernetes-validations": [{
              rule: "self.startsWith('team-')",
              message: "The name must start with team-"
            }]
          },
          claimEditable: true,
          value: "team-orders"
        }, {
          name: "maxSizeGi",
          label: "Maximum size in GiB",
          schema: {
            type: "integer",
            "x-kubernetes-validations": [{
              rule: "self % 10 == 0"
            }]
          },
          claimEditable: true,
          value: 50
        }, {
          name: "minSizeGi",
          label: "Minimum size in GiB",
          schema: {
            type: "integer"
          },
          claimEditable: true,
          value: 10
        }]
      }]
    }
  }
}`,...m.parameters?.docs?.source}}}})))()}g();export{l as AllowedCheckboxes,u as AllowedSelect,s as DefaultAndEmptyOption,d as MapParameter,f as OptionThatNeedsAnInput,m as ParameterRules,p as PresetCards,c as RequiredSection,h as __namedExportsOrder,o as default};