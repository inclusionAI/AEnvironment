package controller

import "strings"

const defaultDatasourceImagePrefix = "docker.io/library/aenv"

func datasourceImageName(deployConfig map[string]interface{}, datasource string) string {
	if isCompleteImageReference(datasource) {
		return datasource
	}

	imagePrefix := defaultDatasourceImagePrefix
	if value, ok := deployConfig["imagePrefix"]; ok {
		if str, ok := value.(string); ok {
			imagePrefix = str
		}
	}
	return imagePrefix + ":" + datasource
}

func isCompleteImageReference(value string) bool {
	if strings.Contains(value, "@") {
		return true
	}
	if strings.Contains(value, "/") {
		return true
	}
	return strings.Contains(value, ":")
}
