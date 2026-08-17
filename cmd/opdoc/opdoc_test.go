// Copyright (C) 2019-2026 Algorand, Inc.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public
// License along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/algorand/go-algorand/test/partitiontest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fieldLine returns a source line that assembles op with the given field
// name in its field immediate, and a 0 for every other immediate (they are
// all uint8 indexes on the opcodes that carry a field group).
func fieldLine(spec logic.OpSpec, field string) string {
	tokens := []string{spec.Name}
	for i := range spec.OpDetails.Immediates {
		if spec.OpDetails.Immediates[i].Group != nil {
			tokens = append(tokens, field)
		} else {
			tokens = append(tokens, "0")
		}
	}
	return strings.Join(tokens, " ")
}

// TestArgEnumsMatchAssembler pins the semantics of the langspec ArgEnum: a
// field appears in an opcode's enum at a version exactly when the assembler
// accepts it there. The enum is derived from the field group on the opcode's
// immediate, but argEnumOverrides and sparse group name arrays can pull the
// documented set away from the group, and nothing else checks that they only
// ever pull it toward what actually assembles.
func TestArgEnumsMatchAssembler(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	// typetrack is off so a lone opcode assembles without its stack inputs.
	// Field immediates are still fully checked.
	assemble := func(line string, v uint64) error {
		_, err := logic.AssembleStringWithVersion("#pragma typetrack false\n"+line, v)
		return err
	}

	for v := uint64(1); v <= docVersion; v++ {
		for _, spec := range logic.OpcodesByVersion(v) {
			var group *logic.FieldGroup
			for i := range spec.OpDetails.Immediates {
				if g := spec.OpDetails.Immediates[i].Group; g != nil {
					group = g
					break
				}
			}
			if group == nil {
				continue
			}

			enum, _, _ := argEnums(spec, v)
			documented := make(map[string]bool, len(enum))
			for _, name := range enum {
				documented[name] = true
			}

			// everything documented assembles
			for _, name := range enum {
				line := fieldLine(spec, name)
				assert.NoError(t, assemble(line, v), "%q is documented at v%d but does not assemble", line, v)
			}

			// nothing else in the group does. The group's names are the
			// universe: a name absent from it (or blanked out of a sparse
			// names array) cannot be written in source at all.
			for _, name := range group.Names {
				if name == "" || documented[name] {
					continue
				}
				fs, ok := group.SpecByName(name)
				if !ok {
					continue
				}
				if fs.Version() <= v && spec.Modes&fs.Modes() == 0 {
					// left out of the enum only because the field's modes do
					// not overlap the op's; the assembler does not track modes
					continue
				}
				line := fieldLine(spec, name)
				require.Error(t, assemble(line, v), "%q assembles at v%d but is not documented", line, v)
			}
		}
	}
}

// TestFieldLine spot-checks the scaffolding above against opcodes whose
// immediates surround the field with plain uint8 indexes.
func TestFieldLine(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	byName := make(map[string]logic.OpSpec)
	for _, spec := range logic.OpcodesByVersion(docVersion) {
		byName[spec.Name] = spec
	}
	require.Equal(t, "gtxn 0 Sender", fieldLine(byName["gtxn"], "Sender"))
	require.Equal(t, "txna Accounts 0", fieldLine(byName["txna"], "Accounts"))
	require.Equal(t, "gtxna 0 Accounts 0", fieldLine(byName["gtxna"], "Accounts"))
	require.Equal(t, fmt.Sprintf("ec_add %s", "BN254g1"), fieldLine(byName["ec_add"], "BN254g1"))
}
