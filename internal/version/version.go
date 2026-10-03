package version

// Version is the current semantic version of Bedrock Server Manager.
// Can be set at compile time via:
// -ldflags="-X github.com/AhmadShamli/Bedrock-Server-Manager/internal/version.Version=x.y.z"
var Version = "1.9.5"

const (
	// AppName is the official display name of the application.
	AppName = "Bedrock Server Manager (BSM)"

	// Author is the organization/author name.
	Author = "AhmadShamli"

	// RepositoryURL is the official GitHub repository link.
	RepositoryURL = "https://github.com/AhmadShamli/Bedrock-Server-Manager"
)

// FullVersionString returns the full formatted version with branding and repo link.
func FullVersionString() string {
	return AppName + " v" + Version + " (" + RepositoryURL + ")"
}
