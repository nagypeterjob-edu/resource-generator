# application-values
Application overrides, spinnaker pipeline deployer

**gen** will calculate md5 hash for existing <values>.yml files & generate new <values>.yamls based on provided values. If there were no change in the contents, **gen** won't generate `application.json` and `pipeline.json` for given yaml. Thus `make generate` will only update applications and pipelines in Spinnaker which has really changed.

## Mandatory directory structure

```bash
root/
├── templates/
│   ├── application.json
│   └── pipeline.json
└── resources (can define other name by flag)
    ├── namespace1/
    │   ├── global.yml
    │   ├── service1.yml
    │   .
    │   └── serviceN.yml
    .
    └── namespaeN/
        ├── global.yml
        ├── service1.yml
        .
        └── serviceM.yml
```

## Flags & Usage
`-destination` : Generated files will end up in this directory
`-templates` : Template files location (default "templates")
`-values` : Values files location

### Example 
```bash
gen -destination "generated" -templates "templates" -values "resources"
```

TODO