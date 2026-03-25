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

package middleware

import (
	"api-service/metrics"
	"api-service/service"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	backendmodels "envhub/models"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// experimentRequest is a minimal struct to extract labels from the request body.
type experimentRequest struct {
	Labels map[string]string `json:"labels"`
}

// createResponse is a minimal struct to extract instance ID from the response.
type createResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ID string `json:"id"`
	} `json:"data"`
}

// ExperimentAdmissionMiddleware checks whether a new experiment should be admitted
// based on cluster resource availability, watermark, and required labels.
//
// Tier logic:
//   - P2 (unlabeled): missing required labels → reject 400
//   - P0 (known): existing experiment → always allow
//   - P1 (new): new experiment → watermark + capacity gate
//
// After successful creation, registers the instance → experiment mapping
// so the periodic task can correctly track per-experiment counts even when
// FaaS backend doesn't return user labels.
func ExperimentAdmissionMiddleware(admission *service.ExperimentAdmission) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Feature gate: if admission is nil (disabled), pass through
		if admission == nil {
			c.Next()
			return
		}

		labels := extractLabelsFromRequest(c)

		// P2 gate: check required labels
		missing := admission.CheckRequiredLabels(labels)
		if len(missing) > 0 {
			reason := fmt.Sprintf("Experiment admission denied: missing required labels [%s]", strings.Join(missing, ", "))
			metrics.InstanceOpsTotal.WithLabelValues("create", "", "admission_rejected").Inc()
			metrics.ExperimentAdmissionTotal.WithLabelValues("rejected", "p2_unlabeled").Inc()
			backendmodels.JSONErrorWithMessage(c, 400, reason)
			c.Abort()
			return
		}

		experiment := labels["experiment"]
		result := admission.ShouldAdmitWithResult(experiment)

		if !result.Allowed {
			metrics.InstanceOpsTotal.WithLabelValues("create", "", "admission_rejected").Inc()
			metrics.ExperimentAdmissionTotal.WithLabelValues("rejected", result.Tier).Inc()
			backendmodels.JSONErrorWithMessage(c, 429, result.Reason)
			c.Abort()
			return
		}

		metrics.ExperimentAdmissionTotal.WithLabelValues("allowed", result.Tier).Inc()

		// Wrap response writer to capture the response body
		rw := &responseCapture{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = rw

		c.Next()

		// After handler: if creation succeeded, register instance → experiment
		if c.Writer.Status() == 200 {
			var resp createResponse
			if err := json.Unmarshal(rw.body.Bytes(), &resp); err == nil && resp.Success && resp.Data.ID != "" {
				admission.RegisterInstance(resp.Data.ID, experiment)
				log.Infof("Experiment admission: registered instance %s → experiment %q", resp.Data.ID, experiment)
			}
		}
	}
}

// responseCapture wraps gin.ResponseWriter to capture the response body.
type responseCapture struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	rc.body.Write(b)
	return rc.ResponseWriter.Write(b)
}

// extractLabelsFromRequest peeks at the request body to extract labels
// without consuming the body, so downstream handlers can still read it.
func extractLabelsFromRequest(c *gin.Context) map[string]string {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil
	}
	// Restore the body for downstream handlers
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var req experimentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}

	return req.Labels
}
