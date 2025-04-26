package mvm1

import (
	"context"
	"fmt"

	"slices"

	"myceliumweb.org/mycelium/internal/cadata"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"

	"go.brendoncarroll.net/exp/slices2"
)

type Compiler struct {
	store  cadata.Store
	accels map[Fingerprint]AccelFunc
	eval   func(context.Context, []I) ([]Word, error)

	prodOffCache map[Fingerprint][]int
}

func newCompiler(s cadata.Store, accels map[Fingerprint]AccelFunc, eval func(context.Context, []I) ([]Word, error)) Compiler {
	return Compiler{store: s, eval: eval, accels: accels}
}

// Prog is a compiled program.
// It includes the Type that will be left on the stack (the result)
// and the function that will run it.
type Prog struct {
	// Type is the type of the data at the top of the stack after this program has executed.
	Type Type
	// I is a slice of instructions
	I []I
	// Value is the result of the Prog, if it is nil, the result is not a constant
	Value []Word
}

// IsType returns true if this program leaves a Type on the stack either as an AnyType or in the native encoding
func (p *Prog) IsType() bool {
	return p.Type.IsAnyTypeType() || p.Type.Type2.TypeCode() == spec.TC_Kind
}

// IsConstant returns true if the program just loads a constant value
func (p *Prog) IsConstant() bool {
	return p.Value != nil
}

func (c *Compiler) evalNow(ctx context.Context, x *Prog, ty *Type) ([]Word, error) {
	if !x.Type.Equals(ty) {
		return nil, fmt.Errorf("compile time evaluation would produce wrong type. HAVE: %v WANT: %v", x.Type, ty)
	}
	var ws []Word
	if x.Value == nil {
		var err error
		// TODO: check for ND here.
		if ws, err = c.eval(ctx, x.I); err != nil {
			return nil, err
		}
	} else {
		ws = x.Value
	}
	if len(ws) != ty.SizeWords() {
		return nil, fmt.Errorf("compile time eval, produced wrong sized data. HAVE: %v WANT: %v", len(ws), ty.SizeWords())
	}
	return ws, nil
}

func (c *Compiler) makeLazy(ctx context.Context, mc machCtx, mprog myc.Prog) (*Lazy, *Prog, error) {
	body, err := mc.closure(ctx, 0, true, mprog)
	if err != nil {
		return nil, nil, err
	}
	if err := myc.ValidateBody(myc.NewExpr(body), 0, !mc.self.IsZero()); err != nil {
		return nil, nil, err
	}
	prog, err := c.compile(ctx, mc, body)
	if err != nil {
		return nil, nil, err
	}
	inputWords, outputWords := 0, prog.Type.SizeWords()
	prog.I = append(prog.I, retI{
		inputWords:  inputWords,
		outputWords: outputWords,
	})
	mref, err := myc.Post(ctx, c.store, &body)
	if err != nil {
		return nil, nil, err
	}
	ref := RefFromCID(mref.Data())
	var laz Lazy
	laz.SetRef(ref)
	laz.SetProgType(newProgType(body.Size()))
	return &laz, prog, nil
}

func (c *Compiler) makeLambda(ctx context.Context, mc machCtx, inAT, outAT AnyType, mprog myc.Prog) (*Lambda, *Prog, error) {
	inType, err := c.loadType(ctx, inAT)
	if err != nil {
		return nil, nil, err
	}
	outType, err := c.loadType(ctx, outAT)
	if err != nil {
		return nil, nil, err
	}
	// logctx.Info(ctx, "makeLambda", zap.Any("in", inType), zap.Any("out", outType), zap.Any("body", e))
	lamType, err := c.makeHType(ctx, spec.TC_Lambda, []Type{inType, outType})
	if err != nil {
		return nil, nil, err
	}
	mc = mc.stackPush(inType)
	mc.self = dynValue{
		t2:       lamType.Type2,
		typeData: lamType.Data,
		// Leave value data empty
	}

	body, err := mc.closure(ctx, 1, false, mprog)
	if err != nil {
		return nil, nil, err
	}
	if err := myc.ValidateBody(myc.NewExpr(body), 1, true); err != nil {
		return nil, nil, err
	}
	prog, err := c.compile(ctx, mc, body)
	if err != nil {
		return nil, nil, err
	}
	prog.I = append(prog.I, retI{
		inputWords:  inType.SizeWords(),
		outputWords: outType.SizeWords(),
	})
	// check output type
	if err := c.checkSupersets(ctx, &outType, &prog.Type); err != nil {
		return nil, nil, fmt.Errorf("makeLambda: %w", err)
	}
	mref, err := myc.Post(ctx, c.store, &body)
	if err != nil {
		return nil, nil, err
	}
	ref := RefFromCID(mref.Data())
	var lam Lambda
	lam.SetRef(ref)
	lam.SetProgType(newProgType(body.Size()))
	return &lam, prog, nil
}

