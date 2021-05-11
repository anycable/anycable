package version

import "fmt"

var (
	version    string
	proVersion string
	ossVersion string
	modifier   string
	sha        string
)

func init() {
	if version == "" {
		version = "1.5.0"
	}

	if proVersion != "" {
		ossVersion = version
		version = proVersion
	}

	if modifier != "" {
		version = version + "-" + modifier
	}

	if sha != "" {
		version = version + "-" + sha
	}
}

// Version returns the current program version
func Version() string {
	if ossVersion != "" {
		return fmt.Sprintf("%s (based on OSS v%s)", version, ossVersion)
	}

	return version
}

// SHA returns the build commit sha
func SHA() string {
	return sha
}
