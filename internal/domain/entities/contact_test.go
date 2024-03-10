package entities

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContact_NewContact(t *testing.T) {
	testcases := []struct {
		testName      string
		id            string
		roid          string
		email         string
		authInfo      string
		expectedError error
	}{
		{
			testName:      "valid contact",
			id:            "test",
			roid:          "12324234_CONT-APEX",
			email:         "contact@apex.domains",
			authInfo:      "VerySrt0ngP@ssword",
			expectedError: nil,
		},
		{
			testName:      "invalid clid",
			id:            "te",
			roid:          "12324234_CONT-APEX",
			email:         "g@me.com",
			authInfo:      "sdfsSDFSD*12312",
			expectedError: ErrInvalidClIDType,
		},
		{
			testName:      "invalid roid",
			id:            "test",
			roid:          "12324234-CONT_APEX",
			email:         "g@me.com",
			authInfo:      "sdfsSDFSD*12312",
			expectedError: ErrInvalidRoid,
		},
		{
			testName:      "invalid email",
			id:            "test",
			roid:          "12324234_CONT-APEX",
			email:         "gme.com",
			authInfo:      "sdfsSDFSD*12312",
			expectedError: ErrInvalidEmail,
		},
		{
			testName:      "invalid authInfo",
			id:            "test",
			roid:          "12324234_CONT-APEX",
			email:         "g@me.com",
			authInfo:      "sh",
			expectedError: ErrInvalidAuthInfo,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.testName, func(t *testing.T) {
			_, err := NewContact(tc.id, tc.roid, tc.email, tc.authInfo, "199-myrar")
			if tc.expectedError == nil {
				require.NoError(t, err)
			} else {
				require.True(t, errors.Is(err, tc.expectedError))
			}
		})
	}
}

func TestContactStatusType_String(t *testing.T) {
	c := ContactStatusType("ok")
	require.Equal(t, "ok", c.String())
}

