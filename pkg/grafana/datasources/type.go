package datasources

type Datasource struct {
	ID             int64
	UID            string
	Name           string
	Type           string
	URL            string
	IsDefault      bool
	JSONData       map[string]any
	SecureJSONData map[string]string
	Access         string
}

func (d *Datasource) SetJSONData(key string, value any) {
	if d.JSONData == nil {
		d.JSONData = make(map[string]any)
	}

	d.JSONData[key] = value
}

func (d *Datasource) SetSecureJSONData(key, value string) {
	if d.SecureJSONData == nil {
		d.SecureJSONData = make(map[string]string)
	}

	d.SecureJSONData[key] = value
}
