/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"envhub/models"
	"envhub/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type DatasourceController struct {
	service service.DatesetService
}

func NewDatasourceController() *DatasourceController {
	return &DatasourceController{
		service: service.NewCompositeDataService(),
	}
}

type DataSearchRequest struct {
	Scenario string `json:"scenario"`
	Instance string `json:"instance"`
}

func (ctrl *DatasourceController) RegisterDataRoutes(r *gin.Engine) {
	dataGroup := r.Group("/data")
	{
		dataGroup.GET("", ctrl.DataLoad)
	}
}

func (ctrl *DatasourceController) DataLoad(c *gin.Context) {
	scenario := c.Query("scenario")
	instance := c.Query("instance")
	if scenario == "" || instance == "" {
		models.JSONErrorWithMessage(c, http.StatusBadRequest, "Invalid scenario or instance")
		return
	}
	data := ctrl.service.Load(context.TODO(), service.DataSearch{
		Scenario: scenario,
		Key:      instance,
	})
	models.JSONSuccess(c, data)
}
