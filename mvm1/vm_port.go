package mvm1

import (
	"context"
	"fmt"
	"slices"

	"golang.org/x/exp/maps"

	"myceliumweb.org/mycelium/internal/cadata"
)

type (
	IOFunc     = func(ctx context.Context, s cadata.Store, buf []Word) error
	InputFunc  = func(ctx context.Context, s cadata.PostExister, buf []Word) error
	OutputFunc = func(ctx context.Context, s cadata.Getter, buf []Word) error
)

type PortBackend struct {
	Input    InputFunc
	Output   OutputFunc
	Interact IOFunc
}

// Port
type Port [8]Word

func PortFromBytes(x [32]byte) (ret Port) {
	bytesToWords(x[:], ret[:])
	return ret
}

func (vm *VM) PutPort(k Port, port PortBackend) {
	vm.ports[k] = port
}

func (vm *VM) RemovePort(pv Port) {
	delete(vm.ports, pv)
}

func (vm *VM) ListPorts() []Port {
	pvs := maps.Keys(vm.ports)
	slices.SortFunc(pvs, func(a, b Port) int {
		return slices.Compare(a[:], b[:])
	})
	return pvs
}

func (vm *VM) pushPort(pv Port) {
	for _, w := range pv {
		vm.push(w)
	}
}

func (vm *VM) popPort() (ret Port) {
	vm.popInto(ret[:])
	return ret
}

// portOutput pops a port off the stack, and consumes consumeWords from the stack
func (vm *VM) portOutput(consumeWords int) {
	vm.portIO(consumeWords, 0, func(p PortBackend, buf []Word) error {
		return p.Output(vm.ctx, vm.store, buf)
	})
}

// portInput pops a port off the stack, and produces produceWords onto the stack
func (vm *VM) portInput(produceWords int) {
	vm.portIO(0, produceWords, func(p PortBackend, buf []Word) error {
		return p.Input(vm.ctx, vm.store, buf)
	})
}

// portInteract pops a port off of the top of the stack,
// consumes consumeWords from the stack, and produces produceWords onto the stack
func (vm *VM) portInteract(consumeWords, produceWords int) {
	vm.portIO(consumeWords, produceWords, func(p PortBackend, buf []Word) error {
		return p.Interact(vm.ctx, vm.store, buf)
	})
}

func (vm *VM) portIO(consumeWords, produceWords int, fn func(PortBackend, []Word) error) {
	pv := vm.popPort()
	backend, exists := vm.ports[pv]
	if !exists {
		vm.fail(fmt.Errorf("invalid port: %v", pv))
		return
	}

	pos := len(vm.stack)
	for i := consumeWords; i < produceWords; i++ {
		vm.stack = append(vm.stack, 13)
	}
	buf := vm.stack[pos-consumeWords:]
	if err := fn(backend, buf); err != nil {
		vm.fail(fmt.Errorf("port op failed: %w", err))
		return
	}
	vm.stack = vm.stack[:pos+produceWords-consumeWords]
}
