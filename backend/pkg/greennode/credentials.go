package greennode

// IAMCredentials holds the GreenNode IAM client credentials used for the
// OAuth2 client_credentials grant.
type IAMCredentials struct {
	ClientID     string
	ClientSecret string
}

// NewIAMCredentials resolves credentials from (in priority order) the given
// values, environment variables (GREENNODE_CLIENT_ID / GREENNODE_CLIENT_SECRET)
// and the .greennode.json config file (client_id / client_secret keys).
func NewIAMCredentials(clientID, clientSecret string) IAMCredentials {
	if clientID == "" {
		clientID = GetConfigValue("GREENNODE_CLIENT_ID", "")
	}
	if clientID == "" {
		clientID = GetConfigValue("client_id", "")
	}
	if clientSecret == "" {
		clientSecret = GetConfigValue("GREENNODE_CLIENT_SECRET", "")
	}
	if clientSecret == "" {
		clientSecret = GetConfigValue("client_secret", "")
	}
	return IAMCredentials{ClientID: clientID, ClientSecret: clientSecret}
}

// IsAvailable reports whether both client_id and client_secret are present.
func (c IAMCredentials) IsAvailable() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

// Require returns ErrCredentialsMissing when credentials are incomplete.
func (c IAMCredentials) Require() error {
	if !c.IsAvailable() {
		return ConfigError(ErrCredentialsMissing.Error(), ErrCredentialsMissing)
	}
	return nil
}
