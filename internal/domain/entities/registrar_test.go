package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRegistrar(t *testing.T) {
	tests := []struct {
		name       string
		clID       string
		nameStr    string
		email      string
		gurID      int
		postalInfo [2]*RegistrarPostalInfo
		wantErr    error
		want       *Registrar
	}{
		{
			name:       "no email",
			clID:       "my-registrar-007",
			nameStr:    "My Registrar",
			email:      "",
			gurID:      123,
			postalInfo: [2]*RegistrarPostalInfo{},
			wantErr:    ErrRegistrarMissingEmail,
			want:       nil,
		},
		{
			name:       "invalid clid",
			clID:       "",
			nameStr:    "My Registrar",
			email:      "geoff@apex.domains",
			gurID:      123,
			postalInfo: [2]*RegistrarPostalInfo{},
			wantErr:    ErrInvalidClIDType,
			want:       nil,
		},
		{
			name:    "valid rar",
			clID:    "my-registrar-007",
			nameStr: "My Registrar",
			email:   "geoff@apex.domains",
			gurID:   123,
			postalInfo: [2]*RegistrarPostalInfo{
				getValidRegistrarPostalInfo("loc"),
			},
			wantErr: nil,
			want: &Registrar{
				ClID:       "my-registrar-007",
				Name:       "My Registrar",
				NickName:   "My Registrar",
				Email:      "geoff@apex.domains",
				GurID:      123,
				Status:     RegistrarStatusReadonly,
				IANAStatus: IANARegistrarStatusUnknown,
				Autorenew:  false,
				PostalInfo: [2]*RegistrarPostalInfo{
					nil,
					getValidRegistrarPostalInfo("loc"),
				},
			},
		},
		{
			name:       "invalid postal info",
			clID:       "my-registrar-008",
			nameStr:    "My Registrar",
			email:      "geoff@apex.domains",
			gurID:      123,
			postalInfo: [2]*RegistrarPostalInfo{},
			wantErr:    ErrInvalidRegistrarPostalInfo,
		},
		{
			name:    "valid rar with both postal info",
			clID:    "my-registrar-007",
			nameStr: "My Registrar",
			email:   "geoff@apex.domains",
			gurID:   123,
			postalInfo: [2]*RegistrarPostalInfo{
				getValidRegistrarPostalInfo("loc"),
				getValidRegistrarPostalInfo("int"),
			},
			wantErr: nil,
			want: &Registrar{
				ClID:       "my-registrar-007",
				Name:       "My Registrar",
				NickName:   "My Registrar",
				Email:      "geoff@apex.domains",
				GurID:      123,
				Status:     RegistrarStatusReadonly,
				IANAStatus: IANARegistrarStatusUnknown,
				PostalInfo: [2]*RegistrarPostalInfo{
					getValidRegistrarPostalInfo("int"),
					getValidRegistrarPostalInfo("loc"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := NewRegistrar(test.clID, test.nameStr, test.email, test.gurID, test.postalInfo)
			require.Equal(t, test.wantErr, err)
			require.Equal(t, test.want, r)
		})

	}

}

func TestRegistrar_IsValid(t *testing.T) {
	testcases := []struct {
		testname string
		reg      *Registrar
		want     error
	}{
		{
			testname: "invalid clid",
			reg: &Registrar{
				ClID:     "",
				Name:     "My Registrar",
				NickName: "My Registrar",
				Email:    "g@my.com",
				GurID:    123,
				Status:   RegistrarStatusReadonly,
			},
			want: ErrInvalidClIDType,
		},
		{
			testname: "invalid name",
			reg: &Registrar{
				ClID:     "my-registrar-007",
				Name:     "",
				NickName: "My Registrar",
				Email:    "@gmy.com",
				GurID:    123,
				Status:   RegistrarStatusReadonly,
			},
			want: ErrRegistrarMissingName,
		},
		{
			testname: "invalid email",
			reg: &Registrar{
				ClID:     "my-registrar-007",
				Name:     "My Registrar",
				NickName: "My Registrar",
				Email:    "gmy.com",
				GurID:    123,
				Status:   RegistrarStatusReadonly,
			},
			want: ErrInvalidEmail,
		},
		{
			testname: "invalid status",
			reg: &Registrar{
				ClID:     "my-registrar-007",
				Name:     "My Registrar",
				NickName: "My Registrar",
				Email:    "g@my.com",
				GurID:    123,
				Status:   RegistrarStatus("invalid"),
			},
			want: ErrInvalidRegistrarStatus,
		},
		{
			testname: "invalid IANAstatus",
			reg: &Registrar{
				ClID:       "my-registrar-007",
				Name:       "My Registrar",
				NickName:   "My Registrar",
				Email:      "g@my.com",
				GurID:      123,
				Status:     RegistrarStatus("ok"),
				IANAStatus: IANARegistrarStatus("invalid"),
			},
			want: ErrInvalidRegistrarIANAStatus,
		},
		{
			testname: "valid",
			reg: &Registrar{
				ClID:     "my-registrar-007",
				Name:     "My Registrar",
				NickName: "My Registrar",
				Email:    "g@my.com",
				GurID:    123,
				PostalInfo: [2]*RegistrarPostalInfo{
					getValidRegistrarPostalInfo("int"),
					getValidRegistrarPostalInfo("loc"),
				},
				Status: RegistrarStatusReadonly,
			},
			want: nil,
		},
		{
			testname: "invalid postal info",
			reg: &Registrar{
				ClID:     "my-registrar-007",
				Name:     "My Registrar",
				NickName: "My Registrar",
				Email:    "g@my.com",
				GurID:    123,
				Status:   RegistrarStatusReadonly,
				PostalInfo: [2]*RegistrarPostalInfo{
					{
						Type: "invalid",
					},
				},
			},
			want: ErrInvalidRegistrarPostalInfo,
		},
		{
			testname: "invalid postal info: no postal info",
			reg: &Registrar{
				ClID:       "my-registrar-007",
				Name:       "My Registrar",
				NickName:   "My Registrar",
				Email:      "g@my.co",
				GurID:      123,
				Status:     RegistrarStatusReadonly,
				PostalInfo: [2]*RegistrarPostalInfo{},
			},
			want: ErrInvalidRegistrarPostalInfo,
		},
	}

	for _, test := range testcases {
		t.Run(test.testname, func(t *testing.T) {
			require.Equal(t, test.want, test.reg.Validate())
		})
	}

}

func getValidRegistrar() *Registrar {
	postalInfo := [2]*RegistrarPostalInfo{
		getValidRegistrarPostalInfo("int"),
		getValidRegistrarPostalInfo("loc"),
	}
	r, _ := NewRegistrar("gomamma01", "Go Mamma registry", "g@go.mamma", 1234, postalInfo)
	return r
}

func getValidRegistrarPostalInfo(t string) *RegistrarPostalInfo {
	a, err := NewAddress("BA", "AR")
	if err != nil {
		panic(err)
	}
	p, err := NewRegistrarPostalInfo(t, a)
	if err != nil {
		panic(err)
	}
	return p
}

func TestRemovePostalInfo(t *testing.T) {
	r := getValidRegistrar()
	r.PostalInfo[0] = getValidRegistrarPostalInfo("int")
	r.PostalInfo[1] = getValidRegistrarPostalInfo("loc")

	// Test case 0: Remove unknown postal info
	err := r.RemovePostalInfo("unknown")
	if err != ErrInvalidPostalInfoEnumType {
		t.Errorf("RemovePostalInfo() returned an unexpected error: %v", err)
	}
	// Verify that no postal info was removed
	if r.PostalInfo[0] == nil || r.PostalInfo[1] == nil {
		t.Errorf("RemovePostalInfo() removed a postal info when it should not have")
	}

	// Test case 1: Remove 'int' postal info
	err = r.RemovePostalInfo("int")
	if err != nil {
		t.Errorf("RemovePostalInfo() returned an unexpected error: %v", err)
	}
	if r.PostalInfo[0] != nil {
		t.Errorf("RemovePostalInfo() did not remove the 'int' postal info")
	}

	// Test case 2: Remove 'loc' postal info
	err = r.RemovePostalInfo("loc")
	if err != nil {
		t.Errorf("RemovePostalInfo() returned an unexpected error: %v", err)
	}
	if r.PostalInfo[1] != nil {
		t.Errorf("RemovePostalInfo() did not remove the 'loc' postal info")
	}

}

func TestRegistrarStatus_String(t *testing.T) {
	tests := []struct {
		name string
		s    RegistrarStatus
		want string
	}{
		{
			name: "terminated",
			s:    RegistrarStatusTerminated,
			want: "terminated",
		},
		{
			name: "readonlu",
			s:    RegistrarStatusReadonly,
			want: "readonly",
		},
		{
			name: "ok",
			s:    RegistrarStatusOK,
			want: "ok",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, test.s.String())
		})
	}
}
func TestRegistrar_IsAccreditedFor(t *testing.T) {
	r := &Registrar{
		TLDs: []*TLD{
			{Name: "com"},
			{Name: "net"},
			{Name: "org"},
		},
	}

	// Test case 1: Accredited TLD
	index, accredited := r.IsAccreditedFor(&TLD{Name: "net"})
	require.True(t, accredited)
	require.Equal(t, 1, index)

	// Test case 2: Non-accredited TLD
	index, accredited = r.IsAccreditedFor(&TLD{Name: "io"})
	require.False(t, accredited)
	require.Equal(t, 0, index)
}
func TestRegistrar_AccreditFor(t *testing.T) {
	r := &Registrar{
		TLDs: []*TLD{
			{Name: "com"},
			{Name: "net"},
			{Name: "org"},
		},
		Status: RegistrarStatusOK,
		GurID:  123,
	}

	// Test case 1: Accredited TLD
	err := r.AccreditFor(&TLD{Name: "net"})
	require.NoError(t, err)
	require.Equal(t, 3, len(r.TLDs))

	// Test case 2: Regstrar status prevents accreditation
	r.Status = RegistrarStatusTerminated
	err = r.AccreditFor(&TLD{Name: "io", Type: TLDTypeCCTLD})
	require.EqualError(t, err, ErrRegistrarStatusPreventsAccreditation.Error())
	require.Equal(t, 3, len(r.TLDs))

	// Test case 3: Registrar is not ICANN accredited
	r.Status = RegistrarStatusOK
	r.GurID = 0
	// Fail if is a GTLD
	err = r.AccreditFor(&TLD{Name: "apex", Type: TLDTypeGTLD})
	require.EqualError(t, err, ErrOnlyICANNAccreditedRegistrarsCanAccreditForGTLDs.Error())
	require.Equal(t, 3, len(r.TLDs))
	// Success if is ccTLD
	err = r.AccreditFor(&TLD{Name: "io", Type: TLDTypeCCTLD})
	require.NoError(t, err)
	require.Equal(t, 4, len(r.TLDs))

	// Test case 4: Accredited GTLD
	r.GurID = 1123
	r.IANAStatus = IANARegistrarStatusAccredited
	err = r.AccreditFor(&TLD{Name: "apex", Type: TLDTypeGTLD})
	require.NoError(t, err)
	require.Equal(t, 5, len(r.TLDs))
}

