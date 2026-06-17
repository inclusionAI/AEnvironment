package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"envhub/models"
)

func (ctrl *EnvController) ListEnvs(c *gin.Context) {
	keys, err := ctrl.storage.List(c.Request.Context(), nil)
	if err != nil {
		models.JSONErrorWithMessage(c, http.StatusInternalServerError, err.Error())
		return
	}

	envs := make([]interface{}, 0, len(keys))
	for _, key := range keys {
		env, _, err := ctrl.storage.Get(c.Request.Context(), key)
		if err != nil {
			continue
		}
		envs = append(envs, env)
	}

	models.JSONSuccess(c, envs)
}
