package fps

import (
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

func TestParseXML_SingleProblem(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<fps version="1.2">
  <problem>
    <title>A Plus B</title>
    <time_limit>1000</time_limit>
    <memory_limit>256</memory_limit>
    <description>Find A+B</description>
    <input>Two integers</input>
    <output>Their sum</output>
    <sample_input>1 2</sample_input>
    <sample_output>3</sample_output>
    <test_input>5 5</test_input>
    <test_output>10</test_output>
    <hint>Simple arithmetic</hint>
    <source>Codeforces 1A</source>
    <tags>math,basic</tags>
    <spj language="cpp"><![CDATA[#include <iostream>]]> </spj>
  </problem>
</fps>`)

	problems, err := ParseXML(xmlData)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(problems) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(problems))
	}

	p := problems[0]
	if p.Title != "A Plus B" {
		t.Errorf("expected title 'A Plus B', got %q", p.Title)
	}
	if p.TimeLimit != 1000 {
		t.Errorf("expected time limit 1000, got %d", p.TimeLimit)
	}
	if p.MemoryLimit != 256*1024 {
		t.Errorf("expected memory limit 262144, got %d", p.MemoryLimit)
	}
	if p.Description != "Find A+B" {
		t.Errorf("expected description 'Find A+B', got %q", p.Description)
	}
	if p.InputFormat != "Two integers" {
		t.Errorf("expected input format 'Two integers', got %q", p.InputFormat)
	}
	if p.OutputFormat != "Their sum" {
		t.Errorf("expected output format 'Their sum', got %q", p.OutputFormat)
	}
	if p.Hint != "Simple arithmetic" {
		t.Errorf("expected hint 'Simple arithmetic', got %q", p.Hint)
	}
	if p.Source != "Codeforces 1A" {
		t.Errorf("expected source 'Codeforces 1A', got %q", p.Source)
	}
	if !p.Visible {
		t.Errorf("expected visible=true")
	}
	if p.Difficulty != "easy" {
		t.Errorf("expected difficulty 'easy', got %q", p.Difficulty)
	}

	// Tags
	if len(p.Tags) != 2 || p.Tags[0] != "math" || p.Tags[1] != "basic" {
		t.Errorf("unexpected tags: %v", p.Tags)
	}

	// Sample cases
	if len(p.SampleCases) != 1 {
		t.Fatalf("expected 1 sample case, got %d", len(p.SampleCases))
	}
	if p.SampleCases[0].Input != "1 2" {
		t.Errorf("expected sample input '1 2', got %q", p.SampleCases[0].Input)
	}
	if p.SampleCases[0].Output != "3" {
		t.Errorf("expected sample output '3', got %q", p.SampleCases[0].Output)
	}

	// SPJ
	if !p.SPJ || p.SPJLanguage != "cpp" || !strings.Contains(p.SPJSourceCode, "#include") {
		t.Errorf("SPJ mapping failed: spj=%v, lang=%s, code=%q", p.SPJ, p.SPJLanguage, p.SPJSourceCode)
	}
}

func TestParseXML_MultipleProblems(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<fps version="1.2">
  <problem>
    <title>Problem A</title>
    <time_limit>1000</time_limit>
    <memory_limit>128</memory_limit>
    <description>Desc A</description>
    <input>Input A</input>
    <output>Output A</output>
  </problem>
  <problem>
    <title>Problem B</title>
    <time_limit>2000</time_limit>
    <memory_limit>512</memory_limit>
    <description>Desc B</description>
    <input>Input B</input>
    <output>Output B</output>
  </problem>
</fps>`)

	problems, err := ParseXML(xmlData)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(problems) != 2 {
		t.Fatalf("expected 2 problems, got %d", len(problems))
	}

	if problems[0].Title != "Problem A" {
		t.Errorf("expected title 'Problem A', got %q", problems[0].Title)
	}
	if problems[1].Title != "Problem B" {
		t.Errorf("expected title 'Problem B', got %q", problems[1].Title)
	}
	if problems[0].MemoryLimit != 128*1024 {
		t.Errorf("expected memory limit 131072, got %d", problems[0].MemoryLimit)
	}
	if problems[1].MemoryLimit != 512*1024 {
		t.Errorf("expected memory limit 524288, got %d", problems[1].MemoryLimit)
	}
}

func TestParseXML_NoSPJ(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<fps version="1.2">
  <problem>
    <title>No SPJ Problem</title>
    <time_limit>1000</time_limit>
    <memory_limit>256</memory_limit>
    <description>Desc</description>
    <input>In</input>
    <output>Out</output>
  </problem>
</fps>`)

	problems, err := ParseXML(xmlData)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if problems[0].SPJ {
		t.Errorf("expected SPJ=false when no spj element present")
	}
}

func TestParseXML_EmptyTags(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<fps version="1.2">
  <problem>
    <title>No Tags</title>
    <time_limit>1000</time_limit>
    <memory_limit>256</memory_limit>
    <description>Desc</description>
    <input>In</input>
    <output>Out</output>
    <tags></tags>
  </problem>
</fps>`)

	problems, err := ParseXML(xmlData)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(problems[0].Tags) != 0 {
		t.Errorf("expected no tags, got %v", problems[0].Tags)
	}
}

func TestParseXML_InvalidXML(t *testing.T) {
	xmlData := []byte(`not valid xml`)

	_, err := ParseXML(xmlData)
	if err == nil {
		t.Fatalf("expected error for invalid XML, got nil")
	}
}

func TestParseXML_MultipleSampleCases(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<fps version="1.2">
  <problem>
    <title>Multi Samples</title>
    <time_limit>1000</time_limit>
    <memory_limit>256</memory_limit>
    <description>Desc</description>
    <input>In</input>
    <output>Out</output>
    <sample_input>1 2</sample_input>
    <sample_input>3 4</sample_input>
    <sample_output>3</sample_output>
    <sample_output>7</sample_output>
  </problem>
</fps>`)

	problems, err := ParseXML(xmlData)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(problems[0].SampleCases) != 2 {
		t.Fatalf("expected 2 sample cases, got %d", len(problems[0].SampleCases))
	}
	if problems[0].SampleCases[1].Input != "3 4" {
		t.Errorf("expected second sample input '3 4', got %q", problems[0].SampleCases[1].Input)
	}
	if problems[0].SampleCases[1].Output != "7" {
		t.Errorf("expected second sample output '7', got %q", problems[0].SampleCases[1].Output)
	}
}

func TestGenerateXML(t *testing.T) {
	p := &model.Problem{
		ID:           "test-id",
		Title:        "Sum Problem",
		TimeLimit:    2000,
		MemoryLimit:  262144, // 256MB in KB
		Description:  "Given two numbers",
		InputFormat:  "Two integers on one line",
		OutputFormat: "Their sum",
		Tags:         []string{"math", "easy"},
		SampleCases: []model.SampleCase{
			{Input: "1 2", Output: "3"},
		},
	}

	xmlBytes, err := GenerateXML([]*model.Problem{p})
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	xml := string(xmlBytes)
	if !strings.Contains(xml, "<title>Sum Problem</title>") {
		t.Errorf("title missing in XML:\n%s", xml)
	}
	if !strings.Contains(xml, "<memory_limit>256</memory_limit>") {
		t.Errorf("memory limit not converted from KB to MB:\n%s", xml)
	}
	if !strings.Contains(xml, "<tags>math,easy</tags>") {
		t.Errorf("tags missing in XML:\n%s", xml)
	}
	if !strings.Contains(xml, "1 2") {
		t.Errorf("sample input missing:\n%s", xml)
	}
}