func TestContactStatusType_IsValid(t *testing.T) {
	testcases := []struct {
		testName      string
		contactStatus ContactStatus
		trySet        ContactStatusType
		expectedError error
	}{
		{
			testName:      "valid set ok",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusOK,
			expectedError: nil,
		},
		{
			testName:      "valid set pending Create",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusPendingCreate,
			expectedError: nil,
		},
		{
			testName:      "valid set pending Update",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusPendingUpdate,
			expectedError: nil,
		},
		{
			testName:      "valid set pending Delete",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusPendingDelete,
			expectedError: nil,
		},
		{
			testName:      "valid set pending Transfer",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusPendingTransfer,
			expectedError: nil,
		},
		{
			testName:      "valid set Client Delete Prohibited",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusClientDeleteProhibited,
			expectedError: nil,
		},
		{
			testName:      "valid set Client Transfer Prohibited",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusClientTransferProhibited,
			expectedError: nil,
		},
		{
			testName:      "valid set Client Update Prohibited",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusClientUpdateProhibited,
			expectedError: nil,
		},
		{
			testName:      "valid set Server Delete Prohibited",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusServerDeleteProhibited,
			expectedError: nil,
		},
		{
			testName:      "valid set Server Transfer Prohibited",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusServerTransferProhibited,
			expectedError: nil,
		},
		{
			testName:      "valid set Server Update Prohibited",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusServerUpdateProhibited,
			expectedError: nil,
		},
		{
			testName:      "valid set Client Delete Prohibited",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusClientDeleteProhibited,
			expectedError: nil,
		},
		{
			testName:      "valid set Client Transfer Prohibited",
			contactStatus: ContactStatus{},
			trySet:        ContactStatusClientTransferProhibited,
			expectedError: nil,
		},
		{
			testName:      "combine pendings w pending Delete",
			contactStatus: ContactStatus{PendingCreate: true},
			trySet:        ContactStatusPendingDelete,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "combine pendings w pending Update",
			contactStatus: ContactStatus{PendingCreate: true},
			trySet:        ContactStatusPendingUpdate,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "combine pendings w pending Transfer",
			contactStatus: ContactStatus{PendingCreate: true},
			trySet:        ContactStatusPendingTransfer,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "combine pendings w pending Create",
			contactStatus: ContactStatus{PendingDelete: true},
			trySet:        ContactStatusPendingCreate,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "pending Delete w Client Delete Prohibited",
			contactStatus: ContactStatus{PendingDelete: true},
			trySet:        ContactStatusClientDeleteProhibited,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "pending Delete w Server Delete Prohibited",
			contactStatus: ContactStatus{PendingDelete: true},
			trySet:        ContactStatusServerDeleteProhibited,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "pending Update w Client Update Prohibited",
			contactStatus: ContactStatus{PendingUpdate: true},
			trySet:        ContactStatusClientUpdateProhibited,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "pending Update w Server Update Prohibited",
			contactStatus: ContactStatus{PendingUpdate: true},
			trySet:        ContactStatusClientUpdateProhibited,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "pending Transfer w Client Transfer Prohibited",
			contactStatus: ContactStatus{PendingTransfer: true},
			trySet:        ContactStatusClientTransferProhibited,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "pending Transfer w Server Transfer Prohibited",
			contactStatus: ContactStatus{PendingTransfer: true},
			trySet:        ContactStatusClientTransferProhibited,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "set linked with update prohibited",
			contactStatus: ContactStatus{ServerUpdateProhibited: true},
			trySet:        ContactStatusLinked,
			expectedError: nil,
		},
		{
			testName:      "set something with update prohibited",
			contactStatus: ContactStatus{ServerUpdateProhibited: true},
			trySet:        ContactStatusClientTransferProhibited,
			expectedError: ErrContactUpdateNotAllowed,
		},
		{
			testName:      "double set update prohibited",
			contactStatus: ContactStatus{ClientUpdateProhibited: true},
			trySet:        ContactStatusClientUpdateProhibited,
			expectedError: nil,
		},
		{
			testName:      "try set ok with prohibition",
			contactStatus: ContactStatus{ServerDeleteProhibited: true},
			trySet:        ContactStatusOK,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "try pendingTransfer",
			contactStatus: ContactStatus{ClientTransferProhibited: true},
			trySet:        ContactStatusPendingTransfer,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "try transfer prohibited",
			contactStatus: ContactStatus{PendingTransfer: true},
			trySet:        ContactStatusServerTransferProhibited,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "try pendingDelete",
			contactStatus: ContactStatus{ClientDeleteProhibited: true},
			trySet:        ContactStatusPendingDelete,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName:      "try pendingDelete",
			contactStatus: ContactStatus{PendingUpdate: true},
			trySet:        ContactStatusServerUpdateProhibited,
			expectedError: ErrInvalidContactStatusCombination,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.testName, func(t *testing.T) {
			c := Contact{
				ContactStatus: tc.contactStatus,
			}
			err := c.SetStatus(tc.trySet)
			require.Equal(t, tc.expectedError, err)
		})
	}

}

func TestContact_CanBeDeleted(t *testing.T) {
	testcases := []struct {
		testName       string
		contactStatus  ContactStatus
		expectedResult bool
	}{
		{
			testName:       "empty",
			contactStatus:  ContactStatus{},
			expectedResult: true,
		},
		{
			testName:       "ok",
			contactStatus:  ContactStatus{},
			expectedResult: true,
		},
		{
			testName:       "server delete prohibited",
			contactStatus:  ContactStatus{ServerDeleteProhibited: true},
			expectedResult: false,
		},
		{
			testName:       "client delete prohibited",
			contactStatus:  ContactStatus{ClientDeleteProhibited: true},
			expectedResult: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.testName, func(t *testing.T) {
			c := Contact{
				ContactStatus: tc.contactStatus,
			}
			require.Equal(t, tc.expectedResult, c.CanBeDeleted())
		})
	}
}

