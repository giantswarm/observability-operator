package alloy

type MonitoringConfig struct {
	Alloy MonitoringConfigAlloy `json:"alloy"`
}

type MonitoringConfigAlloy struct {
	Controller MonitoringConfigAlloyController `json:"controller"`
}

type MonitoringConfigAlloyController struct {
	Replicas int `json:"replicas"`
}
