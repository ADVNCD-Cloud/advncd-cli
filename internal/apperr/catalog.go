package apperr

// EPIC A — Auth (examples)
// Add new entries here. Keep codes stable.
var (
	AuthMissingClientID = E("A-AUTH-001", "Missing Google OAuth client ID")
	AuthInvalidScopes   = E("A-AUTH-002", "Invalid OAuth scopes")

	AuthHTTPBuild  = E("A-AUTH-010", "Failed to build HTTP request")
	AuthHTTPDo     = E("A-AUTH-011", "Failed to perform HTTP request")
	AuthJSONDecode = E("A-AUTH-012", "Failed to decode JSON response")

	AuthDeviceFlowFailed    = E("A-AUTH-100", "Google OAuth Device Flow request failed")
	AuthDeviceFlowMalformed = E("A-AUTH-101", "Google OAuth Device Flow response is malformed")

	AuthPKCEGen     = E("A-AUTH-200", "Failed to generate PKCE verifier/challenge")
	AuthStateGen    = E("A-AUTH-201", "Failed to generate OAuth state")
	AuthListen      = E("A-AUTH-202", "Failed to start localhost callback server")
	AuthServe       = E("A-AUTH-203", "Callback server failed")
	AuthAuthURL     = E("A-AUTH-204", "Failed to build OAuth authorization URL")
	AuthAuthTimeout = E("A-AUTH-205", "Login timed out")

	AuthStateMismatch = E("A-AUTH-210", "OAuth state mismatch")
	AuthDenied        = E("A-AUTH-211", "Authorization denied")
	AuthMissingCode   = E("A-AUTH-212", "Missing authorization code in callback")

	AuthTokenExchange = E("A-AUTH-300", "Failed to exchange authorization code for tokens")
	AuthUserInfo      = E("A-AUTH-301", "Failed to fetch user info")

	AuthBrokerUnavailable = E("A-AUTH-500", "Auth broker is unavailable")
	AuthPollTimeout       = E("A-AUTH-501", "Login polling timed out")
	AuthPollDenied        = E("A-AUTH-502", "Login request was denied")
	AuthPollExpired       = E("A-AUTH-503", "Login session expired")
	AuthInvalidRefresh    = E("A-AUTH-504", "Invalid app refresh token")
	AuthBrokerProtocol    = E("A-AUTH-505", "Auth broker response is malformed")
)

// byCode enables restoring an error by code (e.g., logs, dashboard, remote agent).
var byCode = map[string]Entry{
	AuthMissingClientID.Code: AuthMissingClientID,
	AuthInvalidScopes.Code:   AuthInvalidScopes,

	AuthHTTPBuild.Code:  AuthHTTPBuild,
	AuthHTTPDo.Code:     AuthHTTPDo,
	AuthJSONDecode.Code: AuthJSONDecode,

	AuthDeviceFlowFailed.Code:    AuthDeviceFlowFailed,
	AuthDeviceFlowMalformed.Code: AuthDeviceFlowMalformed,

	AuthPKCEGen.Code:     AuthPKCEGen,
	AuthStateGen.Code:    AuthStateGen,
	AuthListen.Code:      AuthListen,
	AuthServe.Code:       AuthServe,
	AuthAuthURL.Code:     AuthAuthURL,
	AuthAuthTimeout.Code: AuthAuthTimeout,

	AuthStateMismatch.Code: AuthStateMismatch,
	AuthDenied.Code:        AuthDenied,
	AuthMissingCode.Code:   AuthMissingCode,
	
	AuthTokenExchange.Code: AuthTokenExchange,
	AuthUserInfo.Code:      AuthUserInfo,

	AuthBrokerUnavailable.Code: AuthBrokerUnavailable,
	AuthPollTimeout.Code:       AuthPollTimeout,
	AuthPollDenied.Code:        AuthPollDenied,
	AuthPollExpired.Code:       AuthPollExpired,
	AuthInvalidRefresh.Code:    AuthInvalidRefresh,
	AuthBrokerProtocol.Code:    AuthBrokerProtocol,
}

// FromCode returns a catalog entry for a known code, otherwise a generic entry.
func FromCode(code string) Entry {
	if e, ok := byCode[code]; ok {
		return e
	}
	return E(code, "Unknown error")
}