func (c *Compiler) compile(ctx context.Context, mc machCtx, x myc.Prog) (*Prog, error) {
	if x.IsLiteral() {
		return c.compileLiteral(ctx, x.Literal())
	}
	if x.IsParam() {
		return c.compileParam(mc, x.Param())
	}

	// ops in this block modify the machine context
	switch x.Code() {
	case spec.Let:
		return c.compileLet(ctx, mc, x.Input(0), x.Input(1))
	case spec.Lazy:
		return c.compileLazy(ctx, mc, x.Input(0))
	case spec.Lambda:
		return c.compileLambda(ctx, mc, x.Input(0), x.Input(1), x.Input(2))
	case spec.Fractal:
		return c.compileFractal(ctx, x.Input(0))
	case spec.Branch:
		return c.compileBranch(ctx, mc, x.Input(0), x.Input(1), x.Input(2))
	case spec.Try:
		panic(x) // TODO
	}

	// ops in this block evaluate some of their arguments at compile time, but do not modify the machine context
	switch x.Code() {
	case spec.Craft:
		return c.compileCraft(ctx, mc, x.Input(0), x.Input(1))
	case spec.Field:
		return c.compileField(ctx, mc, x.Input(0), x.Input(1))
	case spec.TypeOf:
		return c.compileTypeOf(ctx, mc, x.Input(0))
	case spec.ArrayEmpty:
		return c.compileArrayEmpty(ctx, mc, x.Input(0))
	case spec.MakeSum:
		return c.compileMakeSum(ctx, mc, x.Input(0), x.Input(1), x.Input(2))
	case spec.ListTo:
		return c.compileListTo(ctx, mc, x.Input(0), x.Input(1))
	case spec.AnyTypeTo:
		return c.compileAnyTypeTo(ctx, mc, x.Input(0), x.Input(1))
	case spec.AnyValueTo:
		return c.compileAnyValueTo(ctx, mc, x.Input(0), x.Input(1))
	}

	// The rest of the operations should all be uniform in:
	// - all inputs must evaluate at runtime before they can evaluate also at runtime
	// - they don't modify the context
	var argTypes [4]Type
	var argValues [4][]Word
	var out []I
	var scratch int
	for i := 0; i < x.NumInputs(); i++ {
		prog2, err := c.compile(ctx, mc, x.Input(i))
		if err != nil {
			return nil, err
		}
		scratch += prog2.Type.SizeWords()
		mc.addScratch(prog2.Type.SizeWords())
		argTypes[i] = prog2.Type
		argValues[i] = prog2.Value
		out = append(out, prog2.I...)
	}
	prog2, err := c.compileSimple(ctx, mc, x.Code(), argTypes, argValues)
	if err != nil {
		return nil, err
	}
	mc.addScratch(-1 * scratch)
	out = append(out, prog2.I...)
	return &Prog{
		Type: prog2.Type,
		I:    out,
	}, nil
}

func (c *Compiler) compileTypeOf(ctx context.Context, mc machCtx, a0 myc.Prog) (*Prog, error) {
	x, err := c.compile(ctx, mc, a0)
	if err != nil {
		return nil, err
	}
	tyData := x.Type.Data
	t2, err := c.makeType(ctx, newType2(spec.TC_Kind, 0), x.Type.Type2[:])
	if err != nil {
		return nil, err
	}
	return &Prog{
		Type: t2,
		I:    slices2.Map(tyData, func(x uint32) I { return pushI{x: x} }),
	}, nil
}

func (c *Compiler) compileArrayEmpty(ctx context.Context, mc machCtx, a0 myc.Prog) (*Prog, error) {
	elem, err := c.evalTypeNow(ctx, mc, a0)
	if err != nil {
		return nil, err
	}
	ty, err := c.arrayType(ctx, elem, 0)
	if err != nil {
		return nil, err
	}
	return &Prog{
		Type: ty,
		I:    nil,
	}, nil
}

func (c *Compiler) compileMakeSum(ctx context.Context, mc machCtx, a0, a1, a2 myc.Prog) (*Prog, error) {
	sumType, err := c.evalTypeNow(ctx, mc, a0)
	if err != nil {
		return nil, err
	}
	if sumType.Type2.TypeCode() != spec.TC_Sum {
		return nil, fmt.Errorf("MakeSum: first arg must be SumType have %v", sumType)
	}
	tagOffset, err := sumTagOffset(ctx, c.store, SumType(sumType.Data))
	if err != nil {
		return nil, err
	}
	sizeType, err := c.sizeType(ctx)
	if err != nil {
		return nil, err
	}
	tagProg, err := c.compile(ctx, mc, a1)
	if err != nil {
		return nil, err
	}
	tag, err := c.evalNow(ctx, tagProg, &sizeType)
	if err != nil {
		return nil, err
	}
	content, err := c.compile(ctx, mc, a2)
	if err != nil {
		return nil, err
	}
	inputSize := content.Type.Size
	st := SumType(sumType.Data)
	tagSize := spec.SumTagBits(st.Len())
	return &Prog{
		Type: sumType,
		I: append(content.I, makeSumI{
			tag:       tag[0],
			inputSize: inputSize,
			tagOffset: tagOffset,
			tagSize:   tagSize,
		}),
	}, nil
}

func (c *Compiler) compileListTo(ctx context.Context, mc machCtx, a0, a1 myc.Prog) (*Prog, error) {
	sizeProg, err := c.compile(ctx, mc, a1)
	if err != nil {
		return nil, err
	}
	st, err := c.sizeType(ctx)
	if err != nil {
		return nil, err
	}
	ws, err := c.evalNow(ctx, sizeProg, &st)
	if err != nil {
		return nil, err
	}
	desiredLen := int(ws[0])
	x, err := c.compile(ctx, mc, a0)
	if err != nil {
		return nil, err
	}
	elemTy, err := c.getTypeParam(ctx, &x.Type, 0)
	if err != nil {
		return nil, err
	}
	elemSize := elemTy.Size
	arrTy, err := c.arrayType(ctx, elemTy, desiredLen)
	if err != nil {
		return nil, err
	}
	outTy, err := c.refType(ctx, arrTy)
	if err != nil {
		return nil, err
	}
	return &Prog{
		Type: outTy,
		I: append(x.I, listToI{
			elemSize:   elemSize,
			desiredLen: desiredLen,
		}),
	}, nil
}

