package rdeschema

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testVocab(t *testing.T) vocabulary {
	t.Helper()
	v, err := loadVocabulary()
	require.NoError(t, err)
	return v
}

func TestPrefixFor(t *testing.T) {
	assert.Equal(t, "rdeDomain", prefixFor("urn:ietf:params:xml:ns:rdeDomain-1.0"))
	assert.Equal(t, "secDNS", prefixFor("urn:ietf:params:xml:ns:secDNS-1.1"))
	assert.Equal(t, "domain", prefixFor("urn:ietf:params:xml:ns:domain-1.0"))
}

// Every namespace in the embedded set must get a distinct prefix, or two
// namespaces would be indistinguishable in a finding.
func TestVocabulary_NamespacesHaveDistinctPrefixes(t *testing.T) {
	v := testVocab(t)
	require.NotEmpty(t, v.namespaces)
	seen := map[string]string{}
	for ns, p := range v.namespaces {
		require.NotEmpty(t, p, ns)
		other, dup := seen[p]
		assert.False(t, dup, "prefix %q is used by both %s and %s", p, ns, other)
		seen[p] = ns
	}
	assert.Equal(t, "rde", v.namespaces["urn:ietf:params:xml:ns:rde-1.0"])
	assert.True(t, v.names["watermark"])
	assert.True(t, v.names["rdeMenu"])
}

func TestParseLine(t *testing.T) {
	v := testVocab(t)
	const rde = "urn:ietf:params:xml:ns:rde-1.0"

	cases := []struct {
		name string
		line string
		kind lineKind
		want Violation
	}{
		{
			name: "undeclared element where the schema names the one it wants",
			line: "-:3: Schemas validity error : Element '{" + rde + "}unexpected': This element is not expected. Expected is ( {" + rde + "}watermark ).",
			kind: lineViolation,
			want: Violation{Class: ClassUnexpectedElement, Line: 3, Element: Name{Text: "rde:unexpected"}, Expected: []string{"rde:watermark"}},
		},
		{
			name: "a declared element out of order",
			line: "-:4: Schemas validity error : Element '{" + rde + "}contents': This element is not expected. Expected is ( {" + rde + "}rdeMenu ).",
			kind: lineViolation,
			want: Violation{Class: ClassUnexpectedElement, Line: 4, Element: Name{Known: true, Text: "rde:contents"}, Expected: []string{"rde:rdeMenu"}},
		},
		{
			name: "several expected",
			line: "-:9: Schemas validity error : Element '{" + rde + "}deposit': Missing child element(s). Expected is one of ( {" + rde + "}deletes, {" + rde + "}contents ).",
			kind: lineViolation,
			want: Violation{Class: ClassMissingElement, Line: 9, Element: Name{Known: true, Text: "rde:deposit"}, Expected: []string{"rde:deletes", "rde:contents"}},
		},
		{
			name: "required attribute",
			line: "-:2: Schemas validity error : Element '{" + rde + "}deposit': The attribute 'id' is required but missing.",
			kind: lineViolation,
			want: Violation{Class: ClassMissingAttribute, Line: 2, Element: Name{Known: true, Text: "rde:deposit"}, Attribute: Name{Known: true, Text: "id"}},
		},
		{
			name: "undeclared attribute",
			line: "-:2: Schemas validity error : Element '{" + rde + "}deposit', attribute 'bogus': The attribute 'bogus' is not allowed.",
			kind: lineViolation,
			want: Violation{Class: ClassUnexpectedAttribute, Line: 2, Element: Name{Known: true, Text: "rde:deposit"}, Attribute: Name{Text: "bogus"}},
		},
		{
			name: "facet violation never repeats the value",
			line: "-:2: Schemas validity error : Element '{" + rde + "}deposit', attribute 'type': [facet 'enumeration'] The value 'SECRET-VALUE' is not an element of the set {'FULL', 'INCR', 'DIFF'}.",
			kind: lineViolation,
			want: Violation{Class: ClassInvalidValue, Line: 2, Element: Name{Known: true, Text: "rde:deposit"}, Attribute: Name{Known: true, Text: "type"}, Facet: "enumeration"},
		},
		{
			name: "atomic type",
			line: "-:3: Schemas validity error : Element '{" + rde + "}watermark': 'SECRET-VALUE' is not a valid value of the atomic type 'xs:dateTime'.",
			kind: lineViolation,
			want: Violation{Class: ClassInvalidValue, Line: 3, Element: Name{Known: true, Text: "rde:watermark"}, Type: "xs:dateTime"},
		},
		{
			name: "character content",
			line: "-:3: Schemas validity error : Element '{" + rde + "}deposit': Character content other than whitespace is not allowed because the content type is 'element-only'.",
			kind: lineViolation,
			want: Violation{Class: ClassUnexpectedText, Line: 3, Element: Name{Known: true, Text: "rde:deposit"}},
		},
		{
			name: "unknown root",
			line: "-:1: Schemas validity error : Element '{urn:attacker}bundle': No matching global declaration available for the validation root.",
			kind: lineViolation,
			want: Violation{Class: ClassUndeclaredElement, Line: 1, Element: Name{Text: "{?}bundle"}},
		},
		{
			name: "a line that does not fit the grammar still counts, with nothing from it",
			line: "-:7: Schemas validity error : something libxml2 added in a later version",
			kind: lineViolation,
			want: Violation{Class: ClassOther, Line: 7},
		},
		{
			name: "parse error",
			line: "-:12: parser error : Opening and ending tag mismatch: a line 12 and b",
			kind: lineParseError,
			want: Violation{Class: ClassOther, Line: 12},
		},
		{name: "summary line", line: "- fails to validate", kind: lineIgnored},
		{name: "validates line is never evidence", line: "- validates", kind: lineIgnored},
		{name: "parse failure verdict", line: "- : failed to parse", kind: lineParseFail},
		{name: "context echo", line: "<rde:version>SECRET-VALUE</rde:version>", kind: lineIgnored},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, kind := parseLine(tc.line, v)
			assert.Equal(t, tc.kind, kind)
			if tc.kind == lineViolation || tc.kind == lineParseError {
				assert.Equal(t, tc.want, got)
			}
			assert.NotContains(t, strings.Join(append([]string{got.Element.Text, got.Attribute.Text, got.Facet, got.Type}, got.Expected...), " "), "SECRET-VALUE")
		})
	}
}

// A name that came from the deposit is cleaned and bounded before it can reach
// any field: markup, control characters and length are not the deposit's to
// choose.
func TestName_CleansDepositSuppliedNames(t *testing.T) {
	v := testVocab(t)
	line := "-:1: Schemas validity error : Element '{urn:ietf:params:xml:ns:rde-1.0}" + strings.Repeat("a", 500) + "<script>': This element is not expected."
	got, kind := parseLine(line, v)
	require.Equal(t, lineViolation, kind)
	assert.False(t, got.Element.Known)
	assert.LessOrEqual(t, len([]rune(got.Element.Text)), len("rde:")+maxNameLen+1)
	assert.NotContains(t, got.Element.Text, "<")
}

func TestParseLine_ExpectedListKeepsOnlyDeclaredNames(t *testing.T) {
	v := testVocab(t)
	line := "-:3: Schemas validity error : Element '{urn:ietf:params:xml:ns:rde-1.0}x': This element is not expected. Expected is one of ( {urn:ietf:params:xml:ns:rde-1.0}watermark, {urn:evil}smuggled, ##other{urn:evil}more )."
	got, _ := parseLine(line, v)
	assert.Equal(t, []string{"rde:watermark"}, got.Expected)
}
