//go:build !debug

package userdata

import "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/paths"

var WriteFilesList = []string{paths.AACfgPath, paths.CDHCfgPath, paths.ForwarderCfgPath, paths.AuthFilePath, paths.InitDataPath}
var InitdDataFilesList = []string{paths.AACfgPath, paths.CDHCfgPath, PolicyPath}