func (c *Compiler) compileAnyTypeTo(ctx context.Context, mc machCtx, a0, a1 myc.Prog) (*Prog, error) {
	atProg, err := c.compile(ctx, mc, a0)
	if err != nil {
		return nil, err
	}
	ty, err := c.evalTypeNow(ctx, mc, a1)
	if err != nil {
		return nil, err
	}
	toT2 := ty.Type2
	size := ty.Size
	return &Prog{
		Type: ty,
		I: append(atProg.I, anyTypeToI{
			to:         toT2,
			outputBits: size,
		}),
	}, nil
}

func (c *Compiler) compileAnyValueTo(ctx context.Context, mc machCtx, a0, a1 myc.Prog) (*Prog, error) {
	avProg, err := c.compile(ctx, mc, a0)
	if err != nil {
		return nil, err
	}
	ty, err := c.evalTypeNow(ctx, mc, a1)
	if err != nil {
		return nil, err
	}
	toAT := ty.AsAnyType()
	size := ty.Size
	return &Prog{
		Type: ty,
		I: append(avProg.I, anyValueToI{
			to:         toAT,
			outputBits: size,
		}),
	}, nil
}

func (c *Compiler) compileBranch(ctx context.Context, mc machCtx, checkE, b0E, b1E myc.Prog) (*Prog, error) {
	check, err := c.compile(ctx, mc, checkE)
	if err != nil {
		return nil, err
	}
	b0, err := c.compile(ctx, mc, b0E)
	if err != nil {
		return nil, err
	}
	b1, err := c.compile(ctx, mc, b1E)
	if err != nil {
		return nil, err
	}
	if !b0.Type.Equals(&b1.Type) {
		return nil, fmt.Errorf("branch: mismatched types")
	}
	return &Prog{
		Type: b0.Type,
		I: seq(
			check.I,
			mkBranch(b0.I, b1.I),
		),
	}, nil
}

func (c *Compiler) compileLet(ctx context.Context, mc machCtx, bindE, bodyE myc.Prog) (*Prog, error) {
	bind, err := c.compile(ctx, mc, bindE)
	if err != nil {
		return nil, err
	}
	mc2 := mc.stackPush(bind.Type)
	body, err := c.compile(ctx, mc2, bodyE)
	if err != nil {
		return nil, err
	}
	bindWords := bind.Type.SizeWords()
	bodyWords := body.Type.SizeWords()
	return &Prog{
		Type: body.Type,
		I: seq(
			bind.I,
			body.I,
			[]I{cutI{
				wdepth:  bindWords + bodyWords - 1,
				wlength: bindWords,
			}},
		),
	}, nil
}

func (c *Compiler) compileLazy(ctx context.Context, mc machCtx, bodyE myc.Prog) (*Prog, error) {
	bodyProg, err := c.compile(ctx, mc, bodyE)
	if err != nil {
		return nil, err
	}
	lazType, err := c.makeHType(ctx, spec.TC_Lazy, []Type{bodyProg.Type})
	if err != nil {
		return nil, err
	}
	cid, err := myc.Post(ctx, c.store, &bodyE)
	if err != nil {
		return nil, err
	}
	ref := RefFromCID(cid.Data())
	ty := newProgType(bodyE.Size())
	layout := slices.Clone(mc.layout)
	return &Prog{
		Type: lazType,
		I: []I{
			pushRefI{x: ref},
			pushI{x: 123456}, // TODO: dummy value
			lazyI{et: ty, layout: layout},
		},
	}, nil
}

func (c *Compiler) compileLambda(ctx context.Context, mc machCtx, inE, outE, bodyE myc.Prog) (*Prog, error) {
	inType, err := c.evalTypeNow(ctx, mc, inE)
	if err != nil {
		return nil, err
	}
	outType, err := c.evalTypeNow(ctx, mc, outE)
	if err != nil {
		return nil, err
	}
	lamType, err := c.makeHType(ctx, spec.TC_Lambda, []Type{inType, outType})
	if err != nil {
		return nil, err
	}
	mref, err := myc.Post(ctx, c.store, &bodyE)
	if err != nil {
		return nil, err
	}
	ref := RefFromCID(mref.Data())
	progType := newProgType(bodyE.Size())
	layout := slices.Clone(mc.layout)
	scratch := slices.Clone(mc.scratch)
	return &Prog{
		Type: lamType,
		I: []I{
			pushAnyTypeI{x: inType.AsAnyType()},
			pushAnyTypeI{x: outType.AsAnyType()},
			pushRefI{x: ref},
			pushI{x: 123456}, // TODO: dummy value
			lambdaI{et: progType, layout: layout, scratch: scratch},
		},
	}, nil
}

