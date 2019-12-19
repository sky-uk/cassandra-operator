package imageversion

import "strings"

// Version extracts the value of a version tag from an image:version string. If no tag exists, an empty
// string will be returned instead.
func Version(imageAndVersion *string) string {
	if imageAndVersion == nil {
		return ""
	}

	versionSeparatorIndex := strings.LastIndex(*imageAndVersion, ":")
	if versionSeparatorIndex < 0 || versionSeparatorIndex == len(*imageAndVersion)-1 {
		return ""
	}

	return (*imageAndVersion)[versionSeparatorIndex+1:]
}

// RepositoryPath extracts a repository path from an image:version string. If no repository path exists, an empty string
// will be returned instead.
func RepositoryPath(imageAndVersion *string) string {
	if imageAndVersion == nil {
		return ""
	}

	repositoryPathEnd := strings.LastIndex(*imageAndVersion, "/")
	if repositoryPathEnd < 1 {
		return ""
	}

	return (*imageAndVersion)[:repositoryPathEnd]
}
