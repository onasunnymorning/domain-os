package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

const (
	ckRar = "ckConstraintRar"
	ckRy  = "CkConstraintRy"
)

// ckTLDs are the TLDs these cases file names under. "uk" is among them so that
// a name two levels below it is refused by the CHECK rather than by the TLD
// foreign key.
var ckTLDs = []string{"paco", "gza", "ac.uk", "uk"}

// ConstraintSuite covers the CHECK that ties a name to its TLD. The interesting
// case is not the bad INSERT but the bad UPDATE: #415 created no rows, it moved
// 3,007 existing ones into another TLD.
type ConstraintSuite struct {
	suite.Suite
	db *gorm.DB
}

func TestConstraintSuite(t *testing.T) {
	suite.Run(t, new(ConstraintSuite))
}

func (s *ConstraintSuite) SetupSuite() {
	s.db = setupTestDB()
	s.Require().NoError(AutoMigrate(s.db))

	ctx := context.Background()
	s.TearDownSuite() // leftovers from an interrupted run

	roRepo := NewGORMRegistryOperatorRepository(s.db)
	ro, err := entities.NewRegistryOperator(ckRy, ckRy, "me@my.email")
	s.Require().NoError(err)
	_, err = roRepo.Create(ctx, ro)
	s.Require().NoError(err)

	rarRepo := NewGormRegistrarRepository(s.db)
	rar, err := entities.NewRegistrar(ckRar, "ckConstraintRarName", "ck@gobro.com", 8199, getValidRegistrarPostalInfoArr())
	s.Require().NoError(err)
	_, err = rarRepo.Create(ctx, rar)
	s.Require().NoError(err)

	tldRepo := NewGormTLDRepo(s.db)
	for _, name := range ckTLDs {
		tld, err := entities.NewTLD(name, ckRy)
		s.Require().NoError(err)
		s.Require().NoError(tldRepo.Create(ctx, tld))
	}
}

func (s *ConstraintSuite) TearDownSuite() {
	ctx := context.Background()
	s.db.Exec(`DELETE FROM domains WHERE cl_id = ?`, ckRar)
	s.db.Exec(`DELETE FROM nndns WHERE tld_name IN ?`, ckTLDs)
	tldRepo := NewGormTLDRepo(s.db)
	for _, name := range ckTLDs {
		_ = tldRepo.DeleteByName(ctx, name)
	}
	_ = NewGormRegistrarRepository(s.db).Delete(ctx, ckRar)
	_ = NewGORMRegistryOperatorRepository(s.db).DeleteByRyID(ctx, ckRy)
}

func (s *ConstraintSuite) TearDownTest() {
	s.db.Exec(`DELETE FROM domains WHERE cl_id = ?`, ckRar)
	s.db.Exec(`DELETE FROM nndns WHERE tld_name IN ?`, ckTLDs)
}

func (s *ConstraintSuite) insertDomain(name, tld string) error {
	return s.db.Exec(`
		INSERT INTO domains (ro_id, name, tld_name, cl_id, auth_info, expiry_date, created_at, updated_at)
		VALUES (nextval(pg_get_serial_sequence('domains','ro_id')), ?, ?, ?, 'x', now(), now(), now())`,
		name, tld, ckRar).Error
}

func (s *ConstraintSuite) insertNNDN(name, tld string) error {
	return s.db.Exec(`
		INSERT INTO nndns (name, tld_name, name_state, created_at, updated_at)
		VALUES (?, ?, 'blocked', now(), now())`, name, tld).Error
}

// The constraint has to exist before any of this means anything.
func (s *ConstraintSuite) TestConstraintsArePresent() {
	for table, name := range NameUnderTLDConstraints {
		var n int64
		s.Require().NoError(s.db.Raw(
			`SELECT count(*) FROM pg_constraint WHERE conrelid = ?::regclass AND conname = ?`,
			table, name).Scan(&n).Error)
		s.Equal(int64(1), n, "%s should carry %s", table, name)
	}
}

