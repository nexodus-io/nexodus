export interface RefreshTokenResponse {
  access_token?: string;
  refresh_token?: string;
}

export interface JwtDecodeExp {
  exp: number;
}
