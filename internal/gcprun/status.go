package gcprun

type Condition struct {
	Type  string `json:"type"`
	State string `json:"state"`
}

func DeriveStatus(conds []Condition) string {
	var routesOK, cfgOK bool

	for _, c := range conds {
		if c.State == "CONDITION_FAILED" || c.State == "FAILED" {
			return "ERROR"
		}

		switch c.Type {
		case "RoutesReady":
			if c.State == "CONDITION_SUCCEEDED" {
				routesOK = true
			}
		case "ConfigurationsReady":
			if c.State == "CONDITION_SUCCEEDED" {
				cfgOK = true
			}
		}
	}

	if routesOK && cfgOK {
		return "READY"
	}
	return "UNKNOWN"
}