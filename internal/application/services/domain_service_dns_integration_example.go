package services

// Example: How to integrate DNS event publishing into your existing Domain Service

import (
	"context"
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/dnsevents"
	"gorm.io/gorm"
)

// This is an example of how to extend your existing DomainService
// to publish DNS events. You would add these calls to your actual
// service methods.

// Example: CreateDomain with DNS events
func ExampleCreateDomainWithDNS(
	ctx context.Context,
	db *gorm.DB,
	dnsPublisher *dnsevents.EventPublisher,
	domain *entities.Domain,
	tldName string,
) error {
	// Start transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// 1. Create domain (your existing logic)
		if err := tx.Create(domain).Error; err != nil {
			return err
		}

		// 2. If domain has hosts, publish DNS events
		if len(domain.Hosts) > 0 {
			hostNames := make([]string, 0, len(domain.Hosts))
			for _, host := range domain.Hosts {
				hostNames = append(hostNames, host.Name.String())
			}

			// Publish NS record additions
			err := dnsPublisher.PublishDomainNSRecords(
				ctx,
				tx, // Pass transaction!
				tldName,
				domain.Name.String(),
				hostNames,
				dnsevents.DNSChangeTypeAdd,
				"CreateDomain",
			)
			if err != nil {
				return fmt.Errorf("failed to publish DNS events: %w", err)
			}
		}

		// Transaction commits automatically if no error
		return nil
	})
}

// Example: AddHostToDomain with DNS events
func ExampleAddHostToDomainWithDNS(
	ctx context.Context,
	db *gorm.DB,
	dnsPublisher *dnsevents.EventPublisher,
	domainName string,
	hostName string,
	tldName string,
) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 1. Add host association (your existing logic)
		// ... domain_hosts INSERT ...

		// 2. Check if domain is active and TLD has DNS enabled
		var isActive bool
		var dnsEnabled bool
		err := tx.Raw(`
			SELECT d.inactive = false AND d.pending_delete = false, t.enable_dns
			FROM domains d
			JOIN tlds t ON t.name = d.tld_name
			WHERE d.name = ?
		`, domainName).Row().Scan(&isActive, &dnsEnabled)

		if err != nil {
			return err
		}

		// 3. If active and DNS enabled, publish NS record addition
		if isActive && dnsEnabled {
			err := dnsPublisher.PublishDomainNSRecords(
				ctx,
				tx,
				tldName,
				domainName,
				[]string{hostName},
				dnsevents.DNSChangeTypeAdd,
				"AddHostToDomain",
			)
			if err != nil {
				return fmt.Errorf("failed to publish DNS event: %w", err)
			}
		}

		return nil
	})
}

// Example: Add glue records for in-bailiwick host
func ExampleAddGlueRecordsWithDNS(
	ctx context.Context,
	db *gorm.DB,
	dnsPublisher *dnsevents.EventPublisher,
	hostName string,
	address string,
	ipVersion int, // 4 or 6
	zoneName string,
) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 1. Insert into host_addresses (your existing logic)
		// ... INSERT ...

		// 2. Check if host is in-bailiwick
		var inBailiwick bool
		err := tx.Raw(`
			SELECT in_bailiwick FROM hosts WHERE name = ?
		`, hostName).Row().Scan(&inBailiwick)

		if err != nil {
			return err
		}

		// 3. If in-bailiwick, publish glue record
		if inBailiwick {
			addresses := map[string]int{address: ipVersion}
			err := dnsPublisher.PublishGlueRecords(
				ctx,
				tx,
				zoneName,
				hostName,
				addresses,
				dnsevents.DNSChangeTypeAdd,
				"AddHostAddress",
			)
			if err != nil {
				return fmt.Errorf("failed to publish DNS event: %w", err)
			}
		}

		return nil
	})
}

// Example: Delete domain with DNS cleanup
func ExampleDeleteDomainWithDNS(
	ctx context.Context,
	db *gorm.DB,
	dnsPublisher *dnsevents.EventPublisher,
	domainName string,
	tldName string,
) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 1. Get all hosts for this domain before deletion
		var hostNames []string
		err := tx.Raw(`
			SELECT h.name
			FROM hosts h
			JOIN domain_hosts dh ON dh.host_ro_id = h.ro_id
			JOIN domains d ON d.ro_id = dh.domain_ro_id
			WHERE d.name = ?
		`, domainName).Pluck("name", &hostNames).Error

		if err != nil {
			return err
		}

		// 2. Publish NS record deletions BEFORE deleting domain
		if len(hostNames) > 0 {
			err := dnsPublisher.PublishDomainNSRecords(
				ctx,
				tx,
				tldName,
				domainName,
				hostNames,
				dnsevents.DNSChangeTypeDelete,
				"DeleteDomain",
			)
			if err != nil {
				return fmt.Errorf("failed to publish DNS events: %w", err)
			}
		}

		// 3. Delete domain (your existing logic)
		if err := tx.Where("name = ?", domainName).Delete(&entities.Domain{}).Error; err != nil {
			return err
		}

		return nil
	})
}
