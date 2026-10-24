package redact

import (
	"strings"
	"testing"
)

func TestSensitiveScalarLexicalContinuations(t *testing.T) {
	for _, body := range []string{
		"password: prefix\n  CONFIG_SECRET\nsafe: final-visible",
		"password: &pw |\n  CONFIG_SECRET\nsafe: final-visible",
		"password: !!str &pw >-\n  CONFIG_SECRET\nsafe: final-visible",
		"password: &pw !!str 'first\n  CONFIG_SECRET\n  last'\nsafe: final-visible",
		"env API_TOKEN='first\nCONFIG_SECRET\nlast' tool\nsafe: final-visible",
		"env MYSQL_PWD=\"first\nCONFIG_SECRET\nlast\" tool\nsafe: final-visible",
		"curl --authorization 'Bearer first\nCONFIG_SECRET\nlast'\nsafe: final-visible",
		"mysql -p'first\nCONFIG_SECRET\nlast'\nsafe: final-visible",
		"mysqldump -p\"first\nCONFIG_SECRET\nlast\"\nsafe: final-visible",
		"mariadb -p 'first\nCONFIG_SECRET\nlast'\nsafe: final-visible",
	} {
		assertNormalFormProjections(t, strings.Split("initial-visible\n"+body, "\n"), "CONFIG_SECRET")
	}
}

func TestScalarSyntaxStillEndsAtItsOwnBoundary(t *testing.T) {
	for _, body := range []string{
		"password: prefix\n  hidden\nsafe: ordinary-visible",
		"password: &pw |\n  hidden\nsafe: ordinary-visible",
		"env API_TOKEN='closed' tool\nordinary-visible",
		"curl --token 'closed'\nordinary-visible",
	} {
		if safe := Text(body); !strings.Contains(safe, "ordinary-visible") {
			t.Fatalf("ordinary adjacent text hidden: %s", safe)
		}
	}
}
