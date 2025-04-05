package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

func TestSpec5_TableName(t *testing.T) {
	s := Spec5Label{}
	require.Equal(t, "spec5_labels", s.TableName())
}

type Spec5Suite struct {
	suite.Suite
	db *gorm.DB
}

func TestSpec5Suite(t *testing.T) {
	suite.Run(t, new(Spec5Suite))
}

func (s *Spec5Suite) SetupSuite() {
	s.db = setupTestDB()
	NewGormTLDRepo(s.db)
}

func (s *Spec5Suite) TestUpdateAll() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewSpec5Repository(tx)

	labels := []*entities.Spec5Label{
		{
			Label: "label1",
			Type:  "type1",
		},
		{
			Label: "label2",
			Type:  "type2",
		},
	}

	err := repo.UpdateAll(context.Background(), labels)
	require.NoError(s.T(), err)
}

func (s *Spec5Suite) TestReadAll() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewSpec5Repository(tx)

	labels := []*entities.Spec5Label{
		{
			Label: "label1",
			Type:  "type1",
		},
		{
			Label: "label2",
			Type:  "type2",
		},
	}

	err := repo.UpdateAll(context.Background(), labels)
	require.NoError(s.T(), err)

	readLabels, cursor, err := repo.List(context.Background(), queries.ListItemsQuery{
		PageSize:   25,
		PageCursor: "",
	})
	require.Equal(s.T(), "", cursor)
	require.NoError(s.T(), err)
	for i, label := range labels {
		require.Equal(s.T(), label.Label, readLabels[i].Label)
		require.Equal(s.T(), label.Type, readLabels[i].Type)
	}
}
