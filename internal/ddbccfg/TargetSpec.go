package ddbccfg

import "strings"

type TargetSpec struct {
	Filename string
	Os       string
	Arch     string
	Params   []string
}

func (targetSpec *TargetSpec) GetArtifactName() string {
	if targetSpec.Os != "windows" {
		return targetSpec.Filename
	}

	if strings.HasSuffix(strings.ToLower(targetSpec.Filename), ".exe") {
		return targetSpec.Filename
	}

	return targetSpec.Filename + ".exe"
}
