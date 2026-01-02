package config

type Config struct {
	Version   int    `json:"version"`
	ProjectID string `json:"project_id"`
	Region    string `json:"region"`
}