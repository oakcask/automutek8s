package tools

import (
	"regexp"
	"strings"
)

// SubdomainRegexp is regular expression for dns subdomain name;
// used for k8s resource name.
// cf. https://tools.ietf.org/html/rfc1035
var SubdomainRegexp = regexp.MustCompile(`[A-Za-z](?:[A-Za-z0-9\-]*[A-Za-z0-9])?`)

var k8sSecretNameValidator = regexp.MustCompile(strings.Join([]string{`\A`, SubdomainRegexp.String(), `\z`}, ""))

// IsValidK8sMetadataName validates that text is
// valid subdomain name which can be used as Kubernates resource name.
func IsValidK8sMetadataName(text string) bool {
	return k8sSecretNameValidator.MatchString(text)
}
