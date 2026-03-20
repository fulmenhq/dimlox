package inspect

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type DetectResult struct {
	URI                 string  `json:"uri"`
	Compressed          bool    `json:"compressed"`
	Encoding            string  `json:"encoding"`
	BOM                 bool    `json:"bom"`
	LineEnding          string  `json:"line_ending"`
	Delimiter           string  `json:"delimiter"`
	DelimiterConfidence float64 `json:"delimiter_confidence"`
	FieldsPerRow        int     `json:"fields_per_row"`
	SampleRows          int     `json:"sample_rows"`
	SampleBytes         int64   `json:"sample_bytes"`
}

var detectDelimiters = []string{"|", ",", "\t", ";"}

func Detect(ctx context.Context, rawURI string, sampleBytes int64, opts ProviderOptions) (*DetectResult, error) {
	if sampleBytes <= 0 {
		return nil, fmt.Errorf("sample-bytes must be > 0")
	}

	src, _, err := providerForURI(ctx, rawURI, opts)
	if err != nil {
		return nil, err
	}
	meta, err := src.Stat(ctx, rawURI)
	if err != nil {
		return nil, err
	}
	r, compressed, err := openStream(ctx, src, rawURI, meta)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = r.Close()
	}()

	sample, truncated, err := readDetectSample(r, sampleBytes)
	if err != nil {
		return nil, err
	}
	if len(sample) == 0 {
		return nil, fmt.Errorf("inspect detect: no sample data available")
	}

	encoding, hasBOM, text, err := detectEncoding(sample)
	if err != nil {
		return nil, err
	}
	lineEnding := detectLineEnding(text)
	delim, confidence, fieldsPerRow, sampleRows, err := detectDelimiter(text, truncated)
	if err != nil {
		return nil, err
	}

	return &DetectResult{
		URI:                 rawURI,
		Compressed:          compressed,
		Encoding:            encoding,
		BOM:                 hasBOM,
		LineEnding:          lineEnding,
		Delimiter:           delim,
		DelimiterConfidence: confidence,
		FieldsPerRow:        fieldsPerRow,
		SampleRows:          sampleRows,
		SampleBytes:         int64(len(sample)),
	}, nil
}

