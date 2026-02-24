package handler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sashko-guz/mage/internal/operations"
)

// formatOperations returns a human-readable summary of the operation pipeline for debug logging.
func formatOperations(ops []operations.Operation, filterString string) string {
	if len(ops) == 0 {
		return "none"
	}

	hasFit := containsFitOperation(ops)
	hasExplicitQuality := strings.Contains(filterString, "quality(")
	defaultQuality := findQualityValue(ops)

	var parts []string
	for _, op := range ops {
		if s, ok := formatOperation(op, hasFit, hasExplicitQuality, defaultQuality); ok {
			parts = append(parts, s)
		}
	}

	return strings.Join(parts, " → ")
}

func containsFitOperation(ops []operations.Operation) bool {
	for _, op := range ops {
		if _, ok := op.(*operations.FitOperation); ok {
			return true
		}
	}
	return false
}

func findQualityValue(ops []operations.Operation) int {
	for _, op := range ops {
		if q, ok := op.(*operations.QualityOperation); ok {
			return q.Quality
		}
	}
	return 0
}

// formatOperation formats a single operation for logging.
// Returns ("", false) when the operation should be omitted from the output.
func formatOperation(op operations.Operation, hasFit, hasExplicitQuality bool, defaultQuality int) (string, bool) {
	switch v := op.(type) {
	case *operations.ResizeOperation:
		return formatResizeOperation(v, hasFit, hasExplicitQuality, defaultQuality), true
	case *operations.FormatOperation:
		return fmt.Sprintf("format(%s)", v.Format), true
	case *operations.QualityOperation:
		if hasExplicitQuality {
			return fmt.Sprintf("quality(%d)", v.Quality), true
		}
		return "", false // default quality is shown inline with resize
	case *operations.CropOperation:
		return fmt.Sprintf("crop(%d,%d,%d,%d)", v.X1, v.Y1, v.X2, v.Y2), true
	case *operations.PercentCropOperation:
		return fmt.Sprintf("pcrop(%d,%d,%d,%d)", v.X1, v.Y1, v.X2, v.Y2), true
	case *operations.FitOperation:
		return formatFitOperation(v), true
	default:
		return op.Name(), true
	}
}

func formatResizeOperation(v *operations.ResizeOperation, hasFit, hasExplicitQuality bool, defaultQuality int) string {
	widthStr, heightStr := "auto", "auto"
	if v.Width != nil {
		widthStr = strconv.Itoa(*v.Width)
	}
	if v.Height != nil {
		heightStr = strconv.Itoa(*v.Height)
	}

	s := fmt.Sprintf("resize(%sx%s", widthStr, heightStr)
	if !hasFit {
		s += fmt.Sprintf(", fit=%s", v.Fit)
	}
	if !hasExplicitQuality {
		s += fmt.Sprintf(", quality(%d)", defaultQuality)
	}
	return s + ")"
}

func formatFitOperation(v *operations.FitOperation) string {
	if v.FillColor != "" && v.Mode == "fill" {
		return fmt.Sprintf("fit(%s, %s)", v.Mode, v.FillColor)
	}
	return fmt.Sprintf("fit(%s)", v.Mode)
}
