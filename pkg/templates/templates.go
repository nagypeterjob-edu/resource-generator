package templates

var ApplicationTmpl = `{
    "name": "{{ .ServiceName }}",
    "accounts": "{{ .KubernetesAccount }}",
    "cloudProviders": "kubernetes",
    "email": "nagypeterjob@gmail.com"
}`

var PipelineTmpl = `{
    "appConfig": {},
    "limitConcurrent": true,
    "schema": "v2",
    "template": {
      "artifactAccount": "front50ArtifactCredentials",
      "reference": "spinnaker://{{ .Spinnaker.TemplateId }}",
      "type": "front50/pipelineTemplate"
    },
    "type": "templatedPipeline",
    "application": "{{ .ServiceName }}",
    "name": "{{ .Namespace }}",
    "parameters": { {{ range $k, $v := .Spinnaker.Chart.Parameters }}
        "{{ $k }}": "{{ $v }}",{{ end }}
        "serviceName": "{{ .ServiceName }}",
        "namespace": "{{ .Namespace }}",
        "triggerRegexp": "{{ .Spinnaker.TriggerRegexp }}"
    }
  }`
