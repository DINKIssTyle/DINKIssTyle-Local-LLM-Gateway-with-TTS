package mediapipeworker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
)

const (
	Argument  = "--dkst-mediapipe-worker"
	dimension = 256
)

type request struct {
	Op, Text, Usage, Instructions string
	Criteria                      map[string]string
}

type reply struct {
	Ready         bool               `json:"ready,omitempty"`
	Dimension     int                `json:"dimension,omitempty"`
	Vector        []float64          `json:"vector,omitempty"`
	Selected      string             `json:"selected,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Error         string             `json:"error,omitempty"`
}

var tokenOverflow = regexp.MustCompile(`Input sequence length \(\d+\) exceeds maximum supported signature length \(\d+\)`)

func normalize(vector []float64) ([]float64, error) {
	var sum float64
	for _, value := range vector {
		sum += value * value
	}
	if sum == 0 || math.IsNaN(sum) || math.IsInf(sum, 0) {
		return nil, fmt.Errorf("invalid embedding magnitude")
	}
	norm := math.Sqrt(sum)
	for i := range vector {
		vector[i] /= norm
	}
	return vector, nil
}

// Preserve the existing query/document prompts and split only on actual SDK
// token overflow. Use rune counts for the same weights as the Python worker.
func embedText(embed func(string) ([]float64, error), text, usage string) ([]float64, error) {
	prefix := "title: memory | text: "
	if usage == "query" {
		prefix = "task: search result | query: "
	}
	var embedPart func([]rune) ([]float64, error)
	embedPart = func(part []rune) ([]float64, error) {
		vector, err := embed(prefix + string(part))
		if err == nil {
			return normalize(vector)
		}
		if len(part) < 2 || !tokenOverflow.MatchString(err.Error()) {
			return nil, err
		}
		middle := len(part) / 2
		left, err := embedPart(part[:middle])
		if err != nil {
			return nil, err
		}
		right, err := embedPart(part[middle:])
		if err != nil {
			return nil, err
		}
		weight := float64(middle) / float64(len(part))
		for i := range left {
			left[i] = left[i]*weight + right[i]*(1-weight)
		}
		return left, nil
	}
	vector, err := embedPart([]rune(text))
	if err != nil {
		return nil, err
	}
	return normalize(vector)
}

func serve(in io.Reader, out io.Writer, embed func(string) ([]float64, error), choose func(request) (reply, error)) error {
	encoder := json.NewEncoder(out)
	if err := encoder.Encode(reply{Ready: true, Dimension: dimension}); err != nil {
		return err
	}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var req request
		var res reply
		err := json.Unmarshal(scanner.Bytes(), &req)
		if err == nil {
			switch req.Op {
			case "embed":
				res.Vector, err = embedText(embed, req.Text, req.Usage)
			case "choice":
				res, err = choose(req)
			default:
				err = fmt.Errorf("unknown MediaPipe operation")
			}
		}
		if err != nil {
			res = reply{Error: err.Error()}
		}
		if err := encoder.Encode(res); err != nil {
			return err
		}
	}
	return scanner.Err()
}
