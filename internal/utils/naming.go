package utils

import "strings"

func ExtractUserProjectName(project string) string {
    parts := strings.Split(project, "-")
    if len(parts) > 1 {
        return strings.Join(parts[:len(parts)-1], "-")
    }
    return project
}

// GetBucketName returns the full S3 bucket name for a project
func GetBucketName(project string) string {
    return "auto-mock-" + project
}

// RemoveBucketPrefix removes the 'auto-mock-' prefix from a bucket name if present
func RemoveBucketPrefix(bucket string) string {
    return strings.TrimPrefix(bucket, "auto-mock-")
}