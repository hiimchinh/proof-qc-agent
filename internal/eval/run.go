package eval

import (
	"errors"
	"sync"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
)

// Budget caps what a run may spend. Calls already in flight finish, so a run
// can pass the cap by at most one call per worker.
type Budget struct {
	Max   float64
	mu    sync.Mutex
	spent float64
}

func (b *Budget) add(usd float64) {
	b.mu.Lock()
	b.spent += usd
	b.mu.Unlock()
}

// Spent is the total recorded so far.
func (b *Budget) Spent() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.spent
}

func (b *Budget) exhausted() bool { return b.Spent() >= b.Max }

// Run reviews every case with the given number of workers. Once the budget is
// spent, the remaining cases are marked skipped instead of failed.
func Run(cases []Case, review func(Case) (agent.Result, error), workers int, b *Budget) ([]Outcome, error) {
	if workers < 1 {
		return nil, errors.New("workers must be at least 1")
	}
	outs := make([]Outcome, len(cases))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				outs[i] = Outcome{Case: cases[i]}
				if b.exhausted() {
					outs[i].Skipped = true
					continue
				}
				res, err := review(cases[i])
				b.add(res.CostUSD)
				outs[i].Result = res
				if err != nil {
					outs[i].Err = err.Error()
				}
			}
		}()
	}
	for i := range cases {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return outs, nil
}
