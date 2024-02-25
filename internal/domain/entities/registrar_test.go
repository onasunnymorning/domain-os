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
				ClID:     "my-registrar-007",
				Name:     "My Registrar",
				NickName: "My Registrar",
				Email:    "geoff@apex.domains",
				GurID:    123,
				Status:   RegistrarStatusReadonly,
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
				ClID:     "my-registrar-007",
				Name:     "My Registrar",
				NickName: "My Registrar",
				Email:    "geoff@apex.domains",
				GurID:    123,
				Status:   RegistrarStatusReadonly,
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