func TestContact_CanBeUpdated(t *testing.T) {
	testcases := []struct {
		testName       string
		contactStatus  ContactStatus
		expectedResult bool
	}{
		{
			testName:       "empty",
			contactStatus:  ContactStatus{},
			expectedResult: true,
		},
		{
			testName:       "ok",
			contactStatus:  ContactStatus{},
			expectedResult: true,
		},
		{
			testName:       "server update prohibited",
			contactStatus:  ContactStatus{ServerUpdateProhibited: true},
			expectedResult: false,
		},
		{
			testName:       "client update prohibited",
			contactStatus:  ContactStatus{ClientUpdateProhibited: true},
			expectedResult: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.testName, func(t *testing.T) {
			c := Contact{
				ContactStatus: tc.contactStatus,
			}
			require.Equal(t, tc.expectedResult, c.CanBeUpdated())
		})
	}
}

func TestContact_CanBeTransferred(t *testing.T) {
	testcases := []struct {
		testName       string
		contactStatus  ContactStatus
		expectedResult bool
	}{
		{
			testName:       "empty",
			contactStatus:  ContactStatus{},
			expectedResult: true,
		},
		{
			testName:       "ok",
			contactStatus:  ContactStatus{},
			expectedResult: true,
		},
		{
			testName:       "server transfer prohibited",
			contactStatus:  ContactStatus{ServerTransferProhibited: true},
			expectedResult: false,
		},
		{
			testName:       "client transfer prohibited",
			contactStatus:  ContactStatus{ClientTransferProhibited: true},
			expectedResult: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.testName, func(t *testing.T) {
			c := Contact{
				ContactStatus: tc.contactStatus,
			}
			require.Equal(t, tc.expectedResult, c.CanBeTransferred())
		})
	}
}

func TestContact_CheckOKIsSet(t *testing.T) {
	c := Contact{
		ContactStatus: ContactStatus{
			PendingCreate: true,
		},
	}

	c.UnSetStatus(string(ContactStatusPendingCreate))

	require.False(t, c.PendingCreate, "PendingCreate should have been removed")
	require.True(t, c.OK, "OK should have been set")
}

func TestAddContactPostalInfo(t *testing.T) {
	validEmail := "geoff@apex.domains"
	// Setup
	c, err := NewContact("myreference1234", "123_CONT-APEX", validEmail, ";S987djfl;sdj", "199-myrar")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	a, _ := NewAddress("London", "UK")
	pi, _ := NewContactPostalInfo("int", "Some Name", a)
	// Add a valid 'int' postal info
	err = c.AddPostalInfo(pi)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if c.PostalInfo[0].Type != "int" {
		t.Errorf("Expected postal info type 'int', got %v", c.PostalInfo[0].Type)
	}
	// Try and add a second 'int' postal info, should fail
	err = c.AddPostalInfo(pi)
	if err == nil {
		t.Errorf("Expected ErrPostalInfoTypeExists, got nil")
	}
	// Try and add an invalid 'int' postal info, should fail
	pi.Address.City = "United Kingdoñ"
	err = c.AddPostalInfo(pi)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	// Add a valid 'loc' postal info
	pi.Type = "loc"
	err = c.AddPostalInfo(pi)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// Try and add a second 'loc' postal info, should fail
	err = c.AddPostalInfo(pi)
	if err == nil {
		t.Errorf("Expected ErrPostalInfoTypeExists, got nil")
	}
	// Try and add an invalid type postal info, should fail
	pi.Type = "invalid"
	err = c.AddPostalInfo(pi)
	if err == nil {
		t.Errorf("Expected ErrPostalInfoTypeExists, got nil")
	}
}

