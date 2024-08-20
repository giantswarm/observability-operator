package alloy

import (
	_ "embed"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

var (
	//go:embed templates/alloy-config.alloy.template
	alloyConfig string

	//go:embed templates/monitoring-config.yaml.template
	alloyMonitoringConfig string
)

func init() {
	template.Must(template.New("alloy-config.alloy").Funcs(sprig.FuncMap()).Parse(alloyConfig))
	template.Must(template.New("monitoring-config.yaml").Funcs(sprig.FuncMap()).Parse(alloyMonitoringConfig))
}
