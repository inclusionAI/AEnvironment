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
	"errors"
	"log"
	"os/exec"
)

type ACIHackClient struct{}

func (cli *ACIHackClient) trigger(name string, version string, templateId string, callbackURL string) error {
	cmd := exec.Command("aci_hack", name, version, "--template", templateId, "--callback-url", callbackURL)
	// Get output
	output, err := cmd.Output()
	if err != nil {
		return errors.New("trigger aci pipeline failed:" + err.Error())
	}
	log.Println(string(output))
	return nil
}

var aci = &ACIHackClient{}

func Trigger(name string, version string, templateId string, callbackURL string) error {
	return aci.trigger(name, version, templateId, callbackURL)
}