func TestRemoveContactPostalInfo(t *testing.T) {
	validEmail := "geoff@apex.domains"
	// Setup
	c, err := NewContact("myref1234", "123_CONT-APEX", validEmail, "sdfkSD4ljsd;f", "199-myrar")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	a, _ := NewAddress("London", "UK")
	pi, _ := NewContactPostalInfo("int", "Some Name", a)
	// Add a valid 'int' postal info
	c.AddPostalInfo(pi)
	// Add a valid 'loc' postal info
	pi.Type = "loc"
	c.AddPostalInfo(pi)

	// Remove the 'int' postal info
	err = c.RemovePostalInfo("int")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if c.PostalInfo[0] != nil {
		t.Errorf("Expected postal info to be nil, got %v", c.PostalInfo[0])
	}
	// Remove the 'loc' postal info
	err = c.RemovePostalInfo("loc")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if c.PostalInfo[1] != nil {
		t.Errorf("Expected postal info to be nil, got %v", c.PostalInfo[0])
	}

	// Remove the 'int' postal info again, should work idempotently
	err = c.RemovePostalInfo("int")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if c.PostalInfo[0] != nil {
		t.Errorf("Expected postal info to be nil, got %v", c.PostalInfo[0])
	}
	// Remove the 'loc' postal info again, should work idempotently
	err = c.RemovePostalInfo("loc")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if c.PostalInfo[1] != nil {
		t.Errorf("Expected postal info to be nil, got %v", c.PostalInfo[0])
	}

}

func TestContact_IsValid(t *testing.T) {
	testcases := []struct {
		testName       string
		Contact        Contact
		expectedResult bool
		expectedError  error
	}{
		{
			testName: "invalid ContactStatus",
			Contact: Contact{
				ID:       ClIDType("myref1234"),
				RoID:     RoidType("123_CONT-APEX"),
				Email:    "g@me.com",
				AuthInfo: AuthInfoType("sdfkSD4ljsd;f"),
			},
			expectedResult: false,
			expectedError:  ErrInvalidContactStatusCombination,
		},
		{
			testName: "invalid Voice",
			Contact: Contact{
				ID:            ClIDType("myref1234"),
				RoID:          RoidType("123_CONT-APEX"),
				Email:         "g@me.com",
				AuthInfo:      AuthInfoType("sdfkSD4ljsd;f"),
				ContactStatus: ContactStatus{OK: true},
				Voice:         E164Type("123"),
			},
			expectedResult: false,
			expectedError:  ErrInvalidE164Type,
		},
		{
			testName: "invalid Fax",
			Contact: Contact{
				ID:            ClIDType("myref1234"),
				RoID:          RoidType("123_CONT-APEX"),
				Email:         "g@me.com",
				AuthInfo:      AuthInfoType("sdfkSD4ljsd;f"),
				ContactStatus: ContactStatus{OK: true},
				Fax:           E164Type("123"),
			},
			expectedResult: false,
			expectedError:  ErrInvalidE164Type,
		},
		{
			testName: "invalid Postalinfo",
			Contact: Contact{
				ID:            ClIDType("myref1234"),
				RoID:          RoidType("123_CONT-APEX"),
				Email:         "g@me.com",
				AuthInfo:      AuthInfoType("sdfkSD4ljsd;f"),
				ContactStatus: ContactStatus{OK: true},
				PostalInfo: [2]*ContactPostalInfo{
					{
						Type: "int",
						Name: "Some Näme",
					},
					nil,
				},
			},
			expectedResult: false,
			expectedError:  ErrInvalidContactPostalInfo,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.testName, func(t *testing.T) {
			result, err := tc.Contact.IsValid()
			require.Equal(t, tc.expectedResult, result)
			require.Equal(t, tc.expectedError, err)
		})
	}
}

