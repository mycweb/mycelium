package mvm1

// I is an instruction, it changes the state of the VM
type I interface {
	isI()
}

type baseI struct{}

func (baseI) isI() {}

// literals / parameters

type pushI struct {
	x uint32
	baseI
}

type selfI struct{ baseI }

type pickI struct {
	wdepth  int
	wlength int
	baseI
}

type muxI struct {
	table    []Word
	elemSize int

	baseI
}

// compiler

type lazyI struct {
	et     ProgType
	layout []Type

	baseI
}

type lambdaI struct {
	et      ProgType
	layout  []Type
	scratch []int

	baseI
}

// control flow

type evalI struct{ baseI }

type applyI struct {
	lambdaType              LambdaType
	inputWords, outputWords int
	baseI
}

type retI struct {
	inputWords, outputWords int
	baseI
}

type branchI struct {
	offset uint32
	baseI
}

type jumpI struct {
	offset uint32
	baseI
}

type panicI struct{ baseI }

type accelI struct {
	inputWords, outputWords int
	fn                      AccelFunc
	baseI
}

// stack

type discardI struct {
	words int
	baseI
}

type cutI struct {
	wdepth  int
	wlength int
	baseI
}

type swapI struct {
	loWords, hiWords int
	baseI
}

type equalI struct {
	words int
	baseI
}

// store

type pushRefI struct {
	x Ref
	baseI
}

type loadI struct {
	outputBits int
	baseI
}

type postI struct {
	salt      *Ref
	inputBits int
	baseI
}

// make / break

type makeSumI struct {
	tag       Word
	inputSize int
	tagOffset int
	tagSize   int

	baseI
}

type concatI struct {
	leftBits  int
	rightBits int
	baseI
}

type sliceI struct {
	inputBits int
	beg, end  int
	baseI
}

type arrayGetI struct {
	len      int
	elemSize int

	baseI
}

// lists

type listToI struct {
	elemSize   int
	desiredLen int
	baseI
}

type listGetI struct {
	elemSize int
	baseI
}

// any

type pushAnyTypeI struct {
	x AnyType
	baseI
}

type anyTypeToI struct {
	to         Type2
	outputBits int
	baseI
}

type anyTypeFromI struct {
	t2        Type2
	inputBits int
	baseI
}

type anyValueToI struct {
	to         AnyType
	outputBits int
	baseI
}

type anyValueFromI struct {
	t2        Type2
	tyRef     Ref
	needsSalt bool
	inputBits int
	baseI
}

type sizeOfI struct {
	baseI
}

// ports

type portOutputI struct {
	// consumeWords are taken from the top of the stack
	consumeWords int
	baseI
}

type portInputI struct {
	// produceWords are pushed on the top of the stack
	produceWords int
	baseI
}

type portInteractI struct {
	consumeWords, produceWords int
	baseI
}
