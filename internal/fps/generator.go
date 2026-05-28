package fps

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/tahsinarafat/aioj/internal/model"
)

// GenerateXML generates FPS XML bytes from a slice of model.Problem.
func GenerateXML(problems []*model.Problem) ([]byte, error) {
	fps := FPS{Version: "1.2"}

	for _, p := range problems {
		prob := Problem{
			Title:       p.Title,
			TimeLimit:   p.TimeLimit,
			MemoryLimit: p.MemoryLimit / 1024, // KB to MB
			Description: p.Description,
			Input:       p.InputFormat,
			Output:      p.OutputFormat,
			Hint:        p.Hint,
			Source:      p.Source,
			Tags:        strings.Join(p.Tags, ","),
		}

		for _, s := range p.SampleCases {
			prob.SampleInput = append(prob.SampleInput, s.Input)
			prob.SampleOutput = append(prob.SampleOutput, s.Output)
		}

		if p.SPJ && p.SPJSourceCode != "" {
			prob.SPJ = &SPJ{
				Language:   p.SPJLanguage,
				SourceCode: p.SPJSourceCode,
			}
		}

		fps.Problems = append(fps.Problems, prob)
	}

	out, err := xml.MarshalIndent(fps, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal FPS XML: %w", err)
	}

	return append([]byte(xml.Header), out...), nil
}
