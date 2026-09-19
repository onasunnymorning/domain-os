package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// keyReq performs a key-registry request with an optional operator scope and
// granted OAuth scopes.
func keyReq(method, path, tenant, authScopes string, body interface{}) *httptest.ResponseRecorder {
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	req, err := http.NewRequest(method, api.Server.URL+path, bytes.NewReader(payload))
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Content-Type", "application/json")
	if tenant != "" {
		req.Header.Set(rest.TenantIDHeader, tenant)
	}
	if authScopes != "" {
		req.Header.Set(TestAuthScopesHeader, authScopes)
	}
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	return toRecorder(resp)
}

func decode[T any](resp *httptest.ResponseRecorder) T {
	var out T
	Expect(json.Unmarshal(resp.Body.Bytes(), &out)).To(Succeed(), resp.Body.String())
	return out
}

// lockedPrivateKey armors kp's private key protected by passphrase.
func lockedPrivateKey(kp testKeyPair, passphrase string) string {
	el, err := openpgp.ReadArmoredKeyRing(strings.NewReader(kp.ArmoredPrivate))
	Expect(err).NotTo(HaveOccurred())
	Expect(el[0].EncryptPrivateKeys([]byte(passphrase), nil)).To(Succeed())
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PrivateKeyType, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(el[0].SerializePrivateWithoutSigning(w, nil)).To(Succeed())
	Expect(w.Close()).To(Succeed())
	return buf.String()
}

