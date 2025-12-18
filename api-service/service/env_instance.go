package service

import (
	"api-service/models"
	backend "envhub/models"
)

// EnvInstanceService defines sandbox crud interfaces
type EnvInstanceService interface {
	GetEnvInstance(id string) (*models.EnvInstance, error)
	CreateEnvInstance(req *backend.Env) (*models.EnvInstance, error)
	DeleteEnvInstance(id string) error
	ListEnvInstances(envName string) ([]*models.EnvInstance, error)
	Warmup(req *backend.Env) error
	Cleanup() error
}
