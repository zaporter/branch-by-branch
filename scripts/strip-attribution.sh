#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

if [[ "${1-}" =~ ^-*h(elp)?$ ]]; then
    echo 'Usage: ./strip-attribution.sh

    Traverses the current directory and for all .Lean files,
    if it detects attribution at the beginning of the file in the form:
/-
Copyright ...
...
-/

it deletes that attribution.

This is not meant to obscure authorship or plagiarize code.
The original authorship remains intact in the main repo.
This is simply meant to help reduce noise during training. I would not want an LLM to invent authors & copyright claims when writing new files.
'
    exit
fi

# Find all .lean files in the current directory and subdirectories
find . -type f -name "*.lean" | grep -v "\.lake" | while read -r file; do
    # Create a temporary file
    temp_file=$(mktemp)
    
    # Use awk to remove copyright block if it exists at start of file
    # Only checks the first /- ... -/ block and only if it starts at line 1
    awk '
        BEGIN { skip = 0; buffer = ""; line_num = 0 }
        {line_num++}
        /^\/\-/ { 
            if (line_num == 1) {
                skip = 1
                buffer = $0 "\n"
                next
            }
        }
        skip { buffer = buffer $0 "\n" }
        /\-\// { 
            if (skip) { 
                skip = 0
                if (buffer !~ /Copyright/) {
                    printf "%s", buffer
                }
                buffer = ""
                next 
            }
        }
        !skip { print }
    ' "$file" > "$temp_file"
    
    # Replace original file with the modified content
    mv "$temp_file" "$file"
done


