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

package clients

import (
	"envhub/models"
	"log"
)

type ACITrigger struct {
	TemplateId  string
	CallbackURL string
}

func (c ACITrigger) Trigger(env *models.Env) {
	ACIHook(env, c.TemplateId, c.CallbackURL)
}

func ACIHook(env *models.Env, templateId string, callbackURL string) {
	// Try to trigger pipeline, use hack logic first (crying)
	artis := env.Artifacts
	buildConfig := env.BuildConfig
	policy := "IfNotPresent"
	if buildConfig != nil {
		if val, exist := buildConfig["build_policy"]; exist {
			policy = val.(string)
		}
	}

	for _, a := range artis {
		if a.Type == "image" && policy == "IfNotPresent" {
			log.Printf("No need rebuild image if existed")
			return
		}
	}
	err := Trigger(env.Name, env.Version, templateId, callbackURL)
	if err != nil {
		log.Printf("Trigger aenv aci image build err:%v", err)
	}
}
