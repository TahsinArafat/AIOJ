package fps

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/tahsinarafat/aioj/internal/model"
)

// ParseXML parses raw FPS XML bytes into model.Problem slice.
func ParseXML(data []byte) ([]*model.Problem, error) {
	var raw FPS
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal FPS XML: %w", err)
	}

	var problems []*model.Problem
	for _, p := range raw.Problems {
		prob := &model.Problem{
			Title:       p.Title,
			Description: p.Description,
			InputFormat: p.Input,
			OutputFormat: p.Output,
			Hint:        p.Hint,
			TimeLimit:   p.TimeLimit,
			MemoryLimit: p.MemoryLimit * 1024, // MB to KB
			Source:      p.Source,
			Visible:     true,
		}

		// Map tags (comma-separated list to string array)
		if p.Tags != "" {
			parts := strings.Split(p.Tags, ",")
			for _, tag := range parts {
				trimmed := strings.TrimSpace(tag)
				if trimmed != "" {
					prob.Tags = append(prob.Tags, trimmed)
				}
			}
		}

		// Map sample cases
		minSamples := len(p.SampleInput)
		if len(p.SampleOutput) < minSamples {
			minSamples = len(p.SampleOutput)
		}
		for i := 0; i < minSamples; i++ {
			prob.SampleCases = append(prob.SampleCases, model.SampleCase{
				Input:  strings.TrimSpace(p.SampleInput[i]),
				Output: strings.TrimSpace(p.SampleOutput[i]),
			})
		}

		// Map SPJ
		if p.SPJ != nil && p.SPJ.SourceCode != "" {
			prob.SPJ = true
			prob.SPJLanguage = p.SPJ.Language
			prob.SPJSourceCode = p.SPJ.SourceCode
		}

		problems = append(problems, prob)
	}

	return problems, nil
}
