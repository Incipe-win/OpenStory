export interface ApiResponse<T> {
  data: T;
}

export interface PaginatedMeta {
  page: number;
  page_size: number;
  total: number;
}

export interface PaginatedResponse<T> {
  data: T[];
  meta: PaginatedMeta;
}

export interface ApiError {
  error: {
    code: string;
    message: string;
  };
}
