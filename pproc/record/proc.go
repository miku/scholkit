package record

import (
	"bufio"
	"context"
	"io"
	"runtime"
	"sync"

	"golang.org/x/sync/errgroup"
)

const (
	defaultMaxBufferSize = 1 << 24 // 16MB, soft limit
	defaultMaxTokenSize  = 1 << 26 // 64MB, hard limit, needs to be larger than the buffer size
)

// ProcessFunc is the function type that users provide to transform batches
type ProcessFunc func([]byte) ([]byte, error)

// ProcessorOption allows configuration of the Processor
type ProcessorOption func(*Processor)

// WithWorkers sets the number of worker goroutines
func WithWorkers(n int) ProcessorOption {
	return func(p *Processor) {
		if n > 0 {
			p.numWorkers = n
		}
	}
}

// WithMaxTokenSize sets the maximum token size for the splitter
func WithMaxTokenSize(size int) ProcessorOption {
	return func(p *Processor) {
		if size > 0 {
			p.maxTokenSize = size
		}
	}
}

// WithMaxBufferSize sets the maximum buffer size for the splitter
func WithMaxBufferSize(size int) ProcessorOption {
	return func(p *Processor) {
		if size > 0 {
			p.maxBufferSize = size
		}
	}
}

func WithSplitFunc(f bufio.SplitFunc) ProcessorOption {
	return func(p *Processor) {
		p.splitFunc = f
	}
}

// Processor handles parallel processing of records, delineated by a provided
// bufio.SplitFunc.
type Processor struct {
	splitFunc     bufio.SplitFunc
	processFunc   ProcessFunc
	numWorkers    int
	maxBufferSize int
	maxTokenSize  int
}

// NewProcessor creates a new Processor that by default splits of lines.
func NewProcessor(processFunc ProcessFunc, opts ...ProcessorOption) *Processor {
	p := &Processor{
		splitFunc:     bufio.ScanLines,
		processFunc:   processFunc,
		numWorkers:    runtime.NumCPU(),
		maxBufferSize: defaultMaxBufferSize,
		maxTokenSize:  defaultMaxTokenSize,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Split sets the split function. By default we split on lines.
func (p *Processor) Split(f bufio.SplitFunc) {
	p.splitFunc = f
}

// Process reads from the input, processes batches in parallel, and writes results to output
func (p *Processor) Process(ctx context.Context, r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	scanner := bufio.NewScanner(br)
	scanner.Split(p.splitFunc)
	scanner.Buffer(make([]byte, 0, p.maxBufferSize), p.maxTokenSize)
	workChan := make(chan []byte, p.numWorkers*2)
	var writeMu sync.Mutex
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer close(workChan)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			token := scanner.Bytes()
			data := make([]byte, len(token))
			copy(data, token)
			select {
			case workChan <- data:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return scanner.Err()
	})
	for i := 0; i < p.numWorkers; i++ {
		g.Go(func() error {
			for data := range workChan {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				result, err := p.processFunc(data)
				if err != nil {
					return err
				}
				if result != nil {
					writeMu.Lock()
					_, err := bw.Write(result)
					writeMu.Unlock()
					if err != nil {
						return err
					}
				}
			}
			return nil
		})
	}
	return g.Wait()
}