// AutoMigrate runs at every pod boot; a second run must not fail or duplicate.
func (s *ConstraintSuite) TestAutoMigrateIsIdempotent() {
	s.Require().NoError(AutoMigrate(s.db))
	var n int64
	s.Require().NoError(s.db.Raw(
		`SELECT count(*) FROM pg_constraint WHERE conrelid = 'domains'::regclass AND conname = ?`,
		NameUnderTLDConstraints["domains"]).Scan(&n).Error)
	s.Equal(int64(1), n)
}

func (s *ConstraintSuite) TestNameMustBeUnderItsTLD() {
	s.Require().NoError(s.insertDomain("ck-constraint-good.paco", "paco"), "a name under its TLD is fine")

	err := s.insertDomain("ck-constraint-bad.paco", "gza")
	s.Require().Error(err, "a .paco name filed under gza must be refused")
	s.Contains(err.Error(), NameUnderTLDConstraints["domains"])

	s.Require().NoError(s.insertDomain("ck-constraint-multi.ac.uk", "ac.uk"), "multi-label TLDs must still work")
	s.Require().Error(s.insertDomain("ck-constraint-multi2.ac.uk", "uk"), "a name two levels under the TLD is not directly under it")
	s.Require().Error(s.insertDomain("ck-constraint-empty.paco", ""), "an empty tld_name must be refused")

	// A name that merely ends in the TLD's letters is not under the TLD.
	s.Require().Error(s.insertDomain("ck-constraintpaco", "paco"))
}

// The incident itself: an UPDATE that re-tags an existing domain.
func (s *ConstraintSuite) TestUpdateCannotMoveADomainToAnotherTLD() {
	s.Require().NoError(s.insertDomain("ck-constraint-keep.paco", "paco"))

	err := s.db.Exec(`UPDATE domains SET tld_name = 'gza' WHERE name = 'ck-constraint-keep.paco'`).Error
	s.Require().Error(err, "this is exactly what #415 did")

	var tld string
	s.Require().NoError(s.db.Raw(`SELECT tld_name FROM domains WHERE name = 'ck-constraint-keep.paco'`).Scan(&tld).Error)
	s.Equal("paco", tld, "the domain must stay in its own TLD")
}

func (s *ConstraintSuite) TestNNDNsAreConstrainedToo() {
	s.Require().NoError(s.insertNNDN("ck-constraint-good.paco", "paco"))
	s.Require().Error(s.insertNNDN("ck-constraint-bad.paco", "gza"))
	s.Require().Error(s.db.Exec(
		`UPDATE nndns SET tld_name = 'gza' WHERE name = 'ck-constraint-good.paco'`).Error)
}

// A clean table gets its constraint promoted from NOT VALID.
func (s *ConstraintSuite) TestValidateReportsAndPromotes() {
	statuses, err := ValidateNameUnderTLDConstraints(context.Background(), s.db)
	s.Require().NoError(err)
	s.Require().Len(statuses, len(NameUnderTLDConstraints))
	for _, st := range statuses {
		s.True(st.Present, "%s: constraint should exist", st.Table)
		s.True(st.Validated, "%s: a clean table should end up validated", st.Table)
		s.Zero(st.Violations, "%s: %v", st.Table, st.Samples)
	}

	// Idempotent: a second run over an already-validated table is a no-op.
	statuses, err = ValidateNameUnderTLDConstraints(context.Background(), s.db)
	s.Require().NoError(err)
	for _, st := range statuses {
		s.True(st.Validated)
	}
}

// The builder is pure string work, so one cheap check that it names the right
// table, carries NOT VALID, and keeps the rule off LIKE.
func TestNameUnderTLDConstraintsStatements(t *testing.T) {
	stmts := nameUnderTLDConstraints()
	require.Len(t, stmts, len(NameUnderTLDConstraints))
	require.Contains(t, stmts[0], "ALTER TABLE domains ADD CONSTRAINT ck_domains_name_under_tld")
	require.Contains(t, stmts[0], "NOT VALID")
	require.Contains(t, stmts[1], "ALTER TABLE nndns ADD CONSTRAINT ck_nndns_name_under_tld")
	for _, stmt := range stmts {
		require.NotContains(t, stmt, "LIKE", "the rule must not use LIKE: _ and % are wildcards there")
	}
}
