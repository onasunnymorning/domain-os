package rdevalidate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The name table is bounded separately from the index (maxNames),
// so a deposit large enough to fill it keeps being checked while its findings
// fall back to an ordinal and a byte offset. Reaching the real bound in a test
// would cost a million entries, so this drives the same path by emptying the
// table the index built.
func TestCrossRef_ReportsWithoutNamesWhenTheNameTableIsFull(t *testing.T) {
	x := newCrossRef("example", 0, 0)
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
	x := newCrossRef("example", 0, 0)
	x.enabled = true
	x.declareHost("NS1.Example-1.Example", 1)
	x.useHost("ns1.example-1.example", 2)

	var got []Finding
	x.report(func(code Code, sev Severity, stage Stage, objType, object, locator, rule, msg string) {
		got = append(got, Finding{Code: code, Object: object})
	})
	assert.Empty(t, got, "the host is declared and used, whatever its case")
}

// The bound is per-run configuration now, so the give-up path has to be
// reachable without building five million entries — and the number the finding
// quotes has to be the one that was actually in force, not a constant.
func TestCrossRef_HonoursTheConfiguredObjectBound(t *testing.T) {
	x := newCrossRef("example", 2, 0)
	x.enabled = true
	x.declareContact("CONT1", 1)
	x.declareContact("CONT2", 2)
	x.declareContact("CONT3", 3) // one past the bound
	assert.True(t, x.overflowed)

	var got []Finding
	x.report(func(code Code, sev Severity, stage Stage, objType, object, locator, rule, msg string) {
		got = append(got, Finding{Code: code, Severity: sev, Rule: rule, Message: msg})
	})
	require.Len(t, got, 1, "the check reports that it did not run, and nothing else")
	assert.Equal(t, CodeRDECrossReferenceSkipped, got[0].Code)
	assert.Equal(t, SeverityWarning, got[0].Severity)
	assert.Contains(t, got[0].Rule, "2 cross-referenced identifiers", "the bound in force, not the default")
}

func TestCrossRef_HonoursTheConfiguredNameBound(t *testing.T) {
	x := newCrossRef("example", 0, 1)
	x.enabled = true
	x.declareContact("CONT-FIRST", 1)
	x.declareContact("CONT-SECOND", 2)
	require.Len(t, x.names, 1, "the second identifier is counted and compared, but not remembered")

	var named, unnamed int
	x.report(func(code Code, sev Severity, stage Stage, objType, object, locator, rule, msg string) {
		if object == "" {
			unnamed++
			return
		}
		named++
	})
	assert.Equal(t, 1, named)
	assert.Equal(t, 1, unnamed, "past the name bound a finding carries its ordinal alone")
}

// A bound of zero is what every caller without an opinion passes, including
// the derivative revalidation in the sanitize activities.
func TestCrossRef_ZeroBoundMeansTheDefault(t *testing.T) {
	x := newCrossRef("example", 0, 0)
	assert.Equal(t, DefaultMaxCrossReferenceObjects, x.maxObjects)
	assert.Equal(t, DefaultMaxCrossReferenceNames, x.maxNames)
}

// The default exists to make a real TLD checkable, so the arithmetic behind it
// is written down here rather than only in a comment. .co's deposit carries
// 8,606,233 contacts against 3,358,258 domains: contacts are not shared
// between domains, so each is indexed twice — once declared, once referenced —
// and the same holds for its hosts. Lower the default below this and .co stops
// being cross-checked again.
func TestDefaultObjectBoundCoversARealLargeTLD(t *testing.T) {
	const coContacts, coHosts = 8_606_233, 112_844
	entries := 2*coContacts + 2*coHosts
	assert.LessOrEqual(t, entries, DefaultMaxCrossReferenceObjects,
		"the default no longer covers the deposit it was chosen for (%d entries)", entries)
}
