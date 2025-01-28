package icannspec5

import (
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// FromIcannXmlSpec5RegistryToSpec5Label is a struct representing the ICANN XML Spec5 Registry
func FromIcannXmlSpec5RegistryToSpec5Label(registry *IcannXmlSpec5Registry) []*entities.Spec5Label {
	spec5Labels := []*entities.Spec5Label{}
	registries := registry.Registries
	for r := 0; r < len(registries); r++ {
		records := registries[r].Records
		for i := 0; i < len(records); i++ {
			labels := FromICannSpec5Name(&records[i])
			for i := 0; i < len(labels); i++ {
				labels[i].Type = registries[r].Id
				spec5Labels = append(spec5Labels, &labels[i])
			}
		}
	}

	return spec5Labels
}

// FromICannSpec5Name returns a list of IcannSpec5Labels from a Record
func FromICannSpec5Name(record *Record) []entities.Spec5Label {
	labels := []entities.Spec5Label{}
	if record.Label1 != "" {
		labels = append(labels, entities.Spec5Label{
			Label: record.Label1,
		})
	}
	if record.Label2 != "" {
		labels = append(labels, entities.Spec5Label{
			Label: record.Label2,
		})
	}
	return labels
}
