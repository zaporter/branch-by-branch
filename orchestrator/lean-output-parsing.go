package orchestrator

import (
	"strings"
)

/*
Example output:
```
error: build failed
✖ [12/13] Building Test
trace: .> LEAN_PATH=././.lake/packages/Cli/.lake/build/lib:././.lake/packages/batteries/.lake/build/lib:././.lake/build/lib LD_LIBRARY_PATH= /home/ubuntu/.elan/toolchains/leanprover--lean4---v4.15.0/bin/lean ././././Test.lean -R ./././. -o ././.lake/build/lib/Test.olean -i ././.lake/build/lib/Test.ilean -c ././.lake/build/ir/Test.c --json
info: ././././Test.lean:4:26: Try this: exact rfl
info: ././././Test.lean:5:26: Try this: exact Nat.zero_add m
info: ././././Test.lean:6:63: Try this: exact fun n a => succ_pred_one n a
error: ././././Test.lean:7:63: `exact?` could not close the goal. Try `apply?` to see partial suggestions.
error: Lean exited with code 1
Some required builds logged failures:
- Test
```
Strip lean output breaks out the output by finding line prefixes of "error", "info", "trace", "warning"
Then it reads until the next one or the end.
The logs can be multi lines but belong to the previous prefix until a new one is encountered.
*/

type StripParams struct {
	stripErrors   bool
	stripInfos    bool
	stripTraces   bool
	stripWarnings bool
}

func stripLeanOutput(output string, params StripParams) string {
	newOutput := ""
	currentPrefix := ""
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "error:") {
			currentPrefix = "error:"
		}
		if strings.HasPrefix(line, "info:") {
			currentPrefix = "info:"
		}
		if strings.HasPrefix(line, "trace:") {
			currentPrefix = "trace:"
		}
		if strings.HasPrefix(line, "warning:") {
			currentPrefix = "warning:"
		}
		if currentPrefix == "error:" && !params.stripErrors {
			newOutput += line + "\n"
		}
		if currentPrefix == "info:" && !params.stripInfos {
			newOutput += line + "\n"
		}
		if currentPrefix == "trace:" && !params.stripTraces {
			newOutput += line + "\n"
		}
		if currentPrefix == "warning:" && !params.stripWarnings {
			newOutput += line + "\n"
		}
	}

	return newOutput
}