func TestRegistrar_DeAccreditFor(t *testing.T) {
	r := &Registrar{
		TLDs: []*TLD{
			{Name: "com"},
			{Name: "net"},
			{Name: "org"},
		},
		Status: RegistrarStatusOK,
		GurID:  123,
	}

	// Test case 1: Deaccredit TLD that isnot accredited
	err := r.DeAccreditFor(&TLD{Name: "apex"})
	require.NoError(t, err)
	require.Equal(t, 3, len(r.TLDs))

	// Test case 2: DEaccredit TLD that is accredited
	err = r.DeAccreditFor(&TLD{Name: "com"})
	require.NoError(t, err)
	require.Equal(t, 2, len(r.TLDs))

	// Test case 3: DEaccredit TLD that is accredited
	err = r.DeAccreditFor(&TLD{Name: "net"})
	require.NoError(t, err)
	require.Equal(t, 1, len(r.TLDs))

	// Test case 4: DEaccredit TLD that is accredited
	err = r.DeAccreditFor(&TLD{Name: "org"})
	require.NoError(t, err)
	require.Equal(t, 0, len(r.TLDs))

	// Test case 5: DEaccredit TLD that is no longer accredited
	err = r.DeAccreditFor(&TLD{Name: "com"})
	require.NoError(t, err)
	require.Equal(t, 0, len(r.TLDs))
}

