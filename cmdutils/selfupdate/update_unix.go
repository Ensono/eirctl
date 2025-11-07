//go:build !windows
// +build !windows

package selfupdate

func EnrichFinalLink(link string) string {
	return link
}
