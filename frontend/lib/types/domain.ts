export interface DomainStatus {
  OK?: boolean;
  Inactive?: boolean;
  ClientTransferProhibited?: boolean;
  ClientUpdateProhibited?: boolean;
  ClientDeleteProhibited?: boolean;
  ClientRenewProhibited?: boolean;
  ClientHold?: boolean;
  ServerTransferProhibited?: boolean;
  ServerUpdateProhibited?: boolean;
  ServerDeleteProhibited?: boolean;
  ServerRenewProhibited?: boolean;
  ServerHold?: boolean;
  PendingCreate?: boolean;
  PendingRenew?: boolean;
  PendingTransfer?: boolean;
  PendingUpdate?: boolean;
  PendingRestore?: boolean;
  PendingDelete?: boolean;
}

export interface DomainRGPStatus {
  addPeriodEnd?: string;
  renewPeriodEnd?: string;
  autoRenewPeriodEnd?: string;
  transferLockPeriodEnd?: string;
  redemptionPeriodEnd?: string;
  purgeDate?: string;
}

export interface DomainListItem {
  Name: string;
  TLDName: string;
  ClID: string;
  ExpiryDate?: string;
  CreatedAt?: string;
  UpdatedAt?: string;
  Status?: DomainStatus;
  RGPStatus?: DomainRGPStatus;
}

// Detail types extend beyond list item
export interface HostStatus {
  OK?: boolean;
  Linked?: boolean;
  PendingCreate?: boolean;
  PendingDelete?: boolean;
  PendingUpdate?: boolean;
  PendingTransfer?: boolean;
  ClientDeleteProhibited?: boolean;
  ClientUpdateProhibited?: boolean;
  ServerDeleteProhibited?: boolean;
  ServerUpdateProhibited?: boolean;
}

export interface HostItem {
  RoID?: string;
  Name: string;
  Addresses?: string[]; // backend sends as IP strings
  ClID?: string;
  CrRr?: string;
  UpRr?: string;
  InBailiwick?: boolean;
  CreatedAt?: string;
  UpdatedAt?: string;
  Status?: HostStatus;
}

export interface DomainGrandFathering {
  Amount?: number;
  Currency?: string;
  ExpiryCondition?: string; // transfer | delete | date
  VoidDate?: string | null;
}

export interface DomainDetail extends DomainListItem {
  RoID?: string;
  OriginalName?: string;
  UName?: string;
  CrRr?: string;
  UpRr?: string;
  DropCatch?: boolean;
  RenewedYears?: number;
  AuthInfo?: string;
  RegistrantID?: string;
  AdminID?: string;
  TechID?: string;
  BillingID?: string;
  GrandFathering?: DomainGrandFathering;
  Hosts?: HostItem[];
}

export interface DomainListParams {
  pagesize?: number;
  cursor?: string;
  clid_equals?: string;
  tld_equals?: string;
  name_equals?: string;
  name_like?: string;
}

export interface DomainListResponse {
  Data: DomainListItem[];
  Meta?: {
    PageCursor?: string;
    PageSize?: number;
    NextLink?: string;
    Filter?: Record<string, unknown>;
  };
}

export interface DomainCountResponse {
  ObjectType: string;
  Count: number;
  Timestamp: string;
  Filter?: Record<string, unknown>;
}

// Payload aligned with commands.CreateDomainCommand (admin/import create)
export interface DomainCreateRequest {
  // Required
  Name: string;
  ClID: string;
  AuthInfo: string;
  // time.RFC3339 string expected by backend
  ExpiryDate: string;

  // Optional fields commonly used
  RegistrantID?: string;
  AdminID?: string;
  TechID?: string;
  BillingID?: string;
  EnforcePhasePolicy?: boolean;

  // Less common/advanced (kept optional for completeness)
  RoID?: string;
  OriginalName?: string;
  UName?: string;
  CrRr?: string;
  UpRr?: string;
  DropCatch?: boolean;
  RenewedYears?: number;
  CreatedAt?: string;
  UpdatedAt?: string;
  // Status and RGPStatus types are complex; omit from UI for now.
}
