package helm

import (
	"fmt"
)

// ValuesFileNotFoundError is returned when a provided values file input is not found on the host path.
type ValuesFileNotFoundError struct {
	Path string
}

func (err ValuesFileNotFoundError) Error() string {
	return fmt.Sprintf("Could not resolve values file %s", err.Path)
}

// SetFileNotFoundError is returned when a provided set file input is not found on the host path.
type SetFileNotFoundError struct {
	Path string
}

func (err SetFileNotFoundError) Error() string {
	return fmt.Sprintf("Could not resolve set file path %s", err.Path)
}

// TemplateFileNotFoundError is returned when a provided template file input is not found in the chart
type TemplateFileNotFoundError struct {
	Path     string
	ChartDir string
}

func (err TemplateFileNotFoundError) Error() string {
	return fmt.Sprintf("Could not resolve template file %s relative to chart path %s", err.Path, err.ChartDir)
}

// ChartNotFoundError is returned when a provided chart dir is not found
type ChartNotFoundError struct {
	Path string
}

func (err ChartNotFoundError) Error() string {
	return fmt.Sprintf("Could not chart path %s", err.Path)
}
