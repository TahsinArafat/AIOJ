package fps

import "encoding/xml"

// FPS represents the root Free Problem Set element.
type FPS struct {
	XMLName  xml.Name  `xml:"fps"`
	Version  string    `xml:"version,attr"`
	Problems []Problem `xml:"problem"`
}

// Problem represents a single problem in FPS format.
type Problem struct {
	Title        string   `xml:"title"`
	TimeLimit    int      `xml:"time_limit"`   // in ms
	MemoryLimit  int      `xml:"memory_limit"` // in MB
	Description  string   `xml:"description"`
	Input        string   `xml:"input"`
	Output       string   `xml:"output"`
	SampleInput  []string `xml:"sample_input"`
	SampleOutput []string `xml:"sample_output"`
	TestInput    []string `xml:"test_input"`
	TestOutput   []string `xml:"test_output"`
	Hint         string   `xml:"hint"`
	Source       string   `xml:"source"`
	SPJ          *SPJ     `xml:"spj"`
	Tags         string   `xml:"tags"` // comma-separated
}

// SPJ represents a special judge program inside the FPS element.
type SPJ struct {
	Language   string `xml:"language,attr"`
	SourceCode string `xml:",cdata"`
}
