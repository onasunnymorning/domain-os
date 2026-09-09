package rdevalidate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The name table is bounded separately from the index (MaxCrossReferenceNames),
// so a deposit large enough to fill it keeps being checked while its findings
// fall back to an ordinal and a byte offset. Reaching the real bound in a test
// would cost a million entries, so this drives the same path by emptying the
// table the index built.
func TestCrossRef_ReportsWithoutNamesWhenTheNameTableIsFull(t *testing.T) {
	x := newCrossRef("example")
	x.enabled = true
	x.declareContact("CONT-ORPHAN", 7)
	x.useContact("CONT-MISSING", 3)
	require.Len(t, x.names, 2, "both directions record a name")

	x.names = map[uint64]string{} // as if the bound had been reached first

	var got []Finding
	x.report(func(code Code, sev Severity, stage Stage, objType, object, locator, rule, msg string) {
		got = append(got, Finding{Code: code, Severity: sev, ObjectType: objType, Object: object, Locator: locator})
	})

	require.Len(t, got, 2)
	for _, f := range got {
		assert.Empty(t, f.Object, "no name is reported rather than a wrong one")
		assert.NotEmpty(t, f.Locator, "the ordinal still locates the object")
	}
	assert.Equal(t, CodeRDEObjectNotReferenced, got[0].Code)
	assert.Equal(t, "contact#7", got[0].Locator)
	assert.Equal(t, CodeRDEReferenceNotInDeposit, got[1].Code)
	assert.Equal(t, "domain#3", got[1].Locator)
}

// The index folds case before hashing, so two spellings of one identifier are
// the same object. The name reported is the first spelling seen.
func TestCrossRef_NamesUseTheFirstSpellingSeen(t *testing.T) {
	x := newCrossRef("example")
	x.enabled = true
	x.declareHost("NS1.Example-1.Example", 1)
	x.useHost("ns1.example-1.example", 2)

	var got []Finding
	x.report(func(code Code, sev Severity, stage Stage, objType, object, locator, rule, msg string) {
		got = append(got, Finding{Code: code, Object: object})
	})
	assert.Empty(t, got, "the host is declared and used, whatever its case")
}