var _ = Describe("EscrowKeyRegistry", Ordered, func() {
	const (
		ryID     = "keyop"
		otherRy  = "keyother"
		tldName  = "keytest"
		tldOther = "keytest2"
		opAdmin  = rest.ScopeEscrowKeysAdmin
		platform = rest.ScopeEscrowPlatformKeysAdmin
		passwd   = "correct horse battery staple" // #nosec G101 -- fixture passphrase for a throwaway test key
	)
	var (
		rspID, eveID, otherRspID string
		signer                   = newTestKeyPair()
		service                  = newTestKeyPair()
		signingVersion           string
		decryptVersion           string
	)

	BeforeAll(func() {
		for _, op := range []string{ryID, otherRy} {
			resp := api.POST("/registry-operators", &commands.CreateRegistryOperatorCommand{RyID: op, Name: op, Email: op + "@example.com"})
			Expect(resp.Code).To(Equal(http.StatusCreated))
		}
		for _, tld := range []string{tldName, tldOther} {
			resp := api.POST("/tlds", &request.CreateTLDRequest{Name: tld, RyID: ryID})
			Expect(resp.Code).To(Equal(http.StatusCreated))
		}
	})

	AfterAll(func() {
		api.DELETE("/tlds/" + tldName)
		api.DELETE("/tlds/" + tldOther)
		api.DELETE("/registry-operators/" + ryID)
		api.DELETE("/registry-operators/" + otherRy)
	})

	Describe("parties and permissions", func() {
		It("needs the permission to change anything, and a scope to read", func() {
			body := map[string]string{"name": "Acme Registry", "kind": "RSP", "side": "external"}
			Expect(keyReq(http.MethodPost, "/escrow/parties", ryID, "", body).Code).To(Equal(http.StatusForbidden), "a principal without the permission changes nothing")
			Expect(keyReq(http.MethodPost, "/escrow/parties", "", opAdmin, body).Code).To(Equal(http.StatusBadRequest), "no operator and no platform permission")
			Expect(keyReq(http.MethodGet, "/escrow/parties", "", "", nil).Code).To(Equal(http.StatusBadRequest))
			Expect(keyReq(http.MethodGet, "/escrow/parties", ryID, "", nil).Code).To(Equal(http.StatusOK), "reading an operator's registry needs no permission")
		})

		It("registers an operator party and a platform party", func() {
			resp := keyReq(http.MethodPost, "/escrow/parties", ryID, opAdmin, map[string]string{"name": "Acme Registry", "kind": "RSP", "side": "external"})
			Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
			rsp := decode[rest.EscrowPartyResponse](resp)
			Expect(rsp.Owner).To(Equal(rest.EscrowKeyOwnerResponse{Kind: "operator", Operator: ryID}))
			Expect(rsp.Purposes).To(Equal([]string{"verify-inbound"}))
			// Derived server-side so that every client stops deriving it, each
			// in its own words. Kind and side are still sent: they are what
			// the record and the resolver match on.
			Expect(rsp.Role).To(Equal("source"))
			rspID = rsp.ID

			resp = keyReq(http.MethodPost, "/escrow/parties", "", platform, map[string]string{"name": "EVE", "kind": "DEA", "side": "self"})
			Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
			eve := decode[rest.EscrowPartyResponse](resp)
			Expect(eve.Owner.Kind).To(Equal("platform"))
			Expect(eve.Purposes).To(ConsistOf("decrypt-inbound", "pseudonymise"))
			Expect(eve.Role).To(Equal("receiving-identity"))
			eveID = eve.ID

			resp = keyReq(http.MethodPost, "/escrow/parties", otherRy, opAdmin, map[string]string{"name": "Other RSP", "kind": "RSP", "side": "external"})
			Expect(resp.Code).To(Equal(http.StatusCreated))
			otherRspID = decode[rest.EscrowPartyResponse](resp).ID

			resp = keyReq(http.MethodPost, "/escrow/parties", ryID, opAdmin, map[string]string{"name": "Our RSP", "kind": "RSP", "side": "self"})
			Expect(resp.Code).To(Equal(http.StatusBadRequest), "outbound roles arrive with escrow targets")
		})

		It("shows platform parties to operators but lets only the platform change them", func() {
			resp := keyReq(http.MethodGet, "/escrow/parties/"+eveID, ryID, "", nil)
			Expect(resp.Code).To(Equal(http.StatusOK))
			detail := decode[struct {
				Party rest.EscrowPartyResponse `json:"party"`
			}](resp)
			Expect(detail.Party.Manageable).To(BeFalse())

			resp = keyReq(http.MethodPost, "/escrow/parties/"+eveID+"/versions/generate", ryID, opAdmin, map[string]string{"purpose": "pseudonymise"})
			Expect(resp.Code).To(Equal(http.StatusForbidden))

			Expect(keyReq(http.MethodGet, "/escrow/parties/"+rspID, otherRy, "", nil).Code).To(Equal(http.StatusNotFound), "never another operator's")
			Expect(keyReq(http.MethodGet, "/escrow/parties/"+rspID, "", platform, nil).Code).To(Equal(http.StatusNotFound), "the platform does not see into operators")
		})
	})

	Describe("key versions", func() {
		It("adds a counterparty public key and activates it without a probe", func() {
			resp := keyReq(http.MethodPost, "/escrow/parties/"+rspID+"/versions/public", ryID, opAdmin, map[string]string{"purpose": "verify-inbound", "armoredPublicKey": signer.ArmoredPublic})
			Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
			v := decode[rest.EscrowKeyVersionResponse](resp)
			Expect(v.Fingerprint).To(Equal(signer.Fingerprint))
			Expect(v.State).To(Equal("STAGED"))
			signingVersion = v.ID

			resp = keyReq(http.MethodPost, "/escrow/parties/"+rspID+"/versions/public", ryID, opAdmin, map[string]string{"purpose": "verify-inbound", "armoredPublicKey": signer.ArmoredPublic})
			Expect(resp.Code).To(Equal(http.StatusConflict), "the same key twice")
			resp = keyReq(http.MethodPost, "/escrow/parties/"+rspID+"/versions/public", ryID, opAdmin, map[string]string{"purpose": "verify-inbound", "armoredPublicKey": signer.ArmoredPrivate})
			Expect(resp.Code).To(Equal(http.StatusBadRequest), "a private key is not a public key")
			Expect(resp.Body.String()).NotTo(ContainSubstring("PRIVATE KEY"))

			resp = keyReq(http.MethodPost, "/escrow/key-versions/"+signingVersion+"/activate", ryID, opAdmin, nil)
			Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
			Expect(decode[rest.EscrowKeyVersionResponse](resp).State).To(Equal("ACTIVE"))

			resp = keyReq(http.MethodGet, "/escrow/key-versions/"+signingVersion+"/public-key", ryID, "", nil)
			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(resp.Body.String()).To(ContainSubstring("BEGIN PGP PUBLIC KEY BLOCK"))
		})

		It("imports a private key into the key store, never echoing it, and starts a probe", func() {
			locked := lockedPrivateKey(service, passwd)

			resp := keyReq(http.MethodPost, "/escrow/parties/"+eveID+"/versions/private", "", platform, map[string]string{"purpose": "decrypt-inbound", "armoredPrivateKey": locked, "passphrase": "wrong"})
			Expect(resp.Code).To(Equal(http.StatusBadRequest))
			Expect(resp.Body.String()).NotTo(ContainSubstring("wrong"))
			Expect(resp.Body.String()).NotTo(ContainSubstring("PRIVATE KEY"))

			resp = keyReq(http.MethodPost, "/escrow/parties/"+eveID+"/versions/private", "", platform, map[string]string{"purpose": "decrypt-inbound", "armoredPrivateKey": locked, "passphrase": passwd})
			Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
			Expect(resp.Body.String()).NotTo(ContainSubstring("PRIVATE KEY"))
			Expect(resp.Body.String()).NotTo(ContainSubstring(passwd))
			Expect(resp.Body.String()).NotTo(ContainSubstring("escrow-keys/"), "no secret reference leaves the API")
			v := decode[rest.EscrowKeyVersionResponse](resp)
			Expect(v.Fingerprint).To(Equal(service.Fingerprint))
			Expect(v.Material).To(Equal("openpgp-private"))
			Expect(v.HasPublicKey).To(BeTrue())
			Expect(v.ProbeWorkflowID).NotTo(BeEmpty())
			decryptVersion = v.ID
			Expect(escrowProbes.Versions).To(ContainElement(uuid.MustParse(v.ID)))

			resp = keyReq(http.MethodPost, "/escrow/key-versions/"+decryptVersion+"/activate", "", platform, nil)
			Expect(resp.Code).To(Equal(http.StatusConflict), "no successful probe yet")
		})

		It("generates a pseudonymisation key nobody sees", func() {
			resp := keyReq(http.MethodPost, "/escrow/parties/"+eveID+"/versions/generate", "", platform, map[string]string{"purpose": "pseudonymise"})
			Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
			v := decode[rest.EscrowKeyVersionResponse](resp)
			Expect(v.Material).To(Equal("symmetric"))
			Expect(v.HasPublicKey).To(BeFalse())
			Expect(v.Fingerprint).To(HaveLen(16))
			Expect(keyReq(http.MethodGet, "/escrow/key-versions/"+v.ID+"/public-key", "", platform, nil).Code).To(Equal(http.StatusNotFound))
		})

		It("revokes with a reason and destroys only with the fingerprint repeated", func() {
			Expect(keyReq(http.MethodPost, "/escrow/key-versions/"+decryptVersion+"/revoke", "", platform, map[string]string{}).Code).To(Equal(http.StatusBadRequest))
			resp := keyReq(http.MethodPost, "/escrow/key-versions/"+decryptVersion+"/revoke", "", platform, map[string]interface{}{"reason": "test rotation", "compromised": false})
			Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
			Expect(decode[rest.EscrowKeyVersionResponse](resp).State).To(Equal("REVOKED"))

			Expect(keyReq(http.MethodPost, "/escrow/key-versions/"+decryptVersion+"/destroy", "", platform, map[string]string{"confirmFingerprint": signer.Fingerprint}).Code).
				To(Equal(http.StatusBadRequest))
			resp = keyReq(http.MethodPost, "/escrow/key-versions/"+decryptVersion+"/destroy", "", platform, map[string]string{"confirmFingerprint": strings.ToLower(service.Fingerprint)})
			Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
			Expect(decode[rest.EscrowKeyVersionResponse](resp).State).To(Equal("DESTROYED"))
		})

		It("keeps an audit trail of every change", func() {
			resp := keyReq(http.MethodGet, "/escrow/parties/"+eveID+"/audit?pagesize=50", "", platform, nil)
			Expect(resp.Code).To(Equal(http.StatusOK))
			out := decode[struct {
				Items []rest.EscrowKeyAuditEventResponse `json:"items"`
			}](resp)
			var actions []string
			for _, e := range out.Items {
				actions = append(actions, e.Action)
			}
			Expect(actions).To(ContainElements("party.created", "key_version.imported", "key_version.generated", "key_version.revoked", "key_version.destroyed"))
			Expect(resp.Body.String()).NotTo(ContainSubstring("PRIVATE KEY"))
		})
	})

	Describe("arrangements", func() {
		It("sets an operator default and a TLD override, and resolves them with inheritance", func() {
			Expect(keyReq(http.MethodPut, "/escrow/arrangements/default", ryID, "", map[string]string{"depositorPartyId": rspID}).Code).
				To(Equal(http.StatusForbidden))
			resp := keyReq(http.MethodPut, "/escrow/arrangements/default", ryID, opAdmin, map[string]string{"depositorPartyId": rspID})
			Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
			Expect(decode[rest.EscrowArrangementResponse](resp).Level).To(Equal("operator"))

			resp = keyReq(http.MethodPut, "/escrow/arrangements/default", "", platform, map[string]string{"receiverPartyId": eveID})
			Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
			Expect(decode[rest.EscrowArrangementResponse](resp).Level).To(Equal("platform"))

			resp = keyReq(http.MethodGet, "/escrow/arrangements/tlds/"+tldName+"/effective", ryID, "", nil)
			Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
			eff := decode[struct {
				Depositor *struct {
					PartyID string `json:"partyId"`
					From    string `json:"from"`
				} `json:"depositor"`
				Receiver *struct {
					PartyID string `json:"partyId"`
					From    string `json:"from"`
				} `json:"receiver"`
			}](resp)
			Expect(eff.Depositor.PartyID).To(Equal(rspID))
			Expect(eff.Depositor.From).To(Equal("operator"))
			Expect(eff.Receiver.PartyID).To(Equal(eveID))
			Expect(eff.Receiver.From).To(Equal("platform"))
		})

		It("refuses to point at a party the operator cannot see, or at a TLD it does not operate", func() {
			Expect(keyReq(http.MethodPut, "/escrow/arrangements/tlds/"+tldOther, ryID, opAdmin, map[string]string{"depositorPartyId": otherRspID}).Code).
				To(Equal(http.StatusNotFound), "another operator's party is indistinguishable from none")
			Expect(keyReq(http.MethodPut, "/escrow/arrangements/tlds/"+tldName, otherRy, opAdmin, map[string]string{"depositorPartyId": otherRspID}).Code).
				To(Equal(http.StatusNotFound))
			Expect(keyReq(http.MethodPut, "/escrow/arrangements/default", ryID, opAdmin, map[string]string{"depositorPartyId": eveID}).Code).
				To(Equal(http.StatusBadRequest), "a DEA cannot deposit")
		})

		It("overrides one TLD and removes the override again", func() {
			resp := keyReq(http.MethodPost, "/escrow/parties", ryID, opAdmin, map[string]string{"name": "Second backend", "kind": "RSP", "side": "external"})
			Expect(resp.Code).To(Equal(http.StatusCreated))
			second := decode[rest.EscrowPartyResponse](resp).ID

			resp = keyReq(http.MethodPut, "/escrow/arrangements/tlds/"+tldOther, ryID, opAdmin, map[string]string{"depositorPartyId": second})
			Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
			resp = keyReq(http.MethodGet, "/escrow/arrangements/tlds", ryID, "", nil)
			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(resp.Body.String()).To(ContainSubstring(second))

			resp = keyReq(http.MethodGet, "/escrow/parties/"+second, ryID, "", nil)
			Expect(resp.Body.String()).To(ContainSubstring(tldOther), "a party shows where it is used")

			Expect(keyReq(http.MethodDelete, "/escrow/arrangements/tlds/"+tldOther, ryID, opAdmin, nil).Code).To(Equal(http.StatusNoContent))
			Expect(keyReq(http.MethodGet, "/escrow/arrangements/tlds/"+tldOther, ryID, "", nil).Code).To(Equal(http.StatusNotFound))
			resp = keyReq(http.MethodGet, "/escrow/arrangements/tlds/"+tldOther+"/effective", ryID, "", nil)
			Expect(resp.Body.String()).To(ContainSubstring(rspID), "the TLD inherits the operator default again")
		})
	})

	It("lists runs by key version within the tenant", func() {
		resp := keyReq(http.MethodGet, "/escrow/validations?keyVersionId="+signingVersion, ryID, "", nil)
		Expect(resp.Code).To(Equal(http.StatusOK))
	})
})
