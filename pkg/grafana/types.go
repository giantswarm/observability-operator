package grafana

type Organization struct {
	ID   int64
	Name string
}

type Datasource struct {
	ID     int64
	Name   string
	Type   string
	URL    string
	Access string
}
