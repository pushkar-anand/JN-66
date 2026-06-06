#!/usr/bin/env bash
# Post a test coverage diff as a PR comment.
# Required env: GH_TOKEN, PR_NUMBER
# Reads: coverage.out (PR branch), main-coverage.out (main baseline, optional)
set -euo pipefail

PR_COV="coverage.out"
MAIN_COV="main-coverage.out"
MODULE=$(go list -m)

# parse_profile <profile> → lines of "<pct> <short/pkg>" sorted by package
parse_profile() {
    local profile="$1"
    awk -v mod="$MODULE" '
    /^mode:/ { next }
    {
        split($1, a, ":")
        filepath = a[1]
        sub(mod "/", "", filepath)
        # strip filename (last path component)
        n = split(filepath, parts, "/")
        pkg = ""
        for (i = 1; i < n; i++) pkg = (i > 1 ? pkg "/" : "") parts[i]
        if (pkg == "") pkg = "."
        stmts = $2; covered = $3
        pkg_stmts[pkg] += stmts
        if (covered > 0) pkg_covered[pkg] += stmts
    }
    END {
        for (pkg in pkg_stmts) {
            pct = (pkg_stmts[pkg] > 0) ? 100.0 * pkg_covered[pkg] / pkg_stmts[pkg] : 0.0
            printf "%.1f %s\n", pct, pkg
        }
    }
    ' "$profile" | sort -k2
}

# total_pct <profile> → e.g. "72.3"
total_pct() {
    go tool cover -func="$1" | awk '/^total:/ { gsub(/%/, "", $3); printf "%.1f", $3 }'
}

# delta_str <new> <old> → e.g. "+1.2%" "-0.5%" "±0.0%"
delta_str() {
    awk -v n="$1" -v o="$2" 'BEGIN {
        d = n - o
        if (d > 0.04) printf "+%.1f%%", d
        else if (d < -0.04) printf "-%.1f%%", d
        else print "±0.0%"
    }'
}

pr_total=$(total_pct "$PR_COV")

has_main=false
if [[ -f "$MAIN_COV" ]]; then
    has_main=true
    main_total=$(total_pct "$MAIN_COV")
fi

# Build per-package table rows
declare -A pr_pkg main_pkg

while IFS=' ' read -r pct pkg; do
    pr_pkg["$pkg"]="$pct"
done < <(parse_profile "$PR_COV")

if $has_main; then
    while IFS=' ' read -r pct pkg; do
        main_pkg["$pkg"]="$pct"
    done < <(parse_profile "$MAIN_COV")
fi

# Collect all packages
declare -A all_pkgs
for pkg in "${!pr_pkg[@]}"; do all_pkgs["$pkg"]=1; done
if $has_main; then
    for pkg in "${!main_pkg[@]}"; do all_pkgs["$pkg"]=1; done
fi

changed_rows=""
unchanged_rows=""

for pkg in $(echo "${!all_pkgs[@]}" | tr ' ' '\n' | sort); do
    pr_val="${pr_pkg[$pkg]:-}"
    main_val="${main_pkg[$pkg]:-}"

    if [[ -z "$pr_val" ]]; then pr_display="—"; else pr_display="${pr_val}%"; fi
    if [[ -z "$main_val" ]]; then main_display="—"; else main_display="${main_val}%"; fi

    if $has_main; then
        if [[ -n "$pr_val" && -n "$main_val" ]]; then
            delta=$(delta_str "$pr_val" "$main_val")
        elif [[ -z "$pr_val" ]]; then
            delta="removed"
        else
            delta="new"
        fi
        row="| \`${pkg}\` | ${pr_display} | ${main_display} | ${delta} |"
        if [[ "$delta" == "±0.0%" ]]; then
            unchanged_rows+=$'\n'"$row"
        else
            changed_rows+=$'\n'"$row"
        fi
    else
        row="| \`${pkg}\` | ${pr_display} |"
        changed_rows+=$'\n'"$row"
    fi
done

# Compose comment body
if $has_main; then
    delta_total=$(delta_str "$pr_total" "$main_total")
    header="**Total: ${pr_total}%** (${delta_total} vs main \`${main_total}%\`)"
    table_header=$'| Package | PR | Main | Change |\n|---------|-----|------|--------|'
else
    header="**Total: ${pr_total}%** _(no main baseline yet — will show delta on next push)_"
    table_header=$'| Package | Coverage |\n|---------|----------|'
fi

body="## Test Coverage

${header}

${table_header}${changed_rows}"

if $has_main && [[ -n "$unchanged_rows" ]]; then
    body+="

<details>
<summary>Packages with no change</summary>

${table_header}${unchanged_rows}

</details>"
fi

body+="

<!-- go-coverage-report -->"

# Post or update comment
if ! gh pr comment "$PR_NUMBER" --edit-last --body "$body" 2>/dev/null; then
    gh pr comment "$PR_NUMBER" --body "$body"
fi