func (c *Compiler) compileFractal(ctx context.Context, bodyE myc.Prog) (*Prog, error) {
	outTy, err := c.makeType(ctx, newType2(spec.TC_Kind, 0), []Word{uint32(spec.TC_Fractal)})
	if err != nil {
		return nil, err
	}
	mr, err := myc.Post(ctx, c.store, &bodyE)
	if err != nil {
		return nil, err
	}
	ref := RefFromCID(mr.Data())
	pt := newProgType(bodyE.Size())
	return &Prog{
		Type: outTy,
		I: []I{
			pushRefI{x: ref},
			pushI{x: pt[0]},
		},
	}, nil
}

func (c *Compiler) compileCraft(ctx context.Context, mc machCtx, tyE, dataE myc.Prog) (*Prog, error) {
	ty, err := c.evalTypeNow(ctx, mc, tyE)
	if err != nil {
		return nil, err
	}
	dataProg, err := c.compile(ctx, mc, dataE)
	if err != nil {
		return nil, err
	}
	if ty.Size != dataProg.Type.Size {
		return nil, fmt.Errorf("make: wrong size for arg1 HAVE: %d WANT: %d", dataProg.Type.Size, ty.Size)
	}
	return &Prog{
		Type: ty,
		I:    dataProg.I,
	}, nil
}

func (c *Compiler) compileParam(mc machCtx, idx uint32) (*Prog, error) {
	wdepth, paramType, err := mc.pick(idx)
	if err != nil {
		return nil, err
	}
	wlength := paramType.SizeWords()
	return &Prog{
		Type: paramType,
		I: []I{
			pickI{wdepth: wdepth, wlength: wlength},
		},
	}, nil
}

func (c *Compiler) compileLiteral(ctx context.Context, x myc.Value) (*Prog, error) {
	avData, err := myc.SaveRoot(ctx, c.store, myc.NewAnyValue(x))
	if err != nil {
		return nil, err
	}
	var av AnyValue
	av.FromBytes(avData)

	ref := av.GetRef()
	ty, err := c.loadType(ctx, av.GetType())
	if err != nil {
		return nil, err
	}
	data := make([]Word, ty.SizeWords())
	if err := c.loadWords(ctx, ref, data); err != nil {
		return nil, err
	}
	size := x.Type().SizeOf()
	return &Prog{
		Type: ty,
		I: []I{
			pushRefI{x: ref},
			loadI{outputBits: size},
		},
		Value: data,
	}, nil
}

