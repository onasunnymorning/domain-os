package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRegistrar(t *testing.T) {
	tests := []struct {
		name    string
		clID    string
		nameStr string
		email   string
		gurID   int
		wantErr error
		want    *Registrar
	}{
		{
			name:    "no email",
			clID:    "my-registrar-007",
			nameStr: "My Registrar",
			email:   "",
			gurID:   123,
			wantErr: ErrInvalidRegistrar,
			want:    nil,
		},
		{
			name:    "invalid clid",
			clID:    "",
			nameStr: "My Registrar",
			email:   "geoff@apex.domains",
			gurID:   123,
			wantErr: ErrInvalidRegistrar,
			want:    nil,
		},
		{
			name:    "valid rar",
			clID:    "my-registrar-007",
			nameStr: "My Registrar",
			email:   "geoff@apex.domains",
			gurID:   123,
			wantErr: nil,
			want: &Registrar{
				ClID:     "my-registrar-007",
				Name:     "My Registrar",
				NickName: "My Registrar",
				Email:    "geoff@apex.domains",
				GurID:    123,
				Status:   RegistrarStatusReadonly,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := NewRegistrar(test.clID, test.nameStr, test.email, test.gurID)
			require.Equal(t, test.wantErr, err)
			require.Equal(t, test.want, r)
		})

	}

}

func TestRegistrar_IsValid(t *testing.T) {
	testcases := []struct {
		testname string
		reg      *Registrar
		want     bool
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
			want: false,
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
			want: false,
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
			want: false,
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
			want: false,
		},
		{
			testname: "valid",
			reg: &Registrar{
				ClID:     "my-registrar-007",
				Name:     "My Registrar",
				NickName: "My Registrar",
				Email:    "g@my.com",
				GurID:    123,
				Status:   RegistrarStatusReadonly,
			},
			want: true,
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
			want: false,
		},
	}

	for _, test := range testcases {
		t.Run(test.testname, func(t *testing.T) {
			require.Equal(t, test.want, test.reg.IsValid())
		})
	}

}

func getValidRegistrar() *Registrar {
	r, _ := NewRegistrar("gomamma01", "Go Mamma registry", "g@go.mamma", 1234)
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
func TestRegsitrar_AddPostalInfo(t *testing.T) {
	r := getValidRegistrar()

	// Test case 1: Add 'int' postal info
	pi1 := getValidRegistrarPostalInfo("int")
	err := r.AddPostalInfo(pi1)
	if err != nil {
		t.Errorf("AddPostalInfo() returned an unexpected error: %v", err)
	}
	if r.PostalInfo[0] != pi1 {
		t.Errorf("AddPostalInfo() did not add the 'int' postal info correctly")
	}

	// Test case 2: Add 'loc' postal info
	pi2 := getValidRegistrarPostalInfo("loc")
	err = r.AddPostalInfo(pi2)
	if err != nil {
		t.Errorf("AddPostalInfo() returned an unexpected error: %v", err)
	}
	if r.PostalInfo[1] != pi2 {
		t.Errorf("AddPostalInfo() did not add the 'loc' postal info correctly")
	}

	// Test case 3: Add duplicate 'int' postal info
	err = r.AddPostalInfo(pi1)
	if err != ErrRegistrarPostalInfoTypeExists {
		t.Errorf("AddPostalInfo() did not return the expected error for duplicate 'int' postal info")
	}

	// Test case 4: Add duplicate 'loc' postal info
	err = r.AddPostalInfo(pi2)
	if err != ErrRegistrarPostalInfoTypeExists {
		t.Errorf("AddPostalInfo() did not return the expected error for duplicate 'loc' postal info")
	}

	// Test case 5: Add invalid postal info
	invalidPi := &RegistrarPostalInfo{Type: "invalid", Address: &Address{}}
	err = r.AddPostalInfo(invalidPi)
	if err == nil {
		t.Error("AddPostalInfo() did not return an error for an invalid postal info")
	}
	if r.PostalInfo[0] == invalidPi || r.PostalInfo[1] == invalidPi {
		t.Errorf("AddPostalInfo() added an invalid postal info when it should not have")
	}
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
