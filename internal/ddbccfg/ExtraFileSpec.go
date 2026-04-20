package ddbccfg

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ExtraFileSpec struct {
	SrcPath  string
	DestName string
}

// ParseExtraFileSpec parses strings in the format of "src[:dest]".
func ParseExtraFileSpec(raw string) (ExtraFileSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ExtraFileSpec{}, fmt.Errorf("empty raw string")
	}

	parts := strings.SplitN(raw, ":", 2)
	srcPath := strings.TrimSpace(parts[0])
	if srcPath == "" {
		return ExtraFileSpec{}, fmt.Errorf("source path is required")
	}

	destName := ""
	if len(parts) > 1 {
		destName = strings.TrimSpace(parts[1])
	}
	if destName == "" {
		destName = filepath.Base(srcPath)
	}

	return ExtraFileSpec{
		SrcPath:  srcPath,
		DestName: destName,
	}, nil
}
