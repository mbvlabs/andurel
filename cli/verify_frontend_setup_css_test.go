package cli

import (
	"path/filepath"
	"testing"
)

func TestFrontendCSSStaticGraph(t *testing.T) {
	for _, test := range []struct {
		name, entry, helper string
		wantFail, wantBlock bool
	}{
		{"direct", "import '../../css/base.css'\n", "", false, false},
		{"helper", "import './styles'\n", "import '../../css/base.css'\n", false, false},
		{"reexport", "export * from './styles'\n", "import '../../css/base.css'\nexport const x = 1\n", false, false},
		{"comments", "// import '../../css/base.css'\n/* import '../../css/base.css' */", "", true, false},
		{"string", `const unused = "import '../../css/base.css'"`, "", true, false},
		{"type import", `import type { Styles } from '../../css/base.css'`, "", true, false},
		{"type-only specifier", `import { type Styles } from './styles'`, "import '../../css/base.css'", true, false},
		{"type-only reexport", `export { type Styles } from './styles'`, "import '../../css/base.css'", true, false},
		{"dynamic", `if (false) import('../../css/base.css')`, "", false, true},
		{"unused helper", "export const app = 1\n", "import '../../css/base.css'\n", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "css/base.css", "@import 'tailwindcss';\n")
			writeTestFile(t, root, "resources/js/app.ts", test.entry)
			if test.helper != "" {
				writeTestFile(t, root, "resources/js/styles.ts", test.helper)
			}
			violations, blocked := verifyFrontendCSS(root, []string{filepath.Join(root, "resources/js/app.ts")})
			if (len(violations) > 0) != test.wantFail || (blocked != "") != test.wantBlock {
				t.Fatalf("violations=%#v blocked=%q", violations, blocked)
			}
		})
	}
}

func TestFrontendCSSTsconfigAliasDoesNotProveViteResolution(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "css/base.css", "")
	writeTestFile(t, root, "tsconfig.json", `{"compilerOptions":{"paths":{"@/*":["resources/js/*"]}}}`)
	writeTestFile(t, root, "resources/js/app.ts", "import '@/styles'\n")
	writeTestFile(t, root, "resources/js/styles.ts", "import '../../css/base.css'\n")
	violations, blocked := verifyFrontendCSS(root, []string{"resources/js/app.ts"})
	if len(violations) != 0 || blocked == "" {
		t.Fatalf("violations=%#v blocked=%q; tsconfig-only alias must not prove Vite CSS wiring", violations, blocked)
	}
}

func TestFrontendCSSMissingSharedEntry(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "resources/js/app.ts", "import '../../css/base.css'\n")
	violations, blocked := verifyFrontendCSS(root, []string{"resources/js/app.ts"})
	if len(violations) != 1 || violations[0].File != "css/base.css" || blocked != "" {
		t.Fatalf("violations=%#v blocked=%q", violations, blocked)
	}
}

func TestFrontendCSSChecksEveryBuildEntry(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "css/base.css", "")
	writeTestFile(t, root, "resources/js/app.ts", "import '../../css/base.css'\n")
	writeTestFile(t, root, "resources/js/other.ts", "export const other = 1\n")
	violations, blocked := verifyFrontendCSS(root, []string{"resources/js/app.ts", "resources/js/other.ts"})
	if len(violations) != 1 || violations[0].File != "resources/js/other.ts" || blocked != "" {
		t.Fatalf("violations=%#v blocked=%q", violations, blocked)
	}
}