// compileSimple compiles the simple operations.
// A simple operation doesn't modify the parameters in scope, and computes all of it's inputs.
func (c *Compiler) compileSimple(ctx context.Context, mc machCtx, code spec.Op, argTypes [4]Type, argValues [4][]Word) (*Prog, error) {
	switch code {
	case spec.Self:
		if mc.self.IsZero() {
			return nil, fmt.Errorf("no self in this context")
		}
		outTy, err := c.makeType(ctx, mc.self.t2, mc.self.typeData)
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: outTy,
			I:    []I{selfI{}},
		}, nil
	case spec.Pass:
		return &Prog{Type: argTypes[0]}, nil
	case spec.Uncraft:
		return c.compileUncraft(ctx, argTypes[0])
	case spec.SizeOf:
		if argTypes[0].Type2.TypeCode() != spec.TC_AnyType {
			return nil, fmt.Errorf("sizeOf takes AnyType. HAVE: %v", argTypes[0])
		}
		ty, err := c.arrayType(ctx, c.bitType(), 32)
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: ty,
			I:    []I{sizeOfI{}},
		}, nil
	case spec.ProductEmpty:
		pt, err := c.makeHType(ctx, spec.TC_Product, nil)
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: pt,
			I:    nil,
		}, nil
	case spec.ArrayUnit:
		at, err := c.arrayType(ctx, argTypes[0], 1)
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: at,
			// no-op
		}, nil
	case spec.ProductUnit:
		pt, err := c.makeHType(ctx, spec.TC_Product, []Type{argTypes[0]})
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: pt,
			// no-op
		}, nil
	case spec.Concat:
		return c.compileConcat(ctx, argTypes[0], argTypes[1])
	case spec.Which:
		tagOffset, err := c.sumTagOffset(ctx, &argTypes[0])
		if err != nil {
			return nil, err
		}
		sizeType, err := c.arrayType(ctx, c.bitType(), 32)
		if err != nil {
			return nil, err
		}
		inputSize := argTypes[0].Size
		beg := tagOffset
		return &Prog{
			Type: sizeType,
			I: []I{
				sliceI{
					inputBits: inputSize,
					beg:       beg,
					end:       inputSize,
				},
			},
		}, nil
	case spec.Len:
		return c.compileLen(ctx, argTypes[0])
	case spec.Slot:
		return c.compileSlot(ctx, argTypes[0], argTypes[1])

	// Store
	case spec.Load:
		if argTypes[0].Type2.TypeCode() != spec.TC_Ref {
			return nil, fmt.Errorf("load on non-ref %v", argTypes[0])
		}
		elemType, err := c.getTypeParam(ctx, &argTypes[0], 0)
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: elemType,
			I:    []I{loadI{outputBits: elemType.Size}},
		}, nil
	case spec.Post:
		salt, err := c.saltFor(ctx, argTypes[0])
		if err != nil {
			return nil, err
		}
		inputBits := argTypes[0].Size
		outTy, err := c.makeHType(ctx, spec.TC_Ref, []Type{argTypes[0]})
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: outTy,
			I:    []I{postI{salt: salt, inputBits: inputBits}},
		}, nil

	// Ports
	case spec.Output:
		outgoingType, err := c.getTypeParam(ctx, &argTypes[0], 0)
		if err != nil {
			return nil, err
		}
		unitType, err := c.makeHType(ctx, spec.TC_Product, nil)
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: unitType,
			I: []I{
				swapI{loWords: PortBits / WordBits, hiWords: argTypes[1].SizeWords()},
				portOutputI{consumeWords: outgoingType.SizeWords()},
			},
		}, nil
	case spec.Input:
		incomingType, err := c.getTypeParam(ctx, &argTypes[0], 1)
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: incomingType,
			I: []I{
				swapI{loWords: PortBits / WordBits, hiWords: argTypes[1].SizeWords()},
				portInputI{produceWords: incomingType.SizeWords()},
			},
		}, nil
	case spec.Interact:
		reqType, err := c.getTypeParam(ctx, &argTypes[0], 2)
		if err != nil {
			return nil, err
		}
		respType, err := c.getTypeParam(ctx, &argTypes[0], 3)
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: respType,
			I: []I{
				swapI{loWords: PortBits / WordBits, hiWords: argTypes[1].SizeWords()},
				portInteractI{
					consumeWords: reqType.SizeWords(),
					produceWords: respType.SizeWords(),
				},
			},
		}, nil

	case spec.Eval:
		laz := argTypes[0]
		elemAT := AnyType(laz.Data)
		ty, err := c.loadType(ctx, elemAT)
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: ty,
			I:    []I{evalI{}},
		}, nil
	case spec.Apply:
		lamType := argTypes[0]
		if lamType.Type2.TypeCode() != spec.TC_Lambda {
			return nil, fmt.Errorf("cannot apply non-lambda %v", lamType)
		}
		input := argTypes[1]
		inputType, err := c.getTypeParam(ctx, &lamType, 0)
		if err != nil {
			return nil, err
		}
		outputType, err := c.getTypeParam(ctx, &lamType, 1)
		if err != nil {
			return nil, err
		}
		ss, err := c.supersets(ctx, &inputType, &input)
		if err != nil {
			return nil, err
		}
		if !ss {
			return nil, fmt.Errorf("apply wrong input type. HAVE: %v WANT: %v", input, inputType)
		}
		if lamData := argValues[0]; len(lamData) > 0 {
			lam := Lambda(lamData)
			lt := LambdaType(lamType.Data)
			fp := lam.Fingerprint(&lt)
			if fn, exists := c.accels[fp]; exists {
				return &Prog{
					Type: outputType,
					I: []I{
						swapI{loWords: lamType.SizeWords(), hiWords: input.SizeWords()},
						discardI{words: LambdaBits / WordBits},
						accelI{
							inputWords:  inputType.SizeWords(),
							outputWords: outputType.SizeWords(),
							fn:          fn,
						},
					},
				}, nil
			}
		}
		return &Prog{
			Type: outputType,
			I: []I{
				swapI{loWords: lamType.SizeWords(), hiWords: input.SizeWords()},
				applyI{
					lambdaType:  LambdaType(lamType.Data),
					inputWords:  inputType.SizeWords(),
					outputWords: outputType.SizeWords(),
				},
			},
		}, nil

	case spec.Panic:
		bt, err := c.makeHType(ctx, spec.TC_Sum, nil)
		if err != nil {
			return nil, err
		}
		inputType := argTypes[0]
		needsSalt, err := NeedsSalt(ctx, c.store, inputType.AsAnyType())
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: bt,
			I: []I{
				anyValueFromI{t2: inputType.Type2, tyRef: inputType.Ref, needsSalt: needsSalt, inputBits: inputType.Size},
				panicI{},
			},
		}, nil
	case spec.Equal:
		a := argTypes[0]
		b := argTypes[1]
		if !a.Equals(&b) {
			return &Prog{
				Type: c.bitType(),
				I:    []I{pushI{x: 0}},
			}, nil
		} else {
			return &Prog{
				Type: c.bitType(),
				I:    equal(a.Size),
			}, nil
		}
	case spec.Root:
		outTy, err := c.makeType(ctx, newType2(spec.TC_Kind, 0), []Word{0})
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: outTy,
			I:    []I{pushI{x: 0}},
		}, nil

	// Bits
	case spec.ZERO, spec.ONE:
		var fn I
		var data []Word
		if code == spec.ONE {
			fn = pushI{x: 1}
			data = []Word{1}
		} else {
			fn = pushI{x: 0}
			data = []Word{0}
		}
		return &Prog{
			Type:  c.bitType(),
			I:     []I{fn},
			Value: data,
		}, nil

	case spec.Mux:
		at, idxTy := argTypes[0], argTypes[1]
		elemTy, err := c.getTypeParam(ctx, &at, 0)
		if err != nil {
			return nil, err
		}
		aLen := at.Data[len(at.Data)-1]
		if 1<<idxTy.Size != aLen {
			return nil, fmt.Errorf("mux array len mismatch: %d != 2^%d", aLen, idxTy.Size)
		}
		if len(argValues[0]) < 1 {
			return nil, fmt.Errorf("first argument to mux must be known statically")
		}
		return &Prog{
			Type: elemTy,
			I: []I{
				muxI{table: argValues[0], elemSize: elemTy.Size},
			},
		}, nil

	// List
	case spec.ListFrom:
		if argTypes[0].Type2.TypeCode() != spec.TC_Ref {
			// TODO: also check that it is a Ref of an Array.
			return nil, fmt.Errorf("ListFrom on non Ref[Array[_, _]]: %v", argTypes[0])
		}
		refType := argTypes[0]
		arrType, err := c.getTypeParam(ctx, &refType, 0)
		if err != nil {
			return nil, err
		}
		elemTy, err := c.getTypeParam(ctx, &arrType, 0)
		if err != nil {
			return nil, err
		}
		l := int(arrType.Data[9])
		outTy, err := c.makeHType(ctx, spec.TC_List, []Type{elemTy})
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: outTy,
			I: []I{
				pushI{x: uint32(l)},
			},
		}, nil
	case spec.Gather:
		panic("Gather")

	// AnyType
	case spec.AnyTypeFrom:
		att, err := c.anyTypeType(ctx)
		if err != nil {
			return nil, err
		}
		// if the type is already an AnyType, then passthrough
		if argTypes[0].Equals(&att) {
			return &Prog{
				Type: att,
				// no-op
			}, nil
		}
		if len(argTypes[0].Data) != 1 {
			return nil, fmt.Errorf("AnyTypeFrom on non type %v", argTypes[0])
		}
		t2 := Type2{argTypes[0].Data[0]}
		size := argTypes[0].Size
		return &Prog{
			Type: att,
			I:    []I{anyTypeFromI{t2: t2, inputBits: size}},
		}, nil
	case spec.AnyTypeElemType:
		outTy, err := c.makeType(ctx, Type2{0}, nil)
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: outTy,
			I: []I{sliceI{
				inputBits: AnyTypeBits,
				beg:       RefBits,
				end:       AnyTypeBits,
			}},
		}, nil

	// AnyValue
	case spec.AnyValueFrom:
		avt, err := c.anyValueType(ctx)
		if err != nil {
			return nil, err
		}
		needsSalt, err := NeedsSalt(ctx, c.store, argTypes[0].AsAnyType())
		if err != nil {
			return nil, err
		}
		t2 := argTypes[0].Type2
		tyRef := argTypes[0].Ref
		inputSize := argTypes[0].Size
		return &Prog{
			Type: avt,
			I: []I{anyValueFromI{
				t2:        t2,
				tyRef:     tyRef,
				needsSalt: needsSalt,
				inputBits: inputSize,
			}},
		}, nil
	case spec.AnyValueElemType:
		att, err := c.anyTypeType(ctx)
		if err != nil {
			return nil, err
		}
		return &Prog{
			Type: att,
			I: []I{sliceI{
				inputBits: AnyValueBits,
				beg:       RefBits,
				end:       AnyValueBits,
			}},
		}, nil
	default:
		return nil, fmt.Errorf("unrecognized op %v", code)
	}
}

