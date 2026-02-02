package metrics

type monitoringConfig struct {
	Alloy monitoringConfigAlloy `json:"alloy"`
}

type monitoringConfigAlloy struct {
	Controller monitoringConfigAlloyController `json:"controller"`
}

type monitoringConfigAlloyController struct {
	Replicas int `json:"replicas"`
}
