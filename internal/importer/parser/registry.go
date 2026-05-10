package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	xlslib "github.com/extrame/xls"
)

// Registry holds all registered parsers and selects one based on header auto-detection.
// New format versions add a new parser — old ones are never removed, keeping backwards compatibility.
type Registry struct {
	parsers []Parser
}

// NewRegistry returns a Registry pre-loaded with all known bank parsers.
func NewRegistry() *Registry {
	r := &Registry{}
	r.Register(&AxisV1{})
	r.Register(&IDFCV1{})
	r.Register(&SBIV1{})
	r.Register(&ICICIV1{})
	r.Register(&HDFCV1{})
	return r
}

// Register adds a parser to the registry. Later registrations take priority in Detect.
func (r *Registry) Register(p Parser) {
	r.parsers = append([]Parser{p}, r.parsers...)
}

// ByBank returns the most recently registered parser for the named bank, or an error.
func (r *Registry) ByBank(bank string) (Parser, error) {
	bank = strings.ToLower(bank)
	for _, p := range r.parsers {
		if p.Bank() == bank {
			return p, nil
		}
	}
	return nil, fmt.Errorf("unknown bank %q — supported: axis, idfc, sbi, icici, hdfc", bank)
}

// Detect returns the first parser whose CanParse returns true for the given header row.
func (r *Registry) Detect(header []string) (Parser, error) {
	norm := make([]string, len(header))
	for i, h := range header {
		norm[i] = strings.ToLower(strings.TrimSpace(h))
	}
	for _, p := range r.parsers {
		if p.CanParse(norm) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no parser matched header: %v", header)
}

// DetectFile infers the bank identifier from a statement file path.
// For .xls the bank is always "icici"; for .xlsx always "sbi".
// For .csv it scans rows until one matches a registered parser's CanParse.
func (r *Registry) DetectFile(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".xls":
		return r.detectXLS(path)
	case ".xlsx":
		return "sbi", nil
	case ".csv":
		return r.detectCSV(path)
	default:
		return "", fmt.Errorf("unsupported file extension %q", ext)
	}
}

// detectXLS peeks at an XLS file's rows and returns the matching bank name.
func (r *Registry) detectXLS(path string) (string, error) {
	wb, err := xlslib.Open(path, "utf-8")
	if err != nil {
		return "", fmt.Errorf("open xls for detection: %w", err)
	}
	sheet := wb.GetSheet(0)
	if sheet == nil {
		return "", fmt.Errorf("xls has no sheets: %s", path)
	}
	for rowIdx := 0; rowIdx <= int(sheet.MaxRow); rowIdx++ {
		row := sheet.Row(rowIdx)
		if row == nil {
			continue
		}
		var cells []string
		for colIdx := row.FirstCol(); colIdx <= row.LastCol(); colIdx++ {
			cells = append(cells, strings.ToLower(strings.TrimSpace(row.Col(colIdx))))
		}
		p, err := r.Detect(cells)
		if err == nil {
			return p.Bank(), nil
		}
	}
	return "", fmt.Errorf("could not detect bank from XLS %q", path)
}

func (r *Registry) detectCSV(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open for detection: %w", err)
	}
	defer f.Close()

	rdr := csv.NewReader(f)
	rdr.LazyQuotes = true
	rdr.FieldsPerRecord = -1

	for {
		record, err := rdr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		p, err := r.Detect(record)
		if err == nil {
			return p.Bank(), nil
		}
	}
	return "", fmt.Errorf("could not detect bank from %q — pass --bank explicitly", path)
}
