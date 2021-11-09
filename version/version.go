package version

var (
	version  string
	modifier string
	sha      string
)

func init() {
	if version == "" {
		version = "1.1.1"
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
	return version
}

// SHA returns the build commit sha
func SHA() string {
	return sha
}
