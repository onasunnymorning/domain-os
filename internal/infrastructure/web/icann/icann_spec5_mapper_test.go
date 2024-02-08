package icann

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func TestFromIcannXmlSpec5RegistryToIcannSpec5Label(t *testing.T) {
	registry := &IcannXmlSpec5Registry{
		Registries: []Registry{
			{
				Id: "spec5_1",
				Records: []Record{
					{
						Label1: "51label1",
						Label2: "51label2",
					},
				},
			},
			{
				Id: "spec5_2",
				Records: []Record{
					{
						Label1: "52label1",
						Label2: "52label2",
					},
				},
			},
		},
		Created: "2013-07-03",
		Updated: "2020-02-18",
	}

	expectedLabels := []*entities.Spec5Label{
		{
			Label: "51label1",
			Type:  "spec5_1",
		},
		{
			Label: "51label2",
			Type:  "spec5_1",
		},
		{
			Label: "52label1",
			Type:  "spec5_2",
		},
		{
			Label: "52label2",
			Type:  "spec5_2",
		},
	}

	labels := FromIcannXmlSpec5RegistryToSpec5Label(registry)

	require.Equal(t, 4, len(labels), "Number of labels mismatch")
	require.Equal(t, expectedLabels, labels, "Labels mismatch")
}
