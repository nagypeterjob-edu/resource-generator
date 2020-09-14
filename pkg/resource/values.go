package resource

// Add all variables you use in your Helm values

type Values struct {
	ReplicaCount         int                    `mapstructure:"replicaCount"`
	EnvironmentVariables map[string]interface{} `mapstructure:"environmentVariables"`
}

// Add Spinnaker related variables

type Spinnaker struct {
	TemplateId    string `mapstructure:"templateId"`
	TriggerRegexp string `mapstructure:"triggerRegexp"`
	Chart         struct {
		Name       string            `mapstructure:"name"`
		Parameters map[string]string `mapstructure:"parameters"`
	} `mapstructure:"chart"`
}

type SpinnakerConfig struct {
	Spinnaker `mapstructure:"spinnaker"`
}
