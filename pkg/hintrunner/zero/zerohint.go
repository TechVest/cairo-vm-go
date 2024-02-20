package zero

import (
	"fmt"
	"strconv"

	"github.com/NethermindEth/cairo-vm-go/pkg/hintrunner/core"
	"github.com/NethermindEth/cairo-vm-go/pkg/hintrunner/hinter"
	zero "github.com/NethermindEth/cairo-vm-go/pkg/parsers/zero"
	"github.com/NethermindEth/cairo-vm-go/pkg/utils"
	VM "github.com/NethermindEth/cairo-vm-go/pkg/vm"
	"github.com/NethermindEth/cairo-vm-go/pkg/vm/memory"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

// GenericZeroHinter wraps an adhoc Cairo0 inline (pythonic) hint implementation.
type GenericZeroHinter struct {
	Name string
	Op   func(vm *VM.VirtualMachine, _ *hinter.HintRunnerContext) error
}

func (hint *GenericZeroHinter) String() string {
	return hint.Name
}

func (hint *GenericZeroHinter) Execute(vm *VM.VirtualMachine, ctx *hinter.HintRunnerContext) error {
	return hint.Op(vm, ctx)
}

func GetZeroHints(cairoZeroJson *zero.ZeroProgram) (map[uint64][]hinter.Hinter, error) {
	hints := make(map[uint64][]hinter.Hinter)
	for counter, rawHints := range cairoZeroJson.Hints {
		pc, err := strconv.ParseUint(counter, 10, 64)
		if err != nil {
			return nil, err
		}

		for _, rawHint := range rawHints {
			hint, err := GetHintFromCode(cairoZeroJson, rawHint, pc)
			if err != nil {
				return nil, err
			}

			hints[pc] = append(hints[pc], hint)
		}
	}

	return hints, nil
}

func GetHintFromCode(program *zero.ZeroProgram, rawHint zero.Hint, hintPC uint64) (hinter.Hinter, error) {
	resolver, err := getParameters(program, rawHint, hintPC)
	if err != nil {
		return nil, err
	}

	switch rawHint.Code {
	case allocSegmentCode:
		return CreateAllocSegmentHinter(resolver)
	case isLeFeltCode:
		return createIsLeFeltHinter(resolver)
	case assertLtFeltCode:
		return createAssertLtFeltHinter(resolver)
	case testAssignCode:
		return createTestAssignHinter(resolver)
	case assertLeFeltCode:
		return createAssertLeFeltHinter(resolver)
	case assertLeFeltExcluded0Code:
		return createAssertLeFeltExcluded0Hinter(resolver)
	case assertLeFeltExcluded1Code:
		return createAssertLeFeltExcluded1Hinter(resolver)
	case assertLeFeltExcluded2Code:
		return createAssertLeFeltExcluded2Hinter(resolver)
	case isNNCode:
		return createIsNNHinter(resolver)
	case isNNOutOfRangeCode:
		return createIsNNOutOfRangeHinter(resolver)
	default:
		return nil, fmt.Errorf("Not identified hint")
	}
}

func CreateAllocSegmentHinter(resolver hintReferenceResolver) (hinter.Hinter, error) {
	return &core.AllocSegment{Dst: hinter.ApCellRef(0)}, nil
}

func createIsLeFeltHinter(resolver hintReferenceResolver) (hinter.Hinter, error) {
	argA, err := resolver.GetResOperander("a")
	if err != nil {
		return nil, err
	}
	argB, err := resolver.GetResOperander("b")
	if err != nil {
		return nil, err
	}

	h := &GenericZeroHinter{
		Name: "IsLeFelt",
		Op: func(vm *VM.VirtualMachine, _ *hinter.HintRunnerContext) error {
			//> memory[ap] = 0 if (ids.a % PRIME) <= (ids.b % PRIME) else 1
			apAddr := vm.Context.AddressAp()

			a, err := argA.Resolve(vm)
			if err != nil {
				return err
			}
			aFelt, err := a.FieldElement()
			if err != nil {
				return err
			}
			b, err := argB.Resolve(vm)
			if err != nil {
				return err
			}
			bFelt, err := b.FieldElement()
			if err != nil {
				return err
			}

			var v memory.MemoryValue
			if utils.FeltLe(aFelt, bFelt) {
				v = memory.MemoryValueFromFieldElement(&utils.FeltZero)
			} else {
				v = memory.MemoryValueFromFieldElement(&utils.FeltOne)
			}
			return vm.Memory.WriteToAddress(&apAddr, &v)
		},
	}
	return h, nil
}

func createAssertLtFeltHinter(resolver hintReferenceResolver) (hinter.Hinter, error) {
	argA, err := resolver.GetResOperander("a")
	if err != nil {
		return nil, err
	}
	argB, err := resolver.GetResOperander("b")
	if err != nil {
		return nil, err
	}

	h := &GenericZeroHinter{
		Name: "AssertLtFelt",
		Op: func(vm *VM.VirtualMachine, _ *hinter.HintRunnerContext) error {
			//> from starkware.cairo.common.math_utils import assert_integer
			//> assert_integer(ids.a)
			//> assert_integer(ids.b)
			//> assert (ids.a % PRIME) < (ids.b % PRIME),
			//>        f'a = {ids.a % PRIME} is not less than b = {ids.b % PRIME}.'
			a, err := argA.Resolve(vm)
			if err != nil {
				return err
			}
			aFelt, err := a.FieldElement()
			if err != nil {
				return err
			}
			b, err := argB.Resolve(vm)
			if err != nil {
				return err
			}
			bFelt, err := b.FieldElement()
			if err != nil {
				return err
			}

			if !utils.FeltLt(aFelt, bFelt) {
				return fmt.Errorf("a = %v is not less than b = %v", aFelt, bFelt)
			}
			return nil
		},
	}
	return h, nil
}

func createTestAssignHinter(resolver hintReferenceResolver) (hinter.Hinter, error) {
	arg, err := resolver.GetReference("a")
	if err != nil {
		return nil, err
	}

	a, ok := arg.(hinter.ResOperander)
	if !ok {
		return nil, fmt.Errorf("expected a ResOperander reference")
	}

	h := &GenericZeroHinter{
		Name: "TestAssign",
		Op: func(vm *VM.VirtualMachine, _ *hinter.HintRunnerContext) error {
			apAddr := vm.Context.AddressAp()
			v, err := a.Resolve(vm)
			if err != nil {
				return err
			}
			return vm.Memory.WriteToAddress(&apAddr, &v)
		},
	}
	return h, nil
}

func createAssertLeFeltHinter(resolver hintReferenceResolver) (hinter.Hinter, error) {
	a, err := resolver.GetResOperander("a")
	if err != nil {
		return nil, err
	}
	b, err := resolver.GetResOperander("b")
	if err != nil {
		return nil, err
	}
	rangeCheckPtr, err := resolver.GetResOperander("range_check_ptr")
	if err != nil {
		return nil, err
	}

	h := &core.AssertLeFindSmallArc{
		A:             a,
		B:             b,
		RangeCheckPtr: rangeCheckPtr,
	}
	return h, nil
}

func createAssertLeFeltExcluded0Hinter(resolver hintReferenceResolver) (hinter.Hinter, error) {
	return &core.AssertLeIsFirstArcExcluded{SkipExcludeAFlag: hinter.ApCellRef(0)}, nil
}

func createAssertLeFeltExcluded1Hinter(resolver hintReferenceResolver) (hinter.Hinter, error) {
	return &core.AssertLeIsSecondArcExcluded{SkipExcludeBMinusA: hinter.ApCellRef(0)}, nil
}

func createAssertLeFeltExcluded2Hinter(resolver hintReferenceResolver) (hinter.Hinter, error) {
	// This hint is Cairo0-specific.
	// It only does a python-scoped variable named "excluded" assert.
	// We store that variable inside a hinter context.
	h := &GenericZeroHinter{
		Name: "AssertLeFeltExcluded2",
		Op: func(vm *VM.VirtualMachine, ctx *hinter.HintRunnerContext) error {
			if ctx.ExcludedArc != 2 {
				return fmt.Errorf("assertion `excluded == 2` failed")
			}
			return nil
		},
	}
	return h, nil
}

func createIsNNHinter(resolver hintReferenceResolver) (hinter.Hinter, error) {
	argA, err := resolver.GetResOperander("a")
	if err != nil {
		return nil, err
	}

	h := &GenericZeroHinter{
		Name: "IsNN",
		Op: func(vm *VM.VirtualMachine, _ *hinter.HintRunnerContext) error {
			apAddr := vm.Context.AddressAp()
			//> memory[ap] = 0 if 0 <= (ids.a % PRIME) < range_check_builtin.bound else 1
			a, err := argA.Resolve(vm)
			if err != nil {
				return err
			}
			// aFelt is already modulo PRIME, no need to adjust it.
			aFelt, err := a.FieldElement()
			if err != nil {
				return err
			}
			// range_check_builtin.bound is utils.FeltMax128 (1 << 128).
			var v memory.MemoryValue
			if utils.FeltLt(aFelt, &utils.FeltMax128) {
				v = memory.MemoryValueFromFieldElement(&utils.FeltZero)
			} else {
				v = memory.MemoryValueFromFieldElement(&utils.FeltOne)
			}
			return vm.Memory.WriteToAddress(&apAddr, &v)
		},
	}
	return h, nil
}

func createIsNNOutOfRangeHinter(resolver hintReferenceResolver) (hinter.Hinter, error) {
	// This hint is executed for the negative values.
	// If the value was non-negative, it's usually handled by the IsNN hint.

	argA, err := resolver.GetResOperander("a")
	if err != nil {
		return nil, err
	}

	h := &GenericZeroHinter{
		Name: "IsNNOutOfRange",
		Op: func(vm *VM.VirtualMachine, _ *hinter.HintRunnerContext) error {
			apAddr := vm.Context.AddressAp()
			//> memory[ap] = 0 if 0 <= ((-ids.a - 1) % PRIME) < range_check_builtin.bound else 1
			a, err := argA.Resolve(vm)
			if err != nil {
				return err
			}
			aFelt, err := a.FieldElement()
			if err != nil {
				return err
			}
			var lhs fp.Element
			lhs.Sub(&utils.FeltZero, aFelt) //> -ids.a
			lhs.Sub(&lhs, &utils.FeltOne)
			var v memory.MemoryValue
			if utils.FeltLt(aFelt, &utils.FeltMax128) {
				v = memory.MemoryValueFromFieldElement(&utils.FeltZero)
			} else {
				v = memory.MemoryValueFromFieldElement(&utils.FeltOne)
			}
			return vm.Memory.WriteToAddress(&apAddr, &v)
		},
	}
	return h, nil
}

func getParameters(zeroProgram *zero.ZeroProgram, hint zero.Hint, hintPC uint64) (hintReferenceResolver, error) {
	resolver := NewReferenceResolver()

	for referenceName := range hint.FlowTrackingData.ReferenceIds {
		rawIdentifier, ok := zeroProgram.Identifiers[referenceName]
		if !ok {
			return resolver, fmt.Errorf("missing identifier %s", referenceName)
		}

		if len(rawIdentifier.References) == 0 {
			return resolver, fmt.Errorf("identifier %s should have at least one reference", referenceName)
		}
		references := rawIdentifier.References

		// Go through the references in reverse order to get the one with biggest pc smaller or equal to the hint pc
		var reference zero.Reference
		ok = false
		for i := len(references) - 1; i >= 0; i-- {
			if references[i].Pc <= hintPC {
				reference = references[i]
				ok = true
				break
			}
		}
		if !ok {
			return resolver, fmt.Errorf("identifier %s should have a reference with pc smaller or equal than %d", referenceName, hintPC)
		}

		param, err := ParseIdentifier(reference.Value)
		if err != nil {
			return resolver, err
		}
		param = param.ApplyApTracking(hint.FlowTrackingData.ApTracking, reference.ApTrackingData)
		if err := resolver.AddReference(referenceName, param); err != nil {
			return resolver, err
		}
	}

	return resolver, nil
}
