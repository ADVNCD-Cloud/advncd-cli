package apperr

const (
	ErrMissingClientID     = "A-AUTH-001"
	ErrInvalidScopes       = "A-AUTH-002"

	ErrHTTPBuild           = "A-AUTH-010"
	ErrHTTPDo              = "A-AUTH-011"
	ErrJSONDecode          = "A-AUTH-012"

	ErrDeviceFlowFailed    = "A-AUTH-100"
	ErrDeviceFlowMalformed = "A-AUTH-101"
)

func CatalogMessage(code string) string {
	switch code {
	case ErrMissingClientID:
		return "Missing Google OAuth client ID"
	case ErrInvalidScopes:
		return "Invalid OAuth scopes"
	case ErrHTTPBuild:
		return "Failed to build HTTP request"
	case ErrHTTPDo:
		return "Failed to perform HTTP request"
	case ErrJSONDecode:
		return "Failed to decode JSON response"
	case ErrDeviceFlowFailed:
		return "Google OAuth Device Flow request failed"
	case ErrDeviceFlowMalformed:
		return "Google OAuth Device Flow response is malformed"
	default:
		return "Unknown error"
	}
}