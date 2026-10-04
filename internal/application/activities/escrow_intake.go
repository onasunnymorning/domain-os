package activities

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// ---------------------------------------------------------------------------
// Escrow sFTP intake — the domain-os half of alpaca-infra's sFTP intake
// (alpaca-infra docs/sftp-intake.md, issue #391).
//
// SFTPGo writes each Registry Operator's uploads into the escrow bucket under
// <prefix>inbox/<RyID>/<tld>/. The <RyID> segment is the authenticated intake
// context (issue #412): the operator never chooses it, and the bucket policy
// denies every principal but that operator's role a write under it.
//
// A deposit there is either a signed pair, <base>.ryde with <base>.sig, or —
// only when ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT is on — a lone unsigned
// <name>.xml or <name>.xml.gz, validated with the plaintext profile. These
// activities find complete deposits, move each one out of the operator's
// reach, and settle it once validation has run:
//
//	inbox/<RyID>/<tld>/<base>.*            deposited, not yet picked up
//	claimed/<RyID>/<tld>/<id>/<base>.*     owned by one EscrowIntakeWorkflow
//	rejected/<RyID>/<tld>/<id>/<base>.*    never bound to a deposit; for a human
//
// The <id> level keeps two uploads of the same file name apart while both are
// in flight. The base name is preserved so BindDeposit's filename hints still
// work. A bound deposit lives on in escrow-validation/, so its claimed copy is
// deleted; the bucket's lifecycle rule expires whatever is left.
//
// Logging follows the EVE rule: identifiers and constant vocabulary, never an
// object key or file name.
// ---------------------------------------------------------------------------

const (
	escrowIntakeErrorType = "ESCROW_INTAKE"

	defaultEscrowIntakePrefix = "sftp/"
	escrowIntakeInbox         = "inbox/"
	escrowIntakeClaimed       = "claimed/"
	escrowIntakeRejected      = "rejected/"

	escrowIntakeArtifactExt  = ".ryde"
	escrowIntakeSignatureExt = ".sig"
	escrowIntakeXMLExt       = ".xml"
	escrowIntakeXMLGzExt     = ".xml.gz"

	// escrowIntakeListCap bounds one listing of the inbox. Pairs beyond it are
	// found by a later sweep once the ones before them have been claimed.
	escrowIntakeListCap = 5000
	// defaultEscrowIntakeMaxPairs bounds how many intakes one sweep starts.
	defaultEscrowIntakeMaxPairs = 25
	// escrowIntakeUnpairedWarnAge is how long half a pair may wait before the
	// sweep logs it. It is the cheap precursor to orphan-file alerting.
	escrowIntakeUnpairedWarnAge = time.Hour
)

