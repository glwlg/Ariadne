//go:build !windows

package platform

func enrichFileSearchServiceStatus(status FileSearchStatus) FileSearchStatus {
	return status
}