func TestRegistrar_AddPostalInfo(t *testing.T) {
	testcases := []struct {
		name        string
		reg         *Registrar
		postal      *RegistrarPostalInfo
		expectedErr error
	}{
		{
			name:        "two int postal info",
			reg:         getValidRegistrar(),
			postal:      getValidRegistrarPostalInfo("int"),
			expectedErr: ErrRegistrarPostalInfoTypeExists,
		},
		{
			name:        "two loc postal info",
			reg:         getValidRegistrar(),
			postal:      getValidRegistrarPostalInfo("loc"),
			expectedErr: ErrRegistrarPostalInfoTypeExists,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			err := test.reg.AddPostalInfo(test.postal)
			require.Equal(t, test.expectedErr, err)
		})
	}
}

func TestRegistrarStatus_IsValid(t *testing.T) {
	tests := []struct {
		name string
		s    RegistrarStatus
		want bool
	}{
		{
			name: "ok",
			s:    RegistrarStatusOK,
			want: true,
		},
		{
			name: "readonly",
			s:    RegistrarStatusReadonly,
			want: true,
		},
		{
			name: "terminated",
			s:    RegistrarStatusTerminated,
			want: true,
		},
		{
			name: "invalid",
			s:    RegistrarStatus("invalid"),
			want: false,
		},
		{
			name: "case insensitive",
			s:    RegistrarStatus("tErMiNaTeD"),
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, test.s.IsValid())
		})
	}
}