func (c *Compiler) compileUncraft(ctx context.Context, x Type) (*Prog, error) {
	var outType Type
	isType := func(kc spec.TypeCode) bool {
		return x.Type2.TypeCode() == spec.TC_Kind && spec.TypeCode(x.Data[0]) == kc
	}
	switch {
	case isType(spec.TC_Array):
		att, err := c.anyTypeType(ctx)
		if err != nil {
			return nil, err
		}
		sizeType, err := c.sizeType(ctx)
		if err != nil {
			return nil, err
		}
		ty, err := c.makeHType(ctx, spec.TC_Product, []Type{att, sizeType})
		if err != nil {
			return nil, err
		}
		outType = ty
	case isType(spec.TC_Prog):
		sizeType, err := c.sizeType(ctx)
		if err != nil {
			return nil, err
		}
		ty, err := c.makeHType(ctx, spec.TC_Product, []Type{sizeType})
		if err != nil {
			return nil, err
		}
		outType = ty
	case isType(spec.TC_Ref) || isType(spec.TC_List) || isType(spec.TC_Lazy):
		att, err := c.anyTypeType(ctx)
		if err != nil {
			return nil, err
		}
		ty, err := c.makeHType(ctx, spec.TC_Product, []Type{att})
		if err != nil {
			return nil, err
		}
		outType = ty
	case isType(spec.TC_Lambda):
		att, err := c.anyTypeType(ctx)
		if err != nil {
			return nil, err
		}
		ty, err := c.makeHType(ctx, spec.TC_Product, []Type{att, att})
		if err != nil {
			return nil, err
		}
		outType = ty
	case isType(spec.TC_Port):
		att, err := c.anyTypeType(ctx)
		if err != nil {
			return nil, err
		}
		ty, err := c.makeHType(ctx, spec.TC_Product, []Type{att, att, att, att})
		if err != nil {
			return nil, err
		}
		outType = ty
	case isType(spec.TC_Sum) || isType(spec.TC_Product):
		att, err := c.anyTypeType(ctx)
		if err != nil {
			return nil, err
		}
		ty, err := c.arrayType(ctx, att, int(x.Type2.Data()))
		if err != nil {
			return nil, err
		}
		outType = ty
	case isType(spec.TC_Distinct):
		att, err := c.anyTypeType(ctx)
		if err != nil {
			return nil, err
		}
		avt, err := c.anyValueType(ctx)
		if err != nil {
			return nil, err
		}
		ty, err := c.makeHType(ctx, spec.TC_Product, []Type{att, avt})
		if err != nil {
			return nil, err
		}
		outType = ty
	default:
		return nil, fmt.Errorf("unmake(%v)", x)
	}
	return &Prog{
		Type: outType,
		// Unmake is a no-op,
	}, nil
}

