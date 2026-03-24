package transfer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fulmenhq/dimlox/internal/uri"
)

type copyFileListEntry struct {
	Source      string `json:"src"`
	Destination string `json:"dst"`
}

func buildCopyPlanFromFile(filePath string) (*CopyPlan, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	items := make([]CopyPlanItem, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		var entry copyFileListEntry
		if err := unmarshalJSONLine([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse %s line %d: %w", filePath, lineNo, err)
		}
		if strings.TrimSpace(entry.Source) == "" || strings.TrimSpace(entry.Destination) == "" {
			return nil, fmt.Errorf("parse %s line %d: src and dst are required", filePath, lineNo)
		}
		if _, err := uri.Parse(entry.Source); err != nil {
			return nil, fmt.Errorf("parse %s line %d src: %w", filePath, lineNo, err)
		}
		if _, err := uri.Parse(entry.Destination); err != nil {
			return nil, fmt.Errorf("parse %s line %d dst: %w", filePath, lineNo, err)
		}
		items = append(items, CopyPlanItem(entry))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%s contained no transfer entries", filePath)
	}
	plan := &CopyPlan{Items: items}
	if err := validateCopyPlan(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func unmarshalJSONLine(data []byte, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}
