package alloy

type MonitoringConfig struct {
	Alloy MonitoringConfigAlloy `json:"alloy"`
}

type MonitoringConfigAlloy struct {
	Alloy MonitoringConfigAlloyAlloy `json:"alloy"`
}

type MonitoringConfigAlloyAlloy struct {
	Controller MonitoringConfigAlloyAlloyController `json:"controller"`
}

type MonitoringConfigAlloyAlloyController struct {
	Replicas int `json:"replicas"`
}
