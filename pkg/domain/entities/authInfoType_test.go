package entities

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthInfoType_IsValid(t *testing.T) {
	testCases := []struct {
		password string
		expected error
	}{
		{"ValidP@ssw0rd", nil},
		{"Weak", ErrInvalidAuthInfo},
		{"NoSpecialCharacter", ErrInvalidAuthInfo},
		{"nouppercase1@", ErrInvalidAuthInfo},
		{"NOLOWERCASEP@SSWORD", ErrInvalidAuthInfo},
		{"WayTooLongPasswordWithSpecialChars$!_", ErrInvalidAuthInfo},
	}

	for _, test := range testCases {
		t.Run(fmt.Sprintf("Password: %s", test.password), func(t *testing.T) {
			authInfo := AuthInfoType(test.password)
			if authInfo.Validate() != test.expected {
				t.Errorf("Expected %s to be %v", authInfo.String(), test.expected)
			}
		})
	}
}

func TestAuthInfoType_NewAuthInfo(t *testing.T) {
	testcases := []struct {
		testname string
		authinfo string
		err      error
	}{
		{"Valid password", "Str)NGp@zz", nil},
		{"Invalid password", "weak", ErrInvalidAuthInfo},
	}

	for _, test := range testcases {
		t.Run(test.testname, func(t *testing.T) {
			_, err := NewAuthInfoType(test.authinfo)
			require.Equal(t, test.err, err)
		})
	}
}

// failingReader stands in for an entropy source that has failed.
type failingReader struct{ err error }

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }

// scriptedReader yields a fixed byte sequence and then io.EOF.
type scriptedReader struct{ b []byte }

func (s *scriptedReader) Read(p []byte) (int, error) {
	if len(s.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.b)
	s.b = s.b[n:]
	return n, nil
}

func TestGenerateAuthInfo_IsCompliant(t *testing.T) {
	for i := 0; i < 5000; i++ {
		a, err := GenerateAuthInfo()
		require.NoError(t, err)
		require.NoError(t, a.Validate(), "generated %q", a)
		_, err = NewAuthInfoType(a.String())
		require.NoError(t, err)
		require.Len(t, a.String(), GENERATED_AUTHINFO_LENGTH)

		var upper, lower, digit, special bool
		for _, c := range a.String() {
			switch {
			case strings.ContainsRune(authInfoUpper, c):
				upper = true
			case strings.ContainsRune(authInfoLower, c):
				lower = true
			case strings.ContainsRune(authInfoDigits, c):
				digit = true
			case strings.ContainsRune(authInfoSpecial, c):
				special = true
			default:
				t.Fatalf("generated %q contains %q, outside the alphabet", a, c)
			}
		}
		require.True(t, upper && lower && digit && special, "generated %q is missing a character class", a)
	}
}

// The reason this function exists: an authInfo shared between objects is a transfer credential for all of them.
func TestGenerateAuthInfo_IsNotFixedOrRepeated(t *testing.T) {
	const n = 20000
	seen := make(map[AuthInfoType]struct{}, n)
	for i := 0; i < n; i++ {
		a, err := GenerateAuthInfo()
		require.NoError(t, err)
		require.NotEqual(t, "escr0W1mP*rt", a.String(), "the retired import default must never be produced")
		seen[a] = struct{}{}
	}
	// ~100 bits of entropy: a collision in 20k draws means the source is not random.
	require.Len(t, seen, n)
}

func TestGenerateAuthInfo_EntropyFailureIsAnErrorNotAWeakValue(t *testing.T) {
	sentinel := errors.New("entropy source down")

	a, err := generateAuthInfo(failingReader{err: sentinel})
	require.ErrorIs(t, err, sentinel)
	require.Empty(t, a)

	// A source that runs dry part-way through must fail too, not pad the value.
	a, err = generateAuthInfo(&scriptedReader{b: make([]byte, 10)})
	require.Error(t, err)
	require.Empty(t, a)
}

// Even a degenerate source (all zero bytes) cannot yield an invalid value, because a
// character from each class is guaranteed before the shuffle.
func TestGenerateAuthInfo_DegenerateSourceStillCompliant(t *testing.T) {
	a, err := generateAuthInfo(bytes.NewReader(make([]byte, 4096)))
	require.NoError(t, err)
	require.NoError(t, a.Validate())
}

func TestByteSource_RejectsBiasedBytes(t *testing.T) {
	// For n=76, bytes >= 228 would favour the low indexes and must be discarded, not folded with modulo.
	src := &byteSource{r: &scriptedReader{b: append([]byte{250, 255, 228}, make([]byte, 61)...)}}
	got, err := src.index(76)
	require.NoError(t, err)
	require.Equal(t, 0, got, "the three biased bytes should have been skipped in favour of the following zero")

	// The largest accepted byte maps to the largest index.
	src = &byteSource{r: &scriptedReader{b: append([]byte{227}, make([]byte, 63)...)}}
	got, err = src.index(76)
	require.NoError(t, err)
	require.Equal(t, 227%76, got)
}

func TestByteSource_RefillsAcrossBlocks(t *testing.T) {
	// More draws than one 64 byte block holds, to exercise the refill path.
	src := &byteSource{r: bytes.NewReader(make([]byte, 256))}
	for i := 0; i < 200; i++ {
		_, err := src.index(10)
		require.NoError(t, err)
	}
}

// A generated authInfo passes through NormalizeString on its way into a Contact (NewContact), and a
// value that changes there is not the value we minted: dropping a trailing "." turned some generated
// values invalid, so an import would have had contacts rejected at random. This checks every character
// of the alphabet in every position, which no sample of random draws can guarantee.
func TestGenerateAuthInfo_SurvivesNormalization(t *testing.T) {
	filler := "Ab3xyzwvutsr" // valid on its own; the character under test is added around it
	for _, class := range generatedAuthInfoClasses {
		for _, c := range class {
			for name, s := range map[string]string{
				"leading":  string(c) + filler,
				"middle":   filler[:6] + string(c) + filler[6:],
				"trailing": filler + string(c),
			} {
				require.Equal(t, s, NormalizeString(s), "%q is changed by NormalizeString when %s", c, name)
			}
		}
	}
}

// The generator is only useful if what it returns is accepted by the constructors an import goes through,
// so run a large sample through the real ones rather than only through Validate.
func TestGenerateAuthInfo_AcceptedByEntityConstructors(t *testing.T) {
	// The failure this guards against occurred about once per 2,800 values, so the sample must be well beyond that.
	const n = 60000
	for i := 0; i < n; i++ {
		a, err := GenerateAuthInfo()
		require.NoError(t, err)

		c, err := NewContact("validClID", "12345_CONT-APEX", "email@me.com", a.String(), "myRegstrarID")
		require.NoError(t, err, "NewContact rejected generated authInfo %q", a)
		require.Equal(t, a, c.AuthInfo, "NewContact changed generated authInfo %q", a)

		d, err := NewDomain("12345_DOM-APEX", "apex.domains", "myRegstrarID", a.String())
		require.NoError(t, err, "NewDomain rejected generated authInfo %q", a)
		require.Equal(t, a, d.AuthInfo)
	}
}
