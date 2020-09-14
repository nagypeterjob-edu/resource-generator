# application-values
Application overrides, Spinnaker pipeline deployer

**gen** generates Helm values files, Spinnaker pipeline and application templates for input resources.
When `--bucket` flag is defined, **gen** looks for previously generated resources in remote location and compares it to the local hash. If the hashes are identical, there is no need to re-generate & re-upload the helm resource. Spinnaker application & pipeline templates will always be updated.

## Expected directory structure

```bash
root/
│
└── resources (can define other name by `--values` flag)
    ├── namespace1/
    │   ├── global.yml
    │   ├── service1.yml
    │   .
    │   └── serviceN.yml
    .
    └── namespaceN/
        ├── global.yml
        ├── service1.yml
        .
        └── serviceM.yml
```

## Flags & Usage
`--destination` : Generated files will end up in this directory. If missing, **gen** will generate the following output directories:
```bash
<destination>/ (directory defined by `--destination` flag)
│
└── resources/ (stores generated Helm values)
│    
└── applications/ (stores generated Spinnaker application templates)
│
└── pipelines/ (stores generated Spinnaker pipeline templates)
```

**--values** : Initial values files location  
**--bucket** : S3 bucket name. When `--bucket` flag is defined, **gen** looks for previously generated resources in remote location and compares it to the local hash. If the hashes are identical, there is no need to re-generated & re-upload the helm values file.  
**--region** : Region of the S3 bucket. `default:` **us-east-1**  
**--prefix** : S3 key prefix. Helm values should be uploaded to this directory inside the bucket. `default:` **values**  
**--workers** : Number of goroutines used for resource generation. If x <= 0, **gen** uses all available CPU cores. `default:` **-1** 

```
Please note that when --bucket flag is set, the application requires access to the bucket defined
```


### Example 
```bash
gen --destination "generated" --values "resources" --bucket "helm-charts" --region "us-east-1" --workers 4
```