func readDetectSample(r io.Reader, sampleBytes int64) ([]byte, bool, error) {
	raw, err := io.ReadAll(io.LimitReader(r, sampleBytes+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(raw)) > sampleBytes {
		return raw[:sampleBytes], true, nil
	}
	return raw, false, nil
}

func detectEncoding(sample []byte) (string, bool, string, error) {
	switch {
	case bytes.HasPrefix(sample, []byte{0x00, 0x00, 0xFE, 0xFF}):
		text, err := decodeUTF32(sample[4:], binary.BigEndian)
		return "UTF-32BE", true, text, err
	case bytes.HasPrefix(sample, []byte{0xFF, 0xFE, 0x00, 0x00}):
		text, err := decodeUTF32(sample[4:], binary.LittleEndian)
		return "UTF-32LE", true, text, err
	case bytes.HasPrefix(sample, []byte{0xFE, 0xFF}):
		text, err := decodeUTF16(sample[2:], binary.BigEndian)
		return "UTF-16BE", true, text, err
	case bytes.HasPrefix(sample, []byte{0xFF, 0xFE}):
		text, err := decodeUTF16(sample[2:], binary.LittleEndian)
		return "UTF-16LE", true, text, err
	case bytes.HasPrefix(sample, []byte{0xEF, 0xBB, 0xBF}):
		return "UTF-8", true, string(sample[3:]), nil
	}

	if bytes.IndexByte(sample, 0x00) >= 0 {
		if likelyUTF32(sample) {
			order := likelyUTF32Order(sample)
			text, err := decodeUTF32(sample, order)
			if err != nil {
				return "", false, "", err
			}
			if order == binary.BigEndian {
				return "UTF-32BE", false, text, nil
			}
			return "UTF-32LE", false, text, nil
		}

		order := likelyUTF16Order(sample)
		text, err := decodeUTF16(sample, order)
		if err != nil {
			return "", false, "", err
		}
		if order == binary.BigEndian {
			return "UTF-16BE", false, text, nil
		}
		return "UTF-16LE", false, text, nil
	}

	if utf8.Valid(sample) {
		return "UTF-8", false, string(sample), nil
	}

	return "UTF-8", false, strings.ToValidUTF8(string(sample), ""), nil
}

func decodeUTF16(sample []byte, order binary.ByteOrder) (string, error) {
	if len(sample)%2 != 0 {
		sample = sample[:len(sample)-1]
	}
	if len(sample) == 0 {
		return "", nil
	}
	words := make([]uint16, 0, len(sample)/2)
	for i := 0; i+1 < len(sample); i += 2 {
		words = append(words, order.Uint16(sample[i:i+2]))
	}
	return string(utf16.Decode(words)), nil
}

func decodeUTF32(sample []byte, order binary.ByteOrder) (string, error) {
	if len(sample)%4 != 0 {
		sample = sample[:len(sample)-(len(sample)%4)]
	}
	if len(sample) == 0 {
		return "", nil
	}
	runes := make([]rune, 0, len(sample)/4)
	for i := 0; i+3 < len(sample); i += 4 {
		r := rune(order.Uint32(sample[i : i+4]))
		if !utf8.ValidRune(r) {
			r = utf8.RuneError
		}
		runes = append(runes, r)
	}
	return string(runes), nil
}

func likelyUTF16Order(sample []byte) binary.ByteOrder {
	var evenNulls, oddNulls int
	for i, b := range sample {
		if b != 0 {
			continue
		}
		if i%2 == 0 {
			evenNulls++
		} else {
			oddNulls++
		}
	}
	if evenNulls > oddNulls {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

func likelyUTF32(sample []byte) bool {
	if len(sample) < 8 {
		return false
	}
	var zeroes int
	for _, b := range sample {
		if b == 0 {
			zeroes++
		}
	}
	return float64(zeroes)/float64(len(sample)) >= 0.45
}

func likelyUTF32Order(sample []byte) binary.ByteOrder {
	var leadingNulls, trailingNulls int
	for i := 0; i+3 < len(sample); i += 4 {
		if sample[i] == 0 && sample[i+1] == 0 {
			leadingNulls++
		}
		if sample[i+2] == 0 && sample[i+3] == 0 {
			trailingNulls++
		}
	}
	if leadingNulls > trailingNulls {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

func detectLineEnding(text string) string {
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\r':
			if i+1 < len(text) && text[i+1] == '\n' {
				return "CRLF"
			}
		case '\n':
			return "LF"
		}
	}
	return "unknown"
}

func detectDelimiter(text string, truncated bool) (string, float64, int, int, error) {
	lines := sampledLines(text, 100, truncated)
	if len(lines) == 0 {
		return "", 0, 0, 0, fmt.Errorf("inspect detect: sample does not contain any complete rows")
	}

	best := delimiterScore{}
	for _, candidate := range detectDelimiters {
		score := scoreDelimiter(candidate, lines)
		if score.betterThan(best) {
			best = score
		}
	}

	if !best.valid {
		return "", 0, 0, len(lines), fmt.Errorf("inspect detect: unable to determine delimiter with sufficient confidence")
	}

	return best.delimiter, best.confidence, best.fieldsPerRow, len(lines), nil
}

func sampledLines(text string, limit int, truncated bool) []string {
	rawLines := strings.Split(text, "\n")
	if truncated && !strings.HasSuffix(text, "\n") && len(rawLines) > 0 {
		rawLines = rawLines[:len(rawLines)-1]
	}
	lines := make([]string, 0, min(limit, len(rawLines)))
	for _, line := range rawLines {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) == limit {
			break
		}
	}
	return lines
}

type delimiterScore struct {
	delimiter    string
	confidence   float64
	fieldsPerRow int
	valid        bool
}

func (s delimiterScore) betterThan(other delimiterScore) bool {
	if s.valid != other.valid {
		return s.valid
	}
	if s.confidence != other.confidence {
		return s.confidence > other.confidence
	}
	if s.fieldsPerRow != other.fieldsPerRow {
		return s.fieldsPerRow > other.fieldsPerRow
	}
	return s.delimiter < other.delimiter
}

func scoreDelimiter(delimiter string, lines []string) delimiterScore {
	counts := make([]int, 0, len(lines))
	freq := make(map[int]int)
	for _, line := range lines {
		count := strings.Count(line, delimiter) + 1
		counts = append(counts, count)
		freq[count]++
	}

	dominantFields := 0
	dominantCount := 0
	for fields, count := range freq {
		if fields < 2 {
			continue
		}
		if count > dominantCount || (count == dominantCount && fields > dominantFields) {
			dominantFields = fields
			dominantCount = count
		}
	}
	if dominantFields < 2 || len(lines) == 0 {
		return delimiterScore{delimiter: delimiter}
	}

	consistency := float64(dominantCount) / float64(len(lines))
	if consistency < 0.80 {
		return delimiterScore{delimiter: delimiter}
	}

	mean := 0.0
	for _, count := range counts {
		mean += float64(count)
	}
	mean /= float64(len(counts))
	if mean < 2 {
		return delimiterScore{delimiter: delimiter}
	}

	variance := 0.0
	for _, count := range counts {
		delta := float64(count) - mean
		variance += delta * delta
	}
	variance /= float64(len(counts))
	stddev := math.Sqrt(variance)
	cv := 0.0
	if mean != 0 {
		cv = stddev / mean
	}
	confidence := consistency / (1 + cv)

	return delimiterScore{
		delimiter:    delimiter,
		confidence:   confidence,
		fieldsPerRow: dominantFields,
		valid:        true,
	}
}
