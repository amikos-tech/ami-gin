package gin

import (
	"strings"
	"unicode"

	"github.com/pkg/errors"
)

type NGramConfig struct {
	N       int
	Padding string
}

type NGramOption func(*NGramConfig) error

func WithN(n int) NGramOption {
	return func(c *NGramConfig) error {
		if n < 2 {
			return errors.Errorf("n must be at least 2, got %d", n)
		}
		c.N = n
		return nil
	}
}

func WithPadding(pad string) NGramOption {
	return func(c *NGramConfig) error {
		c.Padding = pad
		return nil
	}
}

type TrigramIndex struct {
	Trigrams  map[string]*RGSet
	NumRGs    int
	N         int
	Padding   string
	MinLength int
}

func NewTrigramIndex(numRGs int, opts ...NGramOption) (*TrigramIndex, error) {
	cfg := &NGramConfig{N: 3}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	return &TrigramIndex{
		Trigrams:  make(map[string]*RGSet),
		NumRGs:    numRGs,
		N:         cfg.N,
		Padding:   cfg.Padding,
		MinLength: cfg.N,
	}, nil
}

func (ti *TrigramIndex) Add(value string, rgID int) {
	runes := []rune(strings.ToLower(value))
	if len(runes) < ti.MinLength {
		return
	}

	ngrams := ti.generateNGrams(runes)
	for _, ng := range ngrams {
		if _, ok := ti.Trigrams[ng]; !ok {
			ti.Trigrams[ng] = MustNewRGSet(ti.NumRGs)
		}
		ti.Trigrams[ng].Set(rgID)
	}
}

func (ti *TrigramIndex) Search(pattern string) *RGSet {
	runes := []rune(strings.ToLower(pattern))
	if len(runes) < ti.N {
		return AllRGs(ti.NumRGs)
	}

	ngrams := ti.generateNGrams(runes)
	if len(ngrams) == 0 {
		return AllRGs(ti.NumRGs)
	}

	var result *RGSet
	for _, ng := range ngrams {
		rgSet, ok := ti.Trigrams[ng]
		if !ok {
			return NoRGs(ti.NumRGs)
		}
		if result == nil {
			result = rgSet.Clone()
		} else {
			result = result.Intersect(rgSet)
		}
		if result.IsEmpty() {
			return result
		}
	}

	return result
}

func (ti *TrigramIndex) generateNGrams(runes []rune) []string {
	text := runes
	if ti.Padding != "" {
		padRunes := []rune(strings.Repeat(ti.Padding, ti.N-1))
		text = make([]rune, 0, len(padRunes)*2+len(runes))
		text = append(text, padRunes...)
		text = append(text, runes...)
		text = append(text, padRunes...)
	}

	if len(text) < ti.N {
		return nil
	}

	seen := make(map[string]struct{})
	result := make([]string, 0, len(text)-ti.N+1)

	for i := 0; i <= len(text)-ti.N; i++ {
		gram := string(text[i : i+ti.N])

		if isWhitespaceOnly(gram) || isPunctuationOnly(gram) {
			continue
		}

		if _, ok := seen[gram]; !ok {
			seen[gram] = struct{}{}
			result = append(result, gram)
		}
	}

	return result
}

func (ti *TrigramIndex) TrigramCount() int {
	return len(ti.Trigrams)
}

func isWhitespaceOnly(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func isPunctuationOnly(s string) bool {
	for _, r := range s {
		if !unicode.IsPunct(r) && !unicode.IsSymbol(r) {
			return false
		}
	}
	return true
}

func GenerateNGrams(text string, n int, opts ...NGramOption) ([]string, error) {
	cfg := &NGramConfig{N: n}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	text = strings.ToLower(text)
	runes := []rune(text)

	if cfg.Padding != "" {
		padRunes := []rune(strings.Repeat(cfg.Padding, n-1))
		padded := make([]rune, 0, len(padRunes)*2+len(runes))
		padded = append(padded, padRunes...)
		padded = append(padded, runes...)
		padded = append(padded, padRunes...)
		runes = padded
	}

	if len(runes) < n {
		return nil, nil
	}

	seen := make(map[string]struct{})
	result := make([]string, 0, len(runes)-n+1)

	for i := 0; i <= len(runes)-n; i++ {
		gram := string(runes[i : i+n])
		if isWhitespaceOnly(gram) || isPunctuationOnly(gram) {
			continue
		}
		if _, ok := seen[gram]; !ok {
			seen[gram] = struct{}{}
			result = append(result, gram)
		}
	}

	return result, nil
}

func GenerateTrigrams(text string) []string {
	result, _ := GenerateNGrams(text, 3)
	return result
}

func GenerateBigrams(text string) []string {
	result, _ := GenerateNGrams(text, 2)
	return result
}

func ExtractTrigrams(s string) []string {
	return GenerateTrigrams(s)
}
