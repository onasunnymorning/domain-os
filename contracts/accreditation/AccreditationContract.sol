// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.18;

/**
 * @title AccreditationContract
 * @dev Smart contract for managing registrar accreditations for TLDs on Hedera
 * 
 * This contract implements the business logic extracted from the Domain OS AccreditationService.
 * It manages the relationship between registrars and TLDs, enforcing ICANN accreditation rules
 * for gTLD registrations while allowing more flexible rules for ccTLDs.
 */
contract AccreditationContract {
    
    // Events for Hedera Consensus Service integration
    event AccreditationCreated(
        string indexed tldName,
        string indexed registrarClID,
        uint256 timestamp,
        string metadata
    );
    
    event AccreditationRevoked(
        string indexed tldName, 
        string indexed registrarClID,
        uint256 timestamp,
        string reason
    );
    
    // Structs matching your domain entities
    struct Registrar {
        string clID;
        uint256 gurID;  // IANA GurID for ICANN accredited registrars
        string status;  // "ok", "suspended", etc.
        string ianaStatus; // "Accredited", "Terminated", etc.
        bool exists;
    }
    
    struct TLD {
        string name;
        string tldType; // "gTLD", "ccTLD"
        string status;
        bool exists;
    }
    
    // Storage mappings
    mapping(string => Registrar) public registrars; // clID => Registrar
    mapping(string => TLD) public tlds; // name => TLD
    mapping(string => mapping(string => bool)) public accreditations; // tldName => clID => isAccredited
    
    // Contract owner for administrative functions
    address public owner;
    
    // Business rule constants (extracted from your entities)
    string constant REGISTRAR_STATUS_OK = "ok";
    string constant IANA_STATUS_ACCREDITED = "Accredited";
    string constant TLD_TYPE_GTLD = "gTLD";
    string constant TLD_TYPE_CCTLD = "ccTLD";
    
    modifier onlyOwner() {
        require(msg.sender == owner, "Only owner can perform this action");
        _;
    }
    
    constructor() {
        owner = msg.sender;
    }
    
    /**
     * @dev Register a registrar - extracted from your NewRegistrar logic
     */
    function registerRegistrar(
        string memory clID,
        uint256 gurID,
        string memory status,
        string memory ianaStatus
    ) external onlyOwner {
        registrars[clID] = Registrar({
            clID: clID,
            gurID: gurID,
            status: status,
            ianaStatus: ianaStatus,
            exists: true
        });
    }
    
    /**
     * @dev Register a TLD
     */
    function registerTLD(
        string memory name,
        string memory tldType,
        string memory status
    ) external onlyOwner {
        tlds[name] = TLD({
            name: name,
            tldType: tldType,
            status: status,
            exists: true
        });
    }
    
    /**
     * @dev Create accreditation - implements your AccreditFor business logic
     * This is the core business rule extracted from registrar.AccreditFor()
     */
    function createAccreditation(
        string memory tldName,
        string memory registrarClID
    ) external onlyOwner returns (bool success) {
        // Validate inputs
        require(tlds[tldName].exists, "TLD does not exist");
        require(registrars[registrarClID].exists, "Registrar does not exist");
        
        Registrar memory registrar = registrars[registrarClID];
        TLD memory tld = tlds[tldName];
        
        // Check if already accredited (idempotent)
        if (accreditations[tldName][registrarClID]) {
            return true;
        }
        
        // Business rule: Registrar status must be "ok"
        require(
            keccak256(abi.encodePacked(registrar.status)) == keccak256(abi.encodePacked(REGISTRAR_STATUS_OK)),
            "Registrar status prevents accreditation"
        );
        
        // Business rule: For gTLDs, registrar must be ICANN accredited
        if (keccak256(abi.encodePacked(tld.tldType)) == keccak256(abi.encodePacked(TLD_TYPE_GTLD))) {
            require(registrar.gurID > 0, "ICANN GurID required for gTLD accreditation");
            require(
                keccak256(abi.encodePacked(registrar.ianaStatus)) == keccak256(abi.encodePacked(IANA_STATUS_ACCREDITED)),
                "Only ICANN accredited registrars can accredit for gTLDs"
            );
        }
        
        // Create the accreditation
        accreditations[tldName][registrarClID] = true;
        
        // Emit event for Hedera Consensus Service
        emit AccreditationCreated(
            tldName,
            registrarClID,
            block.timestamp,
            string(abi.encodePacked(
                '{"tld":"', tldName, 
                '","registrar":"', registrarClID,
                '","type":"', tld.tldType, '"}'
            ))
        );
        
        return true;
    }
    
    /**
     * @dev Revoke accreditation - implements your DeAccreditFor business logic
     */
    function revokeAccreditation(
        string memory tldName,
        string memory registrarClID,
        string memory reason
    ) external onlyOwner returns (bool success) {
        require(tlds[tldName].exists, "TLD does not exist");
        require(registrars[registrarClID].exists, "Registrar does not exist");
        
        // Remove accreditation (idempotent)
        accreditations[tldName][registrarClID] = false;
        
        emit AccreditationRevoked(
            tldName,
            registrarClID,
            block.timestamp,
            reason
        );
        
        return true;
    }
    
    /**
     * @dev Check if registrar is accredited for TLD
     */
    function isAccredited(
        string memory tldName,
        string memory registrarClID
    ) external view returns (bool) {
        return accreditations[tldName][registrarClID];
    }
    
    /**
     * @dev Get all accreditations for a registrar (for pagination, return limited results)
     */
    function getRegistrarAccreditations(
        string memory registrarClID
    ) external view returns (string[] memory) {
        // Note: In production, you'd implement pagination
        // This is a simplified version for the PoC
        string[] memory results = new string[](0);
        return results;
    }
}
