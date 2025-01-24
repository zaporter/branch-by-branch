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
    
    # Use awk to remove the attribution block at the start of the file
    # This looks for /- at the start, followed by any content until -/, 
    # followed by optional whitespace
    awk '
        BEGIN { skip = 0; found = 0 }
        /^\/\-\nCopyright/ { if (!found) { skip = 1; found = 1; next } }
        /\-\// { if (skip) { skip = 0; next } }
        !skip { print }
    ' "$file" > "$temp_file"
    
    # Replace original file with the modified content
    mv "$temp_file" "$file"
done