func (c *Compiler) compileConcat(ctx context.Context, left, right Type) (*Prog, error) {
	if left.Type2.TypeCode() != right.Type2.TypeCode() {
		return nil, fmt.Errorf("concat requires same type2 for operands")
	}
	var outType Type
	kc := left.Type2.TypeCode()
	switch kc {
	case spec.TC_Array:
		leftArrTy := ArrayType(left.Data)
		leftElemTy, err := c.loadType(ctx, leftArrTy.Elem())
		if err != nil {
			return nil, err
		}
		rightArrTy := ArrayType(right.Data)
		rightElemTy, err := c.loadType(ctx, rightArrTy.Elem())
		if err != nil {
			return nil, err
		}
		if !leftElemTy.Equals(&rightElemTy) {
			return nil, fmt.Errorf("concat arrays with different element types left=%v right=%v", leftElemTy, rightElemTy)
		}
		ty, err := c.arrayType(ctx, leftElemTy, leftArrTy.Len()+rightArrTy.Len())
		if err != nil {
			return nil, err
		}
		outType = ty
	case spec.TC_Product:
		leftPT := ProductType(left.Data)
		rightPT := ProductType(right.Data)
		var outPT ProductType
		outPT = append(outPT, leftPT...)
		outPT = append(outPT, rightPT...)
		t2 := newType2(spec.TC_Product, uint32(outPT.Len()))
		ty, err := c.makeType(ctx, t2, outPT)
		if err != nil {
			return nil, err
		}
		outType = ty
	default:
		return nil, fmt.Errorf("compileConcat(%v %v)", left, right)
	}
	leftBits, rightBits := left.Size, right.Size
	return &Prog{
		Type: outType,
		I:    []I{concatI{leftBits: leftBits, rightBits: rightBits}},
	}, nil
}

