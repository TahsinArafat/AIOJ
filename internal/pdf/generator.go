package pdf

import (
	"bytes"
	"fmt"

	"github.com/jung-kurt/gofpdf"
	"github.com/tahsinarafat/aioj/internal/model"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) GenerateContestPDF(contest *model.Contest, problems []model.ProblemWithSamples) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 20)

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 24)
	pdf.Cell(0, 20, contest.Title)
	pdf.Ln(15)

	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(0, 10, fmt.Sprintf("Duration: %s", contest.EndTime.Sub(contest.StartTime)))
	pdf.Ln(8)
	pdf.Cell(0, 10, fmt.Sprintf("Problems: %d", len(problems)))
	pdf.Ln(15)

	pdf.SetFont("Helvetica", "B", 16)
	pdf.Cell(0, 10, "Problems")
	pdf.Ln(10)

	for i, p := range problems {
		pdf.SetFont("Helvetica", "", 12)
		pdf.Cell(0, 8, fmt.Sprintf("%c. %s", 'A'+i, p.Title))
		pdf.Ln(6)
	}

	for i, p := range problems {
		pdf.AddPage()

		pdf.SetFont("Helvetica", "B", 18)
		pdf.Cell(0, 15, fmt.Sprintf("Problem %c: %s", 'A'+i, p.Title))
		pdf.Ln(12)

		pdf.SetFont("Helvetica", "", 10)
		pdf.Cell(0, 6, fmt.Sprintf("Time Limit: %d ms | Memory Limit: %d MB", p.TimeLimit, p.MemoryLimit/1024))
		pdf.Ln(10)

		pdf.SetFont("Helvetica", "", 11)
		pdf.MultiCell(0, 6, p.Description, "", "", false)
		pdf.Ln(8)

		if p.InputFormat != "" {
			pdf.SetFont("Helvetica", "B", 12)
			pdf.Cell(0, 8, "Input")
			pdf.Ln(6)
			pdf.SetFont("Helvetica", "", 11)
			pdf.MultiCell(0, 6, p.InputFormat, "", "", false)
			pdf.Ln(6)
		}

		if p.OutputFormat != "" {
			pdf.SetFont("Helvetica", "B", 12)
			pdf.Cell(0, 8, "Output")
			pdf.Ln(6)
			pdf.SetFont("Helvetica", "", 11)
			pdf.MultiCell(0, 6, p.OutputFormat, "", "", false)
			pdf.Ln(6)
		}

		for j, sample := range p.SampleCases {
			pdf.SetFont("Helvetica", "B", 12)
			pdf.Cell(0, 8, fmt.Sprintf("Sample Input %d", j+1))
			pdf.Ln(6)
			pdf.SetFont("Courier", "", 10)
			pdf.MultiCell(0, 5, sample.Input, "1", "", false)
			pdf.Ln(4)

			pdf.SetFont("Helvetica", "B", 12)
			pdf.Cell(0, 8, fmt.Sprintf("Sample Output %d", j+1))
			pdf.Ln(6)
			pdf.SetFont("Courier", "", 10)
			pdf.MultiCell(0, 5, sample.Output, "1", "", false)
			pdf.Ln(4)

			if sample.Explanation != "" {
				pdf.SetFont("Helvetica", "I", 10)
				pdf.MultiCell(0, 5, sample.Explanation, "", "", false)
				pdf.Ln(6)
			}
		}

		if p.Hint != "" {
			pdf.SetFont("Helvetica", "B", 12)
			pdf.Cell(0, 8, "Hint")
			pdf.Ln(6)
			pdf.SetFont("Helvetica", "", 11)
			pdf.MultiCell(0, 6, p.Hint, "", "", false)
		}
	}

	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("generate pdf: %w", err)
	}

	return buf.Bytes(), nil
}
