// Entity types shared by the type modules.

/**
 * User profile information
 */
export interface User {
  id: number;
  email: string;
  username: string;
  language: string;
  is_admin: boolean;
  created_at: string;
  updated_at: string;
  // Whether the account has two-factor authentication on (never the secret).
  two_factor_enabled?: boolean;
  // Present only on the /users/me response (CurrentUserResponse), not in admin lists.
  custom_field_names?: string[];
  // null means the user has never configured it; apply DEFAULT_ENABLED_CONTACT_FIELDS.
  enabled_contact_fields?: string[] | null;
}