func (c *Compiler) compileField(ctx context.Context, mc machCtx, xE, idxE myc.Prog) (*Prog, error) {
	x, err := c.compile(ctx, mc, xE)
	if err != nil {
		return nil, err
	}
	idx, err := c.compile(ctx, mc, idxE)
	if err != nil {
		return nil, err
	}
	sizeType, err := c.sizeType(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.checkSupersets(ctx, &sizeType, &idx.Type); err != nil {
		return nil, err
	}
	if !idx.IsConstant() {
		return nil, fmt.Errorf("field index must be a known constant")
	}
	inputSize := x.Type.Size
	kc := x.Type.Type2.TypeCode()
	switch kc {
	case spec.TC_Sum:
		i := int(idx.Value[0])
		fieldType, err := c.getTypeParam(ctx, &x.Type, i)
		if err != nil {
			return nil, err
		}
		fieldSize := fieldType.Size
		return &Prog{
			Type: fieldType,
			I: seq(
				x.I,
				[]I{sliceI{
					inputBits: inputSize,
					beg:       0,
					end:       fieldSize,
				}},
			),
		}, nil
	case spec.TC_Product:
		offsets, err := c.productOffsets(ctx, &x.Type)
		if err != nil {
			return nil, err
		}
		i := int(idx.Value[0])
		if i >= len(offsets) {
			return nil, fmt.Errorf("get.product n=%v idx=%v", len(offsets), i)
		}
		beg := offsets[i]
		fieldTy, err := c.getTypeParam(ctx, &x.Type, i)
		if err != nil {
			return nil, err
		}
		end := beg + fieldTy.Size
		return &Prog{
			Type: fieldTy,
			I: seq(
				x.I,
				[]I{sliceI{
					inputBits: inputSize,
					beg:       beg,
					end:       end,
				}},
			),
		}, nil
	default:
		return nil, fmt.Errorf("field on %v", x.Type)
	}
}

func (c *Compiler) compileSlot(ctx context.Context, x Type, idx Type) (*Prog, error) {
	st, err := c.sizeType(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.checkSupersets(ctx, &idx, &st); err != nil {
		return nil, err
	}
	switch x.Type2.TypeCode() {
	case spec.TC_Array:
		at := ArrayType(x.Data)
		elemType, err := c.loadType(ctx, at.Elem())
		if err != nil {
			return nil, err
		}
		arrayLen := at.Len()
		elemSize := elemType.Size
		return &Prog{
			Type: Type{Size: elemSize},
			I: []I{arrayGetI{
				len:      arrayLen,
				elemSize: elemSize,
			}},
		}, nil
	case spec.TC_List:
		lt := ListType(x.Data)
		elemType, err := c.loadType(ctx, lt.Elem())
		if err != nil {
			return nil, err
		}
		elemSize := elemType.Size
		return &Prog{
			Type: elemType,
			I:    []I{listGetI{elemSize: elemSize}},
		}, nil
	default:
		return nil, fmt.Errorf("slot on %v", x)
	}
}

func (c *Compiler) compileLen(ctx context.Context, x Type) (*Prog, error) {
	sizeType, err := c.sizeType(ctx)
	if err != nil {
		return nil, err
	}
	var l int
	switch x.Type2.TypeCode() {
	case spec.TC_Array:
		at := ArrayType(x.Data)
		l = at.Len()
	case spec.TC_Sum:
		st := SumType(x.Data)
		l = st.Len()
	case spec.TC_Product:
		pt := ProductType(x.Data)
		l = pt.Len()
	case spec.TC_List:
		return &Prog{
			Type: sizeType,
			I: []I{sliceI{
				inputBits: ListBits,
				beg:       RefBits,
				end:       ListBits,
			}},
		}, nil
	default:
		return nil, fmt.Errorf("len on %v", x)
	}
	return &Prog{
		Type: sizeType,
		I: []I{
			discardI{words: x.SizeWords()},
			pushI{x: uint32(l)},
		},
	}, nil
}

func (c *Compiler) saltFor(ctx context.Context, x Type) (*Ref, error) {
	if yes, err := NeedsSalt(ctx, c.store, x.AsAnyType()); err != nil {
		return nil, err
	} else if !yes {
		return nil, nil
	}
	if x.Ref == (Ref{}) {
		panic(x)
	}
	return &x.Ref, nil
}

func (c *Compiler) loadWords(ctx context.Context, ref Ref, ws []Word) error {
	return loadWords(ctx, c.store, ref, ws)
}

func (c *Compiler) postWords(ctx context.Context, salt *Ref, size int, ws []Word) (Ref, error) {
	return postWords(ctx, c.store, salt, size, ws)
}

type machCtx struct {
	// stack is the data on the current stack.
	// Lambdas are compiled in the context of a stack
	stack []Word
	// layout describes the type of data currently on the stack
	layout []Type
	// store is the machine's store, which is read-only for the purposes of machCtx
	store cadata.Getter
	// scratch is the number of words before the indexed parameter
	scratch []int
	self    dynValue
}

func (mc machCtx) stackPush(ty Type) machCtx {
	// must be non-pointer receiver
	mc.layout = append(mc.layout, ty)
	for i := 0; i < ty.SizeWords(); i++ {
		mc.stack = append(mc.stack, 13) // fill in unknown value with 13s
	}
	mc.scratch = append(mc.scratch, 0)
	return mc
}

func (mc *machCtx) addScratch(x int) {
	if len(mc.scratch) > 0 {
		mc.scratch[len(mc.scratch)-1] += x
	}
}

// closure replaces params >= level or errors
func (mc machCtx) closure(ctx context.Context, level uint32, bindSelf bool, x myc.Prog) (myc.Prog, error) {
	pctx := myc.ProgCtx{
		Self: func() (myc.Node, error) {
			if mc.self.IsZero() {
				return myc.Node{}, fmt.Errorf("no self in context")
			}
			return myc.Literal(mc.self.AsMycelium(ctx, mc.store)), nil
		},
		Lookup: func(i uint32) (myc.Node, error) {
			wdepth, pty, err := mc.pick(i)
			if err != nil {
				return myc.Node{}, err
			}
			wlength := pty.SizeWords()
			l := len(mc.stack)
			data := mc.stack[l-wdepth : l-wdepth+wlength]
			dv := dynValue{
				t2:       pty.Type2,
				typeData: pty.Data,
				valData:  data,
			}
			return myc.Literal(dv.AsMycelium(ctx, mc.store)), nil
		},
	}
	return myc.Closure(nil, pctx, level, bindSelf, x)
}

func (mc machCtx) pick(param uint32) (depth int, ty Type, _ error) {
	if int(param) >= len(mc.layout) {
		return 0, Type{}, fmt.Errorf("cannot access param %d", param)
	}
	idx := len(mc.layout) - 1 - int(param)
	paramType := mc.layout[idx]
	var wdepth int
	for i := len(mc.layout) - 1; i >= idx; i-- {
		wdepth += mc.layout[i].SizeWords()
		wdepth += mc.scratch[i]
	}
	if wdepth > len(mc.stack)+mc.scratch[len(mc.scratch)-1] {
		return 0, Type{}, fmt.Errorf("depth=%v stack length=%v", wdepth, len(mc.stack))
	}
	return wdepth, paramType, nil
}

func seq(xs ...[]I) (ret []I) {
	for _, x := range xs {
		ret = append(ret, x...)
	}
	return ret
}

func equal(size int) []I {
	wsize := AlignSize(size) / WordBits
	if size == 0 {
		return []I{pushI{x: 1}}
	}
	return []I{equalI{words: wsize}}
}

func mkBranch(b0, b1 []I) (ret []I) {
	ret = append(ret, branchI{
		offset: uint32(len(b0)) + 1,
	})
	ret = append(ret, b0...)
	ret = append(ret, jumpI{
		offset: uint32(len(b1)),
	})
	ret = append(ret, b1...)
	return ret
}

type Loadable interface {
	FromBytes([]byte)
	Size() int
}

// Load loads data from a store
func Load[T Loadable](ctx context.Context, s cadata.Getter, ref Ref, dst T) error {
	cid := ref.CID()
	buf := make([]byte, divCeil(dst.Size(), 8))
	n, err := s.Get(ctx, &cid, nil, buf)
	if err != nil {
		return err
	}
	data := buf[:n]
	if n != len(buf) {
		return fmt.Errorf("load: short read for %T.\nHAVE: %v, len=%d\nWANT: len=%d", dst, data, n, len(buf))
	}
	dst.FromBytes(data)
	return nil
}
