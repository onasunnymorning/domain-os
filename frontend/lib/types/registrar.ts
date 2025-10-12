/**
 * IANA Registrar Types
 * Based on backend entities in internal/domain/entities/ianaRegistrar.go
 */

/**
 * IANA Registrar Status enum
 * Represents the accreditation status of an IANA registrar
 */
export enum IANARegistrarStatus {
  Accredited = "Accredited",
  Reserved = "Reserved",
  Terminated = "Terminated",
  Unknown = "Unknown",
}

/**
 * IANA Registrar entity
 * Represents a registrar from the IANA registry
 */
export interface IANARegistrar {
  GurID: number;
  Name: string;
  Status: IANARegistrarStatus;
  RdapURL: string;
  CreatedAt: string;
}

/**
 * Query parameters for listing IANA registrars
 */
export interface IANARegistrarListParams {
  pagesize?: number;
  cursor?: string;
  name_like?: string;
  status?: IANARegistrarStatus | string;
}

/**
 * Response for IANA registrar list endpoint
 */
export interface IANARegistrarListResponse {
  Data: IANARegistrar[];
  Meta?: {
    Cursor: string;
    Count: number;
    PageSize: number;
    NextLink?: string;
  };
}

/**
 * Response for count endpoint
 */
export interface IANARegistrarCountResponse {
  ObjectType: string;
  Count: number;
  Timestamp: string;
}

/**
 * System Registrar Status enum
 * Based on backend entities in internal/domain/entities/registrar.go
 */
export enum RegistrarStatus {
  OK = "ok",
  Readonly = "readonly",
  Terminated = "terminated",
}

/**
 * Postal info type for system registrars
 */
export type RegistrarPostalInfoType = "int" | "loc";

/**
 * System Registrar entity (basic list item)
 * This is a simplified version used in list views
 */
export interface RegistrarListItem {
  ClID: string;
  Name: string;
  GurID: number;
  Status: RegistrarStatus;
  Autorenew: boolean;
}

/**
 * Full System Registrar entity
 * Complete registrar object with all details
 */
export interface Registrar {
  ClID: string;
  Name: string;
  NickName: string;
  GurID: number;
  Status: RegistrarStatus;
  IANAStatus: IANARegistrarStatus;
  Autorenew: boolean;
  PostalInfo: any[]; // Simplified for now
  Voice: string;
  Fax: string;
  Email: string;
  URL: string;
  WhoisInfo: any; // Simplified for now
  RdapBaseURL: string;
  CreatedAt: string;
  UpdatedAt: string;
  TLDs?: any[]; // Simplified for now
}

/**
 * Query parameters for listing system registrars
 */
export interface RegistrarListParams {
  pagesize?: number;
  cursor?: string;
}

/**
 * Response for system registrar list endpoint
 */
export interface RegistrarListResponse {
  Data: RegistrarListItem[];
  Meta?: {
    Cursor: string;
    Count: number;
    PageSize: number;
    NextLink?: string;
  };
}

/**
 * Response for registrar count endpoint
 */
export interface RegistrarCountResponse {
  ObjectType: string;
  Count: number;
  Timestamp: string;
}

/**
 * Response for sync operations
 */
export interface SyncResult {
  message: string;
  success: boolean;
}
