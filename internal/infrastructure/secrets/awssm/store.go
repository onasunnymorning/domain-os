// Package awssm is the AWS Secrets Manager adapter behind
// interfaces.EscrowSecretStore (issue #429, ADR-0009). It is the only package
// that imports the Secrets Manager SDK.
package awssm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Environment variables read by ConfigFromEnv. All are registered in
// internal/config/env_registry.go. Credentials come from the AWS default
// chain (IRSA on EKS, a task role on ECS, a profile locally), never from here.
const (
	EnvRegion             = "ESCROW_CUSTODY_AWS_REGION"
	EnvPrefix             = "ESCROW_CUSTODY_NAME_PREFIX"
	EnvKMSKeyID           = "ESCROW_CUSTODY_KMS_ID"
	EnvEndpoint           = "ESCROW_CUSTODY_ENDPOINT"
	EnvRecoveryWindowDays = "ESCROW_CUSTODY_RECOVERY_WINDOW_DAYS"
)

// Config configures the store.
type Config struct {
	Region   string
	Prefix   string // every secret name starts with "<Prefix>/"
	KMSKeyID string // optional customer-managed key; empty = the account default
	// Endpoint overrides the service endpoint (LocalStack in development).
	Endpoint string
	// RecoveryWindowDays is how long a scheduled deletion can be undone (7-30).
	RecoveryWindowDays int32
}

// DefaultPrefix and DefaultRecoveryWindowDays are the registry defaults.
const (
	DefaultPrefix             = "domain-os"
	DefaultRecoveryWindowDays = 30
)

// ConfigFromEnv reads the configuration.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Region:             strings.TrimSpace(os.Getenv(EnvRegion)),
		Prefix:             strings.Trim(strings.TrimSpace(os.Getenv(EnvPrefix)), "/"),
		KMSKeyID:           strings.TrimSpace(os.Getenv(EnvKMSKeyID)),
		Endpoint:           strings.TrimSpace(os.Getenv(EnvEndpoint)),
		RecoveryWindowDays: DefaultRecoveryWindowDays,
	}
	if cfg.Prefix == "" {
		cfg.Prefix = DefaultPrefix
	}
	if raw := strings.TrimSpace(os.Getenv(EnvRecoveryWindowDays)); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return cfg, fmt.Errorf("%s: %w", EnvRecoveryWindowDays, err)
		}
		cfg.RecoveryWindowDays = int32(n)
	}
	return cfg, cfg.Validate()
}

// Validate checks the configuration.
func (c Config) Validate() error {
	if c.Region == "" {
		return fmt.Errorf("%s is required for the aws-secrets-manager key store", EnvRegion)
	}
	if c.RecoveryWindowDays < 7 || c.RecoveryWindowDays > 30 {
		return fmt.Errorf("%s must be between 7 and 30", EnvRecoveryWindowDays)
	}
	return nil
}

// api is the subset of the Secrets Manager client the store uses.
type api interface {
	CreateSecret(ctx context.Context, in *secretsmanager.CreateSecretInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	DeleteSecret(ctx context.Context, in *secretsmanager.DeleteSecretInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error)
}

// Store implements interfaces.EscrowSecretStore on AWS Secrets Manager.
type Store struct {
	client api
	cfg    Config
}

var _ interfaces.EscrowSecretStore = (*Store)(nil)

// New builds a store with the AWS default credential chain.
func New(ctx context.Context, cfg Config) (*Store, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	client := secretsmanager.NewFromConfig(awsCfg, func(o *secretsmanager.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})
	return &Store{client: client, cfg: cfg}, nil
}

func newWithClient(client api, cfg Config) *Store {
	return &Store{client: client, cfg: cfg}
}

// fullName prefixes a name. A reference always carries the full name, so a
// later prefix change never orphans an existing version.
func (s *Store) fullName(name string) string {
	return s.cfg.Prefix + "/" + strings.TrimLeft(name, "/")
}

// Put creates a secret. The value is sent as SecretBinary and is never logged.
func (s *Store) Put(ctx context.Context, name string, tags map[string]string, value []byte) (entities.EscrowSecretRef, error) {
	if strings.TrimSpace(name) == "" || len(value) == 0 {
		return entities.EscrowSecretRef{}, errors.Join(entities.ErrInvalidEscrowKeyVersion, errors.New("a secret name and value are required"))
	}
	in := &secretsmanager.CreateSecretInput{
		Name:         aws.String(s.fullName(name)),
		SecretBinary: value,
		Description:  aws.String("domain-os escrow key material (ADR-0009); managed by the key registry, do not edit"),
	}
	if s.cfg.KMSKeyID != "" {
		in.KmsKeyId = aws.String(s.cfg.KMSKeyID)
	}
	for k, v := range tags {
		in.Tags = append(in.Tags, smtypes.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	out, err := s.client.CreateSecret(ctx, in)
	if err != nil {
		return entities.EscrowSecretRef{}, classify("put", err)
	}
	return entities.EscrowSecretRef{Name: aws.ToString(out.Name), BackendVersionID: aws.ToString(out.VersionId)}, nil
}

// Get returns the secret value at the referenced version.
func (s *Store) Get(ctx context.Context, ref entities.EscrowSecretRef) ([]byte, error) {
	in := &secretsmanager.GetSecretValueInput{SecretId: aws.String(ref.Name)}
	if ref.BackendVersionID != "" {
		in.VersionId = aws.String(ref.BackendVersionID)
	}
	out, err := s.client.GetSecretValue(ctx, in)
	if err != nil {
		return nil, classify("get", err)
	}
	if len(out.SecretBinary) > 0 {
		return out.SecretBinary, nil
	}
	if out.SecretString != nil {
		return []byte(*out.SecretString), nil
	}
	return nil, entities.ErrEscrowKeyMaterialUnreadable
}

// ScheduleDelete schedules deletion after the recovery window.
func (s *Store) ScheduleDelete(ctx context.Context, ref entities.EscrowSecretRef) error {
	_, err := s.client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId:             aws.String(ref.Name),
		RecoveryWindowInDays: aws.Int64(int64(s.cfg.RecoveryWindowDays)),
	})
	if err != nil {
		if errors.Is(classify("delete", err), entities.ErrEscrowSecretNotFound) {
			return nil
		}
		return classify("delete", err)
	}
	return nil
}

// classify maps an SDK error onto the fixed-text sentinels. The SDK message is
// deliberately dropped: it can name the secret, and error strings reach
// workflow history.
func classify(op string, err error) error {
	var notFound *smtypes.ResourceNotFoundException
	var exists *smtypes.ResourceExistsException
	var invalidReq *smtypes.InvalidRequestException
	switch {
	case errors.As(err, &notFound):
		return fmt.Errorf("secrets manager %s: %w", op, entities.ErrEscrowSecretNotFound)
	case errors.As(err, &exists):
		return fmt.Errorf("secrets manager %s: %w", op, entities.ErrEscrowSecretAlreadyExists)
	case errors.As(err, &invalidReq) && op == "put":
		// The name is still held by a secret scheduled for deletion.
		return fmt.Errorf("secrets manager %s: %w", op, entities.ErrEscrowSecretAlreadyExists)
	case errors.As(err, &invalidReq):
		// The secret is scheduled for deletion: for reads and deletes it is gone.
		return fmt.Errorf("secrets manager %s: %w", op, entities.ErrEscrowSecretNotFound)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("secrets manager %s: %w", op, err)
	default:
		return fmt.Errorf("secrets manager %s: %w", op, entities.ErrEscrowKeyStoreUnavailable)
	}
}
