package engine

import (
	"context"
	"log"
	"sync"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type WorkerPool struct {
	workers int
	check   CheckFunc
}

func NewWorkerPool(workers int, check CheckFunc) *WorkerPool {
	if workers < 1 {
		workers = 1
	}

	return &WorkerPool{
		workers: workers,
		check:   check,
	}
}

func (p *WorkerPool) Run(
	ctx context.Context,
	monitors []models.Monitor,
) []models.CheckResult {
	jobs := make(chan models.Monitor)
	results := make(chan models.CheckResult)

	var wg sync.WaitGroup

	for i := 0; i < p.workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for monitor := range jobs {
				result := p.check(ctx, monitor)
				results <- result
			}
		}()
	}

	go func() {
		defer close(jobs)

		for _, monitor := range monitors {
			select {
			case jobs <- monitor:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var collected []models.CheckResult

	for result := range results {
		collected = append(collected, result)
	}

	log.Printf(
		"worker pool completed %d checks with %d workers",
		len(collected),
		p.workers,
	)

	return collected
}
