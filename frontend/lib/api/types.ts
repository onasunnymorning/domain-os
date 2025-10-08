// API Response Types
export interface ListResponse<T> {
  Data: T[];
  Meta?: {
    PageCursor?: string;
    PageSize?: number;
    NextLink?: string;
    Filter?: {
      RyidLike?: string;
      NameLike?: string;
      EmailLike?: string;
    };
  };
}

export interface ApiError {
  error: string;
}

// Registry Operator Types
export interface CreateRegistryOperatorCommand {
  RyID: string;
  Name: string;
  Email: string;
  URL?: string;
  Voice?: string;
  Fax?: string;
}

export interface RegistryOperator {
  RyID: string;
  Name: string;
  Email: string;
  URL?: string;
  Voice?: string;
  Fax?: string;
  CreatedAt?: string;
  UpdatedAt?: string;
  TLDs?: any;
  PremiumLists?: any[];
}

// List Query Parameters
export interface ListQueryParams {
  pagesize?: number;
  pagecursor?: string;
  ryid_like?: string;
  name_like?: string;
  email_like?: string;
}