// escrowIntakeRyIDPattern is the RyID shape an sFTP tenant can have. It is
// stricter than ClIDType because the RyID is also the sFTP user name and an
// IAM role suffix (alpaca-infra docs/sftp-intake.md, "RyID slug mapping").
var escrowIntakeRyIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{3,16}$`)

// reservedEscrowPrefixes are areas of the escrow bucket that other code owns.
// The intake prefix must not overlap them, or a claim or a settle could move
// or delete an archived deposit or an API upload.
var reservedEscrowPrefixes = []string{escrowValidationPrefix + "/", "uploads/"}

// EscrowIntakeConfig is the intake's configuration. The composition root reads
// it from the ESCROW_INTAKE_SFTP_* variables (config.LoadEscrowIntake, INV-17).
type EscrowIntakeConfig struct {
	// Enabled turns the sweep on. Off, it lists nothing.
	Enabled bool
	// AllowPlaintext accepts a lone unsigned .xml or .xml.gz as a deposit.
	// Off, such files are left where they are and only counted: an unsigned
	// deposit is personal data in the clear, which a real Registry Operator
	// should never send, so it is for test and simulation environments.
	AllowPlaintext bool
	// Prefix is the bucket prefix the sFTP server writes under, as configured;
	// empty means the default. NewEscrowIntakeActivities normalises it.
	Prefix string
}

// EscrowIntakeActivities holds the dependencies of the sFTP intake workflows.
type EscrowIntakeActivities struct {
	store interfaces.IntakeObjectStore
	runs  repositories.EscrowValidationRunRepository
	cfg   EscrowIntakeConfig
	now   func() time.Time
}

// NewEscrowIntakeActivities builds the activities. The schedule exists whether
// or not intake is enabled, so a worker with intake off still registers these,
// and the sweep finds nothing to do. It refuses a prefix that is the bucket
// root or overlaps an area other code owns.
func NewEscrowIntakeActivities(store interfaces.IntakeObjectStore, runs repositories.EscrowValidationRunRepository, cfg EscrowIntakeConfig) (*EscrowIntakeActivities, error) {
	prefix, err := normalizeEscrowIntakePrefix(cfg.Prefix)
	if err != nil {
		return nil, fmt.Errorf("escrow intake activities: %w", err)
	}
	cfg.Prefix = prefix
	return &EscrowIntakeActivities{
		store: store, runs: runs, cfg: cfg,
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

// normalizeEscrowIntakePrefix turns the configured prefix into the form the
// path contract uses: no leading slash, exactly one trailing slash. Empty
// means the default. It refuses the bucket root and any prefix that overlaps
// an area other code owns.
func normalizeEscrowIntakePrefix(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return defaultEscrowIntakePrefix, nil
	}
	p = strings.Trim(p, "/")
	if p == "" {
		return "", fmt.Errorf("ESCROW_INTAKE_SFTP_PREFIX=%q: the bucket root cannot be the intake prefix", raw)
	}
	p += "/"
	for _, reserved := range reservedEscrowPrefixes {
		if strings.HasPrefix(p, reserved) || strings.HasPrefix(reserved, p) {
			return "", fmt.Errorf("ESCROW_INTAKE_SFTP_PREFIX=%q overlaps %q, which other code owns", raw, reserved)
		}
	}
	return p, nil
}

// ---------------------------------------------------------------------------
// Key parsing and pairing (pure)
// ---------------------------------------------------------------------------

// escrowIntakeKey is one parsed inbox object.
type escrowIntakeKey struct {
	ryID, tld, base, ext string
}

// parseEscrowIntakeKey parses <inbox><RyID>/<tld>/<base>.{ryde,sig,xml,xml.gz}.
// Anything else — a directory marker, a deeper path, another extension, a RyID
// that cannot be an sFTP tenant — is not an intake object and returns false.
// Whether a plaintext file is accepted is decided later, not here.
func parseEscrowIntakeKey(inbox, key string) (escrowIntakeKey, bool) {
	rest, ok := strings.CutPrefix(key, inbox)
	if !ok {
		return escrowIntakeKey{}, false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 3 {
		return escrowIntakeKey{}, false
	}
	ryID, rawTLD, name := parts[0], parts[1], parts[2]
	if !escrowIntakeRyIDPattern.MatchString(ryID) {
		return escrowIntakeKey{}, false
	}
	if _, err := entities.NewOperatorID(ryID); err != nil {
		return escrowIntakeKey{}, false
	}
	tld, err := entities.NormalizeEscrowTLD(rawTLD)
	if err != nil {
		return escrowIntakeKey{}, false
	}
	var ext string
	for _, e := range []string{escrowIntakeArtifactExt, escrowIntakeSignatureExt, escrowIntakeXMLGzExt, escrowIntakeXMLExt} {
		if strings.HasSuffix(name, e) {
			ext = e
			break
		}
	}
	if ext == "" {
		return escrowIntakeKey{}, false
	}
	base := strings.TrimSuffix(name, ext)
	if base == "" {
		return escrowIntakeKey{}, false
	}
	return escrowIntakeKey{ryID: ryID, tld: tld, base: base, ext: ext}, true
}

// isPlaintextIntakeExt reports whether ext is an unsigned deposit on its own.
func isPlaintextIntakeExt(ext string) bool {
	return ext == escrowIntakeXMLExt || ext == escrowIntakeXMLGzExt
}

// intakeObjectKeys returns a deposit's object keys in order — artifact, then
// signature for a signed profile — after checking that the profile and the
// keys present agree. An empty profile is the signed one.
func intakeObjectKeys(profile, artifactKey, signatureKey string) (string, []string, error) {
	if profile == "" {
		profile = entities.EscrowProfileRydeSig
	}
	switch {
	case profile == entities.EscrowProfileRydeSig && signatureKey != "":
		return profile, []string{artifactKey, signatureKey}, nil
	case profile == entities.EscrowProfilePlaintextXML && signatureKey == "":
		return profile, []string{artifactKey}, nil
	default:
		return "", nil, nonRetryableIntake("the profile and the deposit's objects do not agree", nil)
	}
}

// intakeExtFits reports whether the i-th object of a deposit of this profile
// may have this extension.
func intakeExtFits(profile string, i int, ext string) bool {
	if profile == entities.EscrowProfilePlaintextXML {
		return i == 0 && isPlaintextIntakeExt(ext)
	}
	return (i == 0 && ext == escrowIntakeArtifactExt) || (i == 1 && ext == escrowIntakeSignatureExt)
}

// IntakePair is one complete deposit found in the inbox: a .ryde/.sig pair, or
// a lone plaintext file when that is allowed.
type IntakePair struct {
	Scope string `json:"scope"` // the RyID from the key path
	TLD   string `json:"tld"`
	// Profile is entities.EscrowProfileRydeSig or EscrowProfilePlaintextXML.
	Profile string `json:"profile"`
	// IntakeID is derived from the objects' keys, ETags and modification
	// times, so the same upload always gets the same ID and a re-upload of the
	// same file name gets a new one.
	IntakeID     string    `json:"intakeId"`
	WorkflowID   string    `json:"workflowId"`
	ArtifactKey  string    `json:"artifactKey"`            // inbox key
	SignatureKey string    `json:"signatureKey,omitempty"` // inbox key; signed profile only
	ReceivedAt   time.Time `json:"receivedAt"`             // the latest upload of the deposit
}

// escrowIntakeListing is what pairing found in one inbox listing.
type escrowIntakeListing struct {
	pairs            []IntakePair
	unpaired         int
	oldestUnpaired   time.Time
	markers          int
	ignored          int
	plaintextRefused int
}

func newIntakePair(k escrowIntakeKey, profile string, objects ...interfaces.ObjectInfo) IntakePair {
	received := objects[0].LastModified
	for _, o := range objects[1:] {
		if o.LastModified.After(received) {
			received = o.LastModified
		}
	}
	intakeID := escrowIntakeID(objects...)
	p := IntakePair{
		Scope: k.ryID, TLD: k.tld, Profile: profile, IntakeID: intakeID,
		WorkflowID:  "escrow-intake-" + k.ryID + "-" + k.tld + "-" + intakeID,
		ArtifactKey: objects[0].Key, ReceivedAt: received.UTC(),
	}
	if len(objects) > 1 {
		p.SignatureKey = objects[1].Key
	}
	return p
}

// pairEscrowIntakeObjects groups the listing into complete deposits, oldest
// first. A listed object is a complete upload (S3 makes an upload visible
// atomically), so half a pair simply waits for its other half, and a plaintext
// file is complete on its own.
func pairEscrowIntakeObjects(inbox string, objects []interfaces.ObjectInfo, allowPlaintext bool) escrowIntakeListing {
	type half struct {
		key  escrowIntakeKey
		info interfaces.ObjectInfo
	}
	type group struct{ artifact, signature *half }
	groups := map[string]*group{}
	var out escrowIntakeListing
	for _, obj := range objects {
		if strings.HasSuffix(obj.Key, "/") {
			out.markers++ // directory markers created by tofu so /<tld>/ exists in the chroot
			continue
		}
		k, ok := parseEscrowIntakeKey(inbox, obj.Key)
		if !ok {
			out.ignored++
			continue
		}
		if isPlaintextIntakeExt(k.ext) {
			if !allowPlaintext {
				out.plaintextRefused++
				continue
			}
			out.pairs = append(out.pairs, newIntakePair(k, entities.EscrowProfilePlaintextXML, obj))
			continue
		}
		id := k.ryID + "/" + k.tld + "/" + k.base
		g := groups[id]
		if g == nil {
			g = &group{}
			groups[id] = g
		}
		h := &half{key: k, info: obj}
		if k.ext == escrowIntakeArtifactExt {
			g.artifact = h
		} else {
			g.signature = h
		}
	}
	for _, g := range groups {
		if g.artifact == nil || g.signature == nil {
			h := g.artifact
			if h == nil {
				h = g.signature
			}
			out.unpaired++
			if out.oldestUnpaired.IsZero() || h.info.LastModified.Before(out.oldestUnpaired) {
				out.oldestUnpaired = h.info.LastModified
			}
			continue
		}
		out.pairs = append(out.pairs, newIntakePair(g.artifact.key, entities.EscrowProfileRydeSig, g.artifact.info, g.signature.info))
	}
	sort.Slice(out.pairs, func(i, j int) bool {
		if !out.pairs[i].ReceivedAt.Equal(out.pairs[j].ReceivedAt) {
			return out.pairs[i].ReceivedAt.Before(out.pairs[j].ReceivedAt)
		}
		return out.pairs[i].WorkflowID < out.pairs[j].WorkflowID
	})
	return out
}

// escrowIntakeID fingerprints one upload of a deposit.
func escrowIntakeID(objects ...interfaces.ObjectInfo) string {
	h := sha256.New()
	for _, o := range objects {
		fmt.Fprintf(h, "%s\x00%s\x00%d\x00", o.Key, o.ETag, o.LastModified.UTC().UnixNano())
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ---------------------------------------------------------------------------
// 1. ListIntakePairs
// ---------------------------------------------------------------------------

// ListIntakePairsInput bounds one sweep.
type ListIntakePairsInput struct {
	MaxPairs int `json:"maxPairs,omitempty"` // 0 means the default (25)
}

// ListIntakePairsOutput is what the sweep found. Only keys and counts cross
// the Temporal boundary; nothing is read.
type ListIntakePairsOutput struct {
	Enabled bool         `json:"enabled"`
	Pairs   []IntakePair `json:"pairs"`
	// Deferred counts complete pairs left for a later sweep by MaxPairs.
	Deferred int `json:"deferred"`
	// Unpaired counts files still waiting for their other half.
	Unpaired             int           `json:"unpaired"`
	OldestUnpairedWaited time.Duration `json:"oldestUnpairedWaited,omitempty"`
	// Ignored counts objects that are not intake objects (wrong extension,
	// wrong depth, a RyID that cannot be a tenant). Directory markers are not
	// counted.
	Ignored int `json:"ignored"`
	// PlaintextRefused counts unsigned .xml/.xml.gz deposits left in the inbox
	// because ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT is off.
	PlaintextRefused int `json:"plaintextRefused"`
	// Truncated reports that the listing hit its cap, so there may be more.
	Truncated bool `json:"truncated"`
}

// ListIntakePairs lists the inbox and returns the complete pairs, oldest first.
// It returns nothing when sFTP intake is disabled.
func (a *EscrowIntakeActivities) ListIntakePairs(ctx context.Context, in ListIntakePairsInput) (ListIntakePairsOutput, error) {
	if !a.cfg.Enabled {
		return ListIntakePairsOutput{}, nil
	}
	logger := activity.GetLogger(ctx)
	maxPairs := in.MaxPairs
	if maxPairs <= 0 {
		maxPairs = defaultEscrowIntakeMaxPairs
	}
	inbox := a.cfg.Prefix + escrowIntakeInbox
	objects, err := a.store.ListObjectsInfo(ctx, inbox, escrowIntakeListCap)
	if err != nil {
		return ListIntakePairsOutput{}, fmt.Errorf("ListIntakePairs: list inbox: %w", err)
	}
	listing := pairEscrowIntakeObjects(inbox, objects, a.cfg.AllowPlaintext)
	out := ListIntakePairsOutput{
		Enabled: true, Pairs: listing.pairs,
		Unpaired: listing.unpaired, Ignored: listing.ignored, PlaintextRefused: listing.plaintextRefused,
		Truncated: len(objects) >= escrowIntakeListCap,
	}
	if len(out.Pairs) > maxPairs {
		out.Deferred = len(out.Pairs) - maxPairs
		out.Pairs = out.Pairs[:maxPairs]
	}
	if !listing.oldestUnpaired.IsZero() {
		out.OldestUnpairedWaited = a.now().Sub(listing.oldestUnpaired)
		if out.OldestUnpairedWaited > escrowIntakeUnpairedWarnAge {
			logger.Warn("escrow intake: a deposit file has waited for its other half",
				"stage", "pair", "unpaired", out.Unpaired, "oldest_waited", out.OldestUnpairedWaited.Round(time.Second).String())
		}
	}
	if out.Truncated {
		logger.Warn("escrow intake: inbox listing reached its cap", "stage", "pair", "cap", escrowIntakeListCap)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// 2. ClaimIntakePair
// ---------------------------------------------------------------------------

// ClaimIntakeInput names the deposit to move out of the inbox.
type ClaimIntakeInput struct {
	Scope        string `json:"scope"`
	TLD          string `json:"tld"`
	Profile      string `json:"profile,omitempty"` // empty means ryde+sig
	IntakeID     string `json:"intakeId"`
	ArtifactKey  string `json:"artifactKey"`            // inbox key
	SignatureKey string `json:"signatureKey,omitempty"` // inbox key; signed profile only
}

// ClaimIntakeOutput gives the deposit's claimed keys.
type ClaimIntakeOutput struct {
	ArtifactKey  string `json:"artifactKey"`
	SignatureKey string `json:"signatureKey,omitempty"`
}

// ClaimIntakePair moves the pair from the inbox to claimed/, out of the
// operator's reach, so validation never reads a key the operator could rename.
// It is safe to retry from any point: an object already moved is not moved
// again.
func (a *EscrowIntakeActivities) ClaimIntakePair(ctx context.Context, in ClaimIntakeInput) (ClaimIntakeOutput, error) {
	profile, srcs, err := intakeObjectKeys(in.Profile, in.ArtifactKey, in.SignatureKey)
	if err != nil {
		return ClaimIntakeOutput{}, err
	}
	// The sweep only lists plaintext deposits when they are allowed; this
	// holds the line if the setting changed in between.
	if profile == entities.EscrowProfilePlaintextXML && !a.cfg.AllowPlaintext {
		return ClaimIntakeOutput{}, nonRetryableIntake("plaintext sFTP intake is not allowed (ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT)", nil)
	}
	inbox := a.cfg.Prefix + escrowIntakeInbox
	var out ClaimIntakeOutput
	for i, src := range srcs {
		k, ok := parseEscrowIntakeKey(inbox, src)
		if !ok || k.ryID != in.Scope || k.tld != in.TLD || !intakeExtFits(profile, i, k.ext) {
			return ClaimIntakeOutput{}, nonRetryableIntake("the key is not this pair's inbox object", nil)
		}
		dst := a.areaKey(escrowIntakeClaimed, k, in.IntakeID)
		// Not wrapped: Temporal reads retryability from the top-level error, so
		// wrapping move's non-retryable error would make it retryable again.
		if err := a.move(ctx, src, dst); err != nil {
			return ClaimIntakeOutput{}, err
		}
		if i == 0 {
			out.ArtifactKey = dst
		} else {
			out.SignatureKey = dst
		}
	}
	activity.GetLogger(ctx).Info("escrow intake: pair claimed",
		"correlation_id", activity.GetInfo(ctx).WorkflowExecution.ID, "tld", in.TLD, "stage", "claim")
	return out, nil
}

// ---------------------------------------------------------------------------
// 3. SettleIntakePair
// ---------------------------------------------------------------------------

// Dispositions of a settled intake.
const (
	// EscrowIntakeArchived: validation bound the pair to a deposit, whose copy
	// in escrow-validation/ is now the record. The claimed copy was deleted.
	EscrowIntakeArchived = "archived"
	// EscrowIntakeRejected: validation never bound the pair (the TLD is not the
	// operator's, the signature is too large, …). It was moved to rejected/.
	EscrowIntakeRejected = "rejected"
)

// SettleIntakeInput names a claimed deposit and the validation that ran on it.
type SettleIntakeInput struct {
	Scope                string `json:"scope"`
	TLD                  string `json:"tld"`
	Profile              string `json:"profile,omitempty"` // empty means ryde+sig
	IntakeID             string `json:"intakeId"`
	ValidationWorkflowID string `json:"validationWorkflowId"`
	ArtifactKey          string `json:"artifactKey"`            // claimed key
	SignatureKey         string `json:"signatureKey,omitempty"` // claimed key; signed profile only
}

// SettleIntakeOutput reports what happened to the claimed pair.
type SettleIntakeOutput struct {
	Disposition     string `json:"disposition"`
	ValidationRunID string `json:"validationRunId,omitempty"`
}

// SettleIntakePair decides by fact, not by outcome: a validation run exists
// for the workflow exactly when BindDeposit archived the deposit. Then the
// claimed copy is redundant and is deleted, whatever the outcome — an ERROR
// run is re-run from the archive. Without a run nothing was archived, so the
// pair is kept, in rejected/, for a human.
func (a *EscrowIntakeActivities) SettleIntakePair(ctx context.Context, in SettleIntakeInput) (SettleIntakeOutput, error) {
	logger := activity.GetLogger(ctx)
	scope, err := entities.NewOperatorID(in.Scope)
	if err != nil {
		return SettleIntakeOutput{}, nonRetryableIntake("invalid operator scope", err)
	}
	// Settling does not consult AllowPlaintext: a deposit already claimed is
	// finished whatever the setting is now.
	profile, srcs, err := intakeObjectKeys(in.Profile, in.ArtifactKey, in.SignatureKey)
	if err != nil {
		return SettleIntakeOutput{}, err
	}
	claimed := a.cfg.Prefix + escrowIntakeClaimed
	var keys []escrowIntakeKey
	for i, key := range srcs {
		k, ok := parseClaimedIntakeKey(claimed, key, in.IntakeID)
		if !ok || k.ryID != in.Scope || k.tld != in.TLD || !intakeExtFits(profile, i, k.ext) {
			return SettleIntakeOutput{}, nonRetryableIntake("the key is not this pair's claimed object", nil)
		}
		keys = append(keys, k)
	}

	run, err := a.runs.GetByWorkflowID(ctx, scope, in.ValidationWorkflowID)
	switch {
	case err == nil:
		for _, key := range srcs {
			if err := a.store.RemoveObject(ctx, key); err != nil {
				return SettleIntakeOutput{}, fmt.Errorf("SettleIntakePair: remove claimed object: %w", err)
			}
		}
		logger.Info("escrow intake: pair archived",
			"correlation_id", activity.GetInfo(ctx).WorkflowExecution.ID, "run_id", run.ID.String(),
			"tld", in.TLD, "stage", "settle", "outcome", EscrowIntakeArchived)
		return SettleIntakeOutput{Disposition: EscrowIntakeArchived, ValidationRunID: run.ID.String()}, nil
	case errors.Is(err, entities.ErrEscrowValidationRunNotFound):
		for i, key := range srcs {
			if err := a.move(ctx, key, a.areaKey(escrowIntakeRejected, keys[i], in.IntakeID)); err != nil {
				return SettleIntakeOutput{}, err // not wrapped; see ClaimIntakePair
			}
		}
		logger.Warn("escrow intake: pair rejected without a deposit",
			"correlation_id", activity.GetInfo(ctx).WorkflowExecution.ID,
			"tld", in.TLD, "stage", "settle", "outcome", EscrowIntakeRejected)
		return SettleIntakeOutput{Disposition: EscrowIntakeRejected}, nil
	default:
		return SettleIntakeOutput{}, fmt.Errorf("SettleIntakePair: run lookup: %w", err)
	}
}

// parseClaimedIntakeKey parses <claimed><RyID>/<tld>/<intakeID>/<base><ext>
// and checks the intake ID.
func parseClaimedIntakeKey(claimed, key, intakeID string) (escrowIntakeKey, bool) {
	rest, ok := strings.CutPrefix(key, claimed)
	if !ok {
		return escrowIntakeKey{}, false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 4 || parts[2] != intakeID || intakeID == "" {
		return escrowIntakeKey{}, false
	}
	return parseEscrowIntakeKey("", parts[0]+"/"+parts[1]+"/"+parts[3])
}

// areaKey is the key of an intake object in the claimed or rejected area.
func (a *EscrowIntakeActivities) areaKey(area string, k escrowIntakeKey, intakeID string) string {
	return a.cfg.Prefix + area + k.ryID + "/" + k.tld + "/" + intakeID + "/" + k.base + k.ext
}

// move copies src to dst and then deletes src. Run again after a partial
// move, it finishes the move; run again after a complete one, it does nothing.
func (a *EscrowIntakeActivities) move(ctx context.Context, src, dst string) error {
	srcExists, err := a.store.Exists(ctx, src)
	if err != nil {
		return fmt.Errorf("move intake object: source lookup: %w", err)
	}
	if !srcExists {
		dstExists, err := a.store.Exists(ctx, dst)
		if err != nil {
			return fmt.Errorf("move intake object: destination lookup: %w", err)
		}
		if dstExists {
			return nil
		}
		return nonRetryableIntake("an intake object disappeared before it was moved", nil)
	}
	if err := a.store.CopyObject(ctx, src, dst); err != nil {
		return fmt.Errorf("move intake object: copy: %w", err)
	}
	if err := a.store.RemoveObject(ctx, src); err != nil {
		return fmt.Errorf("move intake object: remove source: %w", err)
	}
	return nil
}

func nonRetryableIntake(msg string, cause error) error {
	if cause != nil {
		msg = msg + ": " + cause.Error()
	}
	return temporal.NewNonRetryableApplicationError(msg, escrowIntakeErrorType, cause)
}
