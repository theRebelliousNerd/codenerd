package system

import (
	"fmt"
	"reflect"
	"slices"

	"codenerd/internal/prompt"
)

var (
	_ prompt.KernelFactSnapshotQuerier = (*KernelAdapter)(nil)
	_ prompt.KernelFactSnapshotQuerier = (*kernelCompilationScope)(nil)
)

func (adapter *KernelAdapter) QueryAll() (map[string][]prompt.Fact, error) {
	if adapter == nil {
		return nil, fmt.Errorf("query prompt kernel snapshot: nil adapter")
	}
	if adapter.kernel == nil {
		return nil, fmt.Errorf("query prompt kernel snapshot: nil kernel")
	}
	kernelValue := reflect.ValueOf(adapter.kernel)
	switch kernelValue.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if kernelValue.IsNil() {
			return nil, fmt.Errorf("query prompt kernel snapshot: nil kernel")
		}
	}
	facts, err := adapter.kernel.QueryAll()
	if err != nil {
		return nil, fmt.Errorf("query prompt kernel snapshot: %w", err)
	}
	snapshot := make(map[string][]prompt.Fact, len(facts))
	for predicate, rows := range facts {
		var converted []prompt.Fact
		if rows != nil {
			converted = make([]prompt.Fact, len(rows))
		}
		for index, fact := range rows {
			converted[index] = prompt.Fact{Predicate: fact.Predicate, Args: slices.Clone(fact.Args)}
		}
		snapshot[predicate] = converted
	}
	return snapshot, nil
}
