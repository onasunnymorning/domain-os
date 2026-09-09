package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// scoped performs a request with the operator scope header set (ADR-0006).
func scoped(method, path, tenant string, body interface{}) *httptest.ResponseRecorder {
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
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	return toRecorder(resp)
}

var _ = Describe("EscrowValidationController", Ordered, func() {
	const (
		ryID    = "eveop"
		otherRy = "eveother"
		tldName = "evetest"
	)

	BeforeAll(func() {
		resp := api.POST("/registry-operators", &commands.CreateRegistryOperatorCommand{RyID: ryID, Name: "EVE Test Operator", Email: "eve@example.com"})
		Expect(resp.Code).To(Equal(http.StatusCreated))
		resp = api.POST("/registry-operators", &commands.CreateRegistryOperatorCommand{RyID: otherRy, Name: "Other Operator", Email: "other@example.com"})
		Expect(resp.Code).To(Equal(http.StatusCreated))
		resp = api.POST("/tlds", &request.CreateTLDRequest{Name: tldName, RyID: ryID})
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	AfterAll(func() {
		api.DELETE("/tlds/" + tldName)
		api.DELETE("/registry-operators/" + ryID)
		api.DELETE("/registry-operators/" + otherRy)
	})

	body := map[string]string{"tld": tldName, "artifactObjectKey": "uploads/x.ryde", "signatureObjectKey": "uploads/x.sig"}

	It("requires the operator scope header", func() {
		resp := scoped(http.MethodPost, "/escrow/validations", "", body)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
		resp = scoped(http.MethodGet, "/escrow/validations", "", nil)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("hides a TLD the caller does not operate", func() {
		resp := scoped(http.MethodPost, "/escrow/validations", otherRy, body)
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	It("rejects an incomplete launch request", func() {
		resp := scoped(http.MethodPost, "/escrow/validations", ryID, map[string]string{"tld": tldName})
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("rejects a mismatched profile and artifact set", func() {
		// A signed profile without a signature, and an unsigned one with it,
		// are both caller errors — the profile decides the artifact set.
		resp := scoped(http.MethodPost, "/escrow/validations", ryID,
			map[string]string{"tld": tldName, "artifactObjectKey": "uploads/x.ryde"})
		Expect(resp.Code).To(Equal(http.StatusBadRequest))

		resp = scoped(http.MethodPost, "/escrow/validations", ryID,
			map[string]string{"tld": tldName, "profile": "xml", "artifactObjectKey": "uploads/x.xml", "signatureObjectKey": "uploads/x.sig"})
		Expect(resp.Code).To(Equal(http.StatusBadRequest))

		resp = scoped(http.MethodPost, "/escrow/validations", ryID,
			map[string]string{"tld": tldName, "profile": "ryde", "artifactObjectKey": "uploads/x.ryde"})
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
	})

	It("lists nothing for a fresh tenant and 404s unknown runs", func() {
		resp := scoped(http.MethodGet, "/escrow/validations?tld="+tldName, ryID, nil)
		Expect(resp.Code).To(Equal(http.StatusOK))
		var out struct {
			Items []interface{} `json:"items"`
			Count int           `json:"count"`
		}
		Expect(json.Unmarshal(resp.Body.Bytes(), &out)).To(Succeed())
		Expect(out.Count).To(Equal(0))

		resp = scoped(http.MethodGet, "/escrow/validations/"+uuid.New().String(), ryID, nil)
		Expect(resp.Code).To(Equal(http.StatusNotFound))
		resp = scoped(http.MethodGet, "/escrow/validations/not-a-uuid", ryID, nil)
		Expect(resp.Code).To(Equal(http.StatusBadRequest))
		resp = scoped(http.MethodGet, "/escrow/deposits/"+uuid.New().String(), ryID, nil)
		Expect(resp.Code).To(Equal(http.StatusNotFound))
	})

	Describe("trusted keys", func() {
		var keyID string
		kp := newTestKeyPair()

		It("registers a key for an operated TLD and computes its fingerprint", func() {
			resp := scoped(http.MethodPost, "/escrow/trusted-keys", ryID, map[string]string{"tld": tldName, "armoredPublicKey": kp.ArmoredPublic, "label": "primary"})
			Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())
			var out rest.EscrowTrustedKeyResponse
			Expect(json.Unmarshal(resp.Body.Bytes(), &out)).To(Succeed())
			Expect(out.Fingerprint).To(Equal(kp.Fingerprint))
			Expect(out.Active).To(BeTrue())
			keyID = out.ID
		})

		It("rejects material that is not a single public key", func() {
			resp := scoped(http.MethodPost, "/escrow/trusted-keys", ryID, map[string]string{"tld": tldName, "armoredPublicKey": kp.ArmoredPrivate})
			Expect(resp.Code).To(Equal(http.StatusBadRequest))
			resp = scoped(http.MethodPost, "/escrow/trusted-keys", ryID, map[string]string{"tld": tldName, "armoredPublicKey": "garbage"})
			Expect(resp.Code).To(Equal(http.StatusBadRequest))
		})

		It("refuses registration against a foreign TLD", func() {
			resp := scoped(http.MethodPost, "/escrow/trusted-keys", otherRy, map[string]string{"tld": tldName, "armoredPublicKey": kp.ArmoredPublic})
			Expect(resp.Code).To(Equal(http.StatusNotFound))
		})

		It("lists keys per tenant only", func() {
			// Keys are immutable (never deleted), so earlier runs against the shared
			// test database leave theirs behind: assert on this run's fingerprint.
			fingerprints := func(tenant string) []string {
				resp := scoped(http.MethodGet, "/escrow/trusted-keys?tld="+tldName, tenant, nil)
				Expect(resp.Code).To(Equal(http.StatusOK))
				var out struct {
					Items []rest.EscrowTrustedKeyResponse `json:"items"`
				}
				Expect(json.Unmarshal(resp.Body.Bytes(), &out)).To(Succeed())
				var fps []string
				for _, k := range out.Items {
					fps = append(fps, k.Fingerprint)
				}
				return fps
			}
			Expect(fingerprints(ryID)).To(ContainElement(kp.Fingerprint))
			Expect(fingerprints(otherRy)).NotTo(ContainElement(kp.Fingerprint))
		})

		It("retires a key once, and only for its own tenant", func() {
			resp := scoped(http.MethodPost, "/escrow/trusted-keys/"+keyID+"/retire", otherRy, nil)
			Expect(resp.Code).To(Equal(http.StatusNotFound))
			resp = scoped(http.MethodPost, "/escrow/trusted-keys/"+keyID+"/retire", ryID, nil)
			Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())
			var out rest.EscrowTrustedKeyResponse
			Expect(json.Unmarshal(resp.Body.Bytes(), &out)).To(Succeed())
			Expect(out.RetiredAt).NotTo(BeNil())
			Expect(out.Active).To(BeFalse())
			resp = scoped(http.MethodPost, "/escrow/trusted-keys/"+keyID+"/retire", ryID, nil)
			Expect(resp.Code).To(Equal(http.StatusConflict))
		})
	})
})

type testKeyPair struct {
	ArmoredPublic, ArmoredPrivate, Fingerprint string
}

// newTestKeyPair generates a throwaway OpenPGP identity (Ginkgo's T does not
// satisfy testing.TB, so the rdetest helper cannot be used here).
func newTestKeyPair() testKeyPair {
	e, err := openpgp.NewEntity("registry-signer", "", "signer@example.com", &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA})
	Expect(err).NotTo(HaveOccurred())
	armorIt := func(blockType string, serialize func(w *bytes.Buffer) error) string {
		var buf bytes.Buffer
		w, err := armor.Encode(&buf, blockType, nil)
		Expect(err).NotTo(HaveOccurred())
		var inner bytes.Buffer
		Expect(serialize(&inner)).To(Succeed())
		_, _ = w.Write(inner.Bytes())
		_ = w.Close()
		return buf.String()
	}
	return testKeyPair{
		ArmoredPublic:  armorIt(openpgp.PublicKeyType, func(w *bytes.Buffer) error { return e.Serialize(w) }),
		ArmoredPrivate: armorIt(openpgp.PrivateKeyType, func(w *bytes.Buffer) error { return e.SerializePrivate(w, nil) }),
		Fingerprint:    rdevalidate.FingerprintHex(e),
	}
}
