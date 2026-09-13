package awssm

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAPI struct {
	created   *secretsmanager.CreateSecretInput
	getIn     *secretsmanager.GetSecretValueInput
	deleteIn  *secretsmanager.DeleteSecretInput
	createErr error
	getErr    error
	deleteErr error
	value     []byte
}

func (f *fakeAPI) CreateSecret(_ context.Context, in *secretsmanager.CreateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	f.created = in
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &secretsmanager.CreateSecretOutput{Name: in.Name, VersionId: aws.String("ver-1")}, nil
}

func (f *fakeAPI) GetSecretValue(_ context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	f.getIn = in
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &secretsmanager.GetSecretValueOutput{SecretBinary: f.value}, nil
}

func (f *fakeAPI) DeleteSecret(_ context.Context, in *secretsmanager.DeleteSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
	f.deleteIn = in
	return &secretsmanager.DeleteSecretOutput{}, f.deleteErr
}

func testCfg() Config {
	return Config{Region: "us-west-2", Prefix: "dos-test", KMSKeyID: "alias/escrow", RecoveryWindowDays: 7}
}

func TestStore_PutGetDelete(t *testing.T) {
	ctx := context.Background()
	api := &fakeAPI{value: []byte("material")}
	s := newWithClient(api, testCfg())

	ref, err := s.Put(ctx, "/escrow-keys/platform/p/v", map[string]string{"purpose": "decrypt-inbound"}, []byte("material"))
	require.NoError(t, err)
	assert.Equal(t, "dos-test/escrow-keys/platform/p/v", ref.Name, "the reference carries the full name")
	assert.Equal(t, "ver-1", ref.BackendVersionID)
	assert.Equal(t, "alias/escrow", aws.ToString(api.created.KmsKeyId))
	assert.Equal(t, []byte("material"), api.created.SecretBinary)
	require.Len(t, api.created.Tags, 1)
	assert.Nil(t, api.created.SecretString, "binary only; never a string that might be echoed")

	got, err := s.Get(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, []byte("material"), got)
	assert.Equal(t, "ver-1", aws.ToString(api.getIn.VersionId), "reads are pinned to the recorded backend version")

	require.NoError(t, s.ScheduleDelete(ctx, ref))
	assert.Equal(t, int64(7), aws.ToInt64(api.deleteIn.RecoveryWindowInDays))
}

func TestStore_ErrorsAreClassifiedAndNeverEchoTheBackendMessage(t *testing.T) {
	ctx := context.Background()
	// #nosec G101 -- a fake secret ARN used to prove it never reaches an error string
	secretish := "arn:aws:secretsmanager:us-west-2:1:secret:dos-test/escrow-keys/op-x/party/version"
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"not found", &smtypes.ResourceNotFoundException{Message: aws.String(secretish)}, entities.ErrEscrowSecretNotFound},
		{"outage", errors.New("dial tcp: " + secretish), entities.ErrEscrowKeyStoreUnavailable},
		{"marked for deletion", &smtypes.InvalidRequestException{Message: aws.String(secretish)}, entities.ErrEscrowSecretNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newWithClient(&fakeAPI{getErr: tc.err}, testCfg()).Get(ctx, entities.EscrowSecretRef{Name: "x"})
			require.ErrorIs(t, err, tc.want)
			assert.NotContains(t, err.Error(), secretish)
		})
	}

	_, err := newWithClient(&fakeAPI{createErr: &smtypes.ResourceExistsException{Message: aws.String(secretish)}}, testCfg()).Put(ctx, "n", nil, []byte("v"))
	assert.ErrorIs(t, err, entities.ErrEscrowSecretAlreadyExists)
	_, err = newWithClient(&fakeAPI{createErr: &smtypes.InvalidRequestException{}}, testCfg()).Put(ctx, "n", nil, []byte("v"))
	assert.ErrorIs(t, err, entities.ErrEscrowSecretAlreadyExists, "a name held by a scheduled deletion is taken")

	assert.NoError(t, newWithClient(&fakeAPI{deleteErr: &smtypes.ResourceNotFoundException{}}, testCfg()).ScheduleDelete(ctx, entities.EscrowSecretRef{Name: "gone"}),
		"deleting what is already gone is idempotent")
}

func TestConfigValidate(t *testing.T) {
	assert.Error(t, Config{RecoveryWindowDays: 30}.Validate(), "region is required")
	assert.Error(t, Config{Region: "r", RecoveryWindowDays: 3}.Validate())
	assert.NoError(t, Config{Region: "r", RecoveryWindowDays: 30}.Validate())
}

// TestStore_LocalStack exercises the real SDK against LocalStack when
// TEST_LOCALSTACK_ENDPOINT is set (docker compose --profile keystore up).
func TestStore_LocalStack(t *testing.T) {
	endpoint := os.Getenv("TEST_LOCALSTACK_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_LOCALSTACK_ENDPOINT not set")
	}
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	ctx := context.Background()
	s, err := New(ctx, Config{Region: "us-east-1", Prefix: "dos-it", Endpoint: endpoint, RecoveryWindowDays: 7})
	require.NoError(t, err)

	name := "escrow-keys/it/" + uuid.NewString()
	ref, err := s.Put(ctx, name, map[string]string{"purpose": "pseudonymise"}, []byte{0, 1, 2, 3})
	require.NoError(t, err)
	got, err := s.Get(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 1, 2, 3}, got)
	_, err = s.Put(ctx, name, nil, []byte{9})
	assert.ErrorIs(t, err, entities.ErrEscrowSecretAlreadyExists)
	require.NoError(t, s.ScheduleDelete(ctx, ref))
	_, err = s.Get(ctx, ref)
	assert.ErrorIs(t, err, entities.ErrEscrowSecretNotFound)
	_, err = s.Get(ctx, entities.EscrowSecretRef{Name: "dos-it/does-not-exist"})
	assert.ErrorIs(t, err, entities.ErrEscrowSecretNotFound)
}
