package mangle_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	manglepkg "codenerd/internal/mangle"
	"codenerd/internal/mangle/synth"
	"codenerd/internal/mangle/transpiler"
)

func TestProductionParserCallersShareSerializedEntryPoint(t *testing.T) {
	const workers = 48
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup

	spec := synth.Spec{
		Format: synth.FormatV1,
		Program: synth.ProgramSpec{
			Clauses: []synth.ClauseSpec{{
				Head: synth.AtomSpec{
					Pred: "p",
					Args: []synth.ExprSpec{{Kind: "number", Value: "1"}},
				},
			}},
		},
	}

	for i := range workers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start

			var err error
			switch worker % 4 {
			case 0:
				_, err = manglepkg.ParseUnit(strings.NewReader(`p(1).`))
			case 1:
				_, err = manglepkg.ParseAtom(`p(1)`)
			case 2:
				_, err = transpiler.NewSanitizer().Sanitize(`p("value").`)
			case 3:
				_, err = synth.Compile(spec, synth.DefaultOptions())
			}
			if err != nil {
				errs <- fmt.Errorf("worker %d: %w", worker, err)
			}
		}(i)
	}

	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