func TestRegistrarIANAStatus_IsValid(t *testing.T) {
	tests := []struct {
		name string
		s    IANARegistrarStatus
		want bool
	}{
		{
			name: "empty", // nil value is not allowed, should use unknown
			s:    IANARegistrarStatus(""),
			want: false,
		},
		{
			name: "Unknown",
			s:    IANARegistrarStatus("Unknown"),
			want: true,
		},
		{
			name: "Accredited",
			s:    IANARegistrarStatusAccredited,
			want: true,
		},
		{
			name: "Reserved",
			s:    IANARegistrarStatusReserved,
			want: true,
		},
		{
			name: "Terminated",
			s:    IANARegistrarStatusTerminated,
			want: true,
		},
		{
			name: "invalid",
			s:    IANARegistrarStatus("invalid"),
			want: false,
		},
		{
			name: "case insensitive",
			s:    IANARegistrarStatus("tErMiNaTeD"),
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, test.s.IsValid())
		})
	}
}

func TestRegistrarStatus_SetStatus(t *testing.T) {
	r := &Registrar{
		Status: RegistrarStatusOK,
	}

	// Test case 1: Set status to readonly
	err := r.SetStatus(RegistrarStatusReadonly)
	require.NoError(t, err)
	require.Equal(t, RegistrarStatusReadonly, r.Status)

	// Test case 2: Set status to terminated
	err = r.SetStatus(RegistrarStatusTerminated)
	require.NoError(t, err)
	require.Equal(t, RegistrarStatusTerminated, r.Status)

	// Test case 3: Set status to invalid
	err = r.SetStatus(RegistrarStatus("invalid"))
	require.EqualError(t, err, ErrInvalidRegistrarStatus.Error())
	require.Equal(t, RegistrarStatusTerminated, r.Status)

	// Test case 4: Set status to readonly wiht strange casing
	err = r.SetStatus(RegistrarStatus("rEaDoNlY"))
	require.NoError(t, err)
	require.Equal(t, RegistrarStatusReadonly, r.Status)
}
func TestRegistrar_GetListRegistrarItem(t *testing.T) {
	tests := []struct {
		name string
		reg  *Registrar
		want *RegistrarListItem
	}{
		{
			name: "valid registrar",
			reg: &Registrar{
				ClID:      "my-registrar-007",
				Name:      "My Registrar",
				GurID:     123,
				Status:    RegistrarStatusOK,
				Autorenew: true,
			},
			want: &RegistrarListItem{
				ClID:      "my-registrar-007",
				Name:      "My Registrar",
				GurID:     123,
				Status:    RegistrarStatusOK,
				Autorenew: true,
			},
		},
		{
			name: "readonly status",
			reg: &Registrar{
				ClID:      "my-registrar-008",
				Name:      "Another Registrar",
				GurID:     456,
				Status:    RegistrarStatusReadonly,
				Autorenew: false,
			},
			want: &RegistrarListItem{
				ClID:      "my-registrar-008",
				Name:      "Another Registrar",
				GurID:     456,
				Status:    RegistrarStatusReadonly,
				Autorenew: false,
			},
		},
		{
			name: "terminated status",
			reg: &Registrar{
				ClID:      "my-registrar-009",
				Name:      "Terminated Registrar",
				GurID:     789,
				Status:    RegistrarStatusTerminated,
				Autorenew: true,
			},
			want: &RegistrarListItem{
				ClID:      "my-registrar-009",
				Name:      "Terminated Registrar",
				GurID:     789,
				Status:    RegistrarStatusTerminated,
				Autorenew: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.reg.GetListRegistrarItem()
			require.Equal(t, test.want, got)
		})
	}
}
