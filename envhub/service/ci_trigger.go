package service

import "envhub/models"

type CITrigger interface {
	Trigger(env *models.Env)
}