func TestContact_UnSetStatus(t *testing.T) {
	testcases := []struct {
		testName      string
		Contact       Contact
		status        ContactStatusType
		expectedError error
	}{
		{
			testName: "unset OK",
			Contact: Contact{
				ContactStatus: ContactStatus{OK: true},
			},
			status:        ContactStatusOK,
			expectedError: ErrInvalidContactStatusCombination,
		},
		{
			testName: "unset PendingCreate",
			Contact: Contact{
				ContactStatus: ContactStatus{PendingCreate: true},
			},
			status:        ContactStatusPendingCreate,
			expectedError: nil,
		},
		{
			testName: "unset PendingUpdate",
			Contact: Contact{
				ContactStatus: ContactStatus{PendingUpdate: true},
			},
			status:        ContactStatusPendingUpdate,
			expectedError: nil,
		},
		{
			testName: "unset PendingDelete",
			Contact: Contact{
				ContactStatus: ContactStatus{PendingDelete: true},
			},
			status:        ContactStatusPendingDelete,
			expectedError: nil,
		},
		{
			testName: "unset PendingTransfer",
			Contact: Contact{
				ContactStatus: ContactStatus{PendingTransfer: true},
			},
			status:        ContactStatusPendingTransfer,
			expectedError: nil,
		},
		{
			testName: "unset ClientDeleteProhibited",
			Contact: Contact{
				ContactStatus: ContactStatus{ClientDeleteProhibited: true},
			},
			status:        ContactStatusClientDeleteProhibited,
			expectedError: nil,
		},
		{
			testName: "unset ClientTransferProhibited",
			Contact: Contact{
				ContactStatus: ContactStatus{ClientTransferProhibited: true},
			},
			status:        ContactStatusClientTransferProhibited,
			expectedError: nil,
		},
		{
			testName: "unset ClientUpdateProhibited",
			Contact: Contact{
				ContactStatus: ContactStatus{ClientUpdateProhibited: true},
			},
			status:        ContactStatusClientUpdateProhibited,
			expectedError: nil,
		},
		{
			testName: "try unset OK",
			Contact: Contact{
				ContactStatus: ContactStatus{ClientUpdateProhibited: true},
			},
			status:        ContactStatusOK,
			expectedError: ErrContactUpdateNotAllowed,
		},
		{
			testName: "unset OK when not set",
			Contact: Contact{
				ContactStatus: ContactStatus{ClientTransferProhibited: true},
			},
			status:        ContactStatusOK,
			expectedError: nil,
		},
		{
			testName: "unset linked",
			Contact: Contact{
				ContactStatus: ContactStatus{Linked: true},
			},
			status:        ContactStatusLinked,
			expectedError: nil,
		},
		{
			testName: "unset server delete prohibited",
			Contact: Contact{
				ContactStatus: ContactStatus{ServerDeleteProhibited: true},
			},
			status:        ContactStatusServerDeleteProhibited,
			expectedError: nil,
		},
		{
			testName: "unset server update prohibited",
			Contact: Contact{
				ContactStatus: ContactStatus{ServerUpdateProhibited: true},
			},
			status:        ContactStatusServerUpdateProhibited,
			expectedError: nil,
		},
		{
			testName: "unset server transfer prohibited",
			Contact: Contact{
				ContactStatus: ContactStatus{ServerTransferProhibited: true},
			},
			status:        ContactStatusServerTransferProhibited,
			expectedError: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.testName, func(t *testing.T) {
			err := tc.Contact.UnSetStatus(string(tc.status))
			require.Equal(t, tc.expectedError, err)
		})
	}
}

func TestContact_Status_IsNil(t *testing.T) {
	cs := ContactStatus{}

	require.True(t, cs.IsNil())

	cs = ContactStatus{
		OK: true,
	}

	require.False(t, cs.IsNil())
}

func TestContact_SetFullStatus(t *testing.T) {
	c, _ := NewContact("myref1234", "123_CONT-APEX", "me@g.com", "sdfkSD4ljsd;f", "199-myrar")

	cs := ContactStatus{
		PendingCreate: true,
	}

	err := c.SetFullStatus(cs)

	require.NoError(t, err)
	require.True(t, c.PendingCreate)
	require.False(t, c.OK)
}

func TestContact_SetFullStatus_Error(t *testing.T) {
	c, _ := NewContact("myref1234", "123_CONT-APEX", "me@g.com", "sdfkSD4ljsd;f", "199-myrar")

	cs := ContactStatus{
		OK:            true,
		PendingCreate: true,
	}

	err := c.SetFullStatus(cs)

	require.Error(t, err)
	require.True(t, c.OK)
}

func TestContact_Disclose_IsNil(t *testing.T) {
	cd := ContactDisclose{}

	require.True(t, cd.IsNil())

	cd = ContactDisclose{
		DiscloseNameInt: true,
	}

	require.False(t, cd.IsNil())
}
