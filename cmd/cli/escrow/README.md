# Importing Escrows

* Check if the TLD exist 
<!-- TODO: * Check if TLD.EnableEscrowImport is true -->
* analyze the escrow file
* check if we can import the escrow

```go

tld := getTLD(APIClient)
canImport := tld.EnableEscrowImport
if !canImport {
    log.Errorf("Escrow import is disabled for TLD %s", tld.Name)
}
```

* import the escrow
  * Contacts
  * Hosts
  * Domains

    These follow a common pattern:
    ```go
    contacts := getContacts(escrowFile)

    existingContactCount := getExistingContactCount(APIClient)

    if len(contacts) == existingContactCount {
        // nothing to import, we're in the desired state
        return
    }

    if len(existingContactCount) == 0 {
        // no contacts, exist, try and import as efficiently as possible
        importContactsInBatches(contacts)
    } else {
        // some contacts exist (from a prior failed import?) import one by one and skip over the ones that exist without an error
        importContactsOneByOne(contacts)
    }
    ```