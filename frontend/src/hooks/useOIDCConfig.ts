import { useState, useEffect } from 'react';
import { API_BASE_URL } from '../auth';
import type { components } from '../types/generated/api';

// The response of GET /auth/oidc/config, generated from the backend's OpenAPI document
type OIDCConfigResponse = components['schemas']['OIDCConfigOutputBody'];

interface OIDCConfig {
  enabled: boolean;
  provider_name: string;
}

export function useOIDCConfig(): OIDCConfig {
  const [config, setConfig] = useState<OIDCConfig>({ enabled: false, provider_name: 'SSO' });

  useEffect(() => {
    fetch(`${API_BASE_URL}/auth/oidc/config`)
      .then((res) => (res.ok ? (res.json() as Promise<OIDCConfigResponse>) : null))
      .then((data) => {
        if (data) {
          setConfig({
            enabled: data.enabled,
            provider_name: data.provider_name || 'SSO',
          });
        }
      })
      .catch(() => {
        // OIDC config is best-effort; if it fails, SSO button stays hidden
      });
  }, []);

  return config;
}
