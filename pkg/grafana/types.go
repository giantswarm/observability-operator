package grafana

type Organization struct {
	ID   int64
	Name string
}

type Datasource struct {
	ID        int64
	Name      string
	IsDefault bool
	Type      string
	URL       string
	Access    string
	JSONData  map[string]interface{}
}
