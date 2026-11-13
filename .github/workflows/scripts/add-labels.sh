#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
#

set -euo pipefail

if [[ -z "${ISSUE:-}" || -z "${COMMENT:-}" || -z "${SENDER:-}" ]]; then
    echo "At least one of ISSUE, COMMENT, or SENDER has not been set, please ensure each is set."
    exit 0
fi

CUR_DIRECTORY=$(dirname "$0")

if [[ ${COMMENT:0:6} != "/label" ]]; then
    echo "Comment is not a label comment, exiting."
    exit 0
fi

declare -A COMMON_LABELS
COMMON_LABELS["arm64"]="arm64"
COMMON_LABELS["good-first-issue"]="good first issue"
COMMON_LABELS["help-wanted"]="help wanted"
COMMON_LABELS["discussion-needed"]="discussion needed"
COMMON_LABELS["needs-triage"]="needs triage"
COMMON_LABELS["os:mac"]="os:mac"
COMMON_LABELS["os:windows"]="os:windows"
COMMON_LABELS["waiting-for-author"]="waiting for author"
COMMON_LABELS["waiting-for-code-owners"]="waiting-for-code-owners"
COMMON_LABELS["waiting-for-approvers"]="waiting-for-approvers"
COMMON_LABELS["bug"]="bug"
COMMON_LABELS["priority:p0"]="priority:p0"
COMMON_LABELS["priority:p1"]="priority:p1"
COMMON_LABELS["priority:p2"]="priority:p2"
COMMON_LABELS["priority:p3"]="priority:p3"
COMMON_LABELS["stale"]="Stale"
COMMON_LABELS["never-stale"]="never stale"
COMMON_LABELS["skip-changelog"]="Skip Changelog"

LABELS=$(echo "${COMMENT}" | sed -E 's%^/label%%')

for LABEL_REQ in ${LABELS}; do
    LABEL=$(echo "${LABEL_REQ}" | sed -E s/^[+-]?//)
    # Trim newlines from label that would cause matching to fail
    LABEL=$(echo "${LABEL}" | tr -d '\n')

    SHOULD_ADD=true
    if [[ "${LABEL_REQ:0:1}" = "-" ]]; then
        SHOULD_ADD=false
    fi

    # Only a code owner of a component this PR changes may add the
    # waiting-for-approvers label by commenting. Triagers, approvers and
    # maintainers have the access to add it from the UI instead.
    if [[ "${LABEL}" = "waiting-for-approvers" && ${SHOULD_ADD} = true ]]; then
        if [[ "${IS_PR:-false}" != "true" ]]; then
            echo "waiting-for-approvers can only be set on a pull request, skipping."
            continue
        fi

        CHANGED_FILES=$(gh api --paginate "repos/${GITHUB_REPOSITORY}/pulls/${ISSUE}/files" --jq '.[].filename' 2>/dev/null) || CHANGED_FILES=""

        IS_CODE_OWNER=false
        while IFS= read -r CHANGED_FILE; do
            [[ -z "${CHANGED_FILE}" ]] && continue
            # Individual (non-team) owners of the last CODEOWNERS entry matching this file.
            OWNERS=$(awk -v f="${CHANGED_FILE}" '
                /^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
                {
                    pat=$1; sub(/^\//,"",pat); matched=0
                    if (pat=="*") matched=1
                    else if (pat ~ /\/$/) { if (substr(f,1,length(pat))==pat) matched=1 }
                    else { if (f==pat || substr(f,1,length(pat)+1)==pat"/") matched=1 }
                    if (matched) { owners=""; for (i=2;i<=NF;i++) if ($i !~ /\//) owners=owners" "$i }
                }
                END { print owners }
            ' .github/CODEOWNERS)
            for OWNER in ${OWNERS}; do
                if [[ "${OWNER,,}" = "@${SENDER,,}" ]]; then
                    IS_CODE_OWNER=true
                    break
                fi
            done
            [[ "${IS_CODE_OWNER}" = true ]] && break
        done <<< "${CHANGED_FILES}"

        if [[ "${IS_CODE_OWNER}" != true ]]; then
            echo "${SENDER} is not a code owner of a component changed by this PR, skipping waiting-for-approvers."
            continue
        fi
    fi

    if [[ -v COMMON_LABELS["${LABEL}"] ]]; then
        if [[ ${SHOULD_ADD} = true ]]; then
            gh issue edit "${ISSUE}" --add-label "${COMMON_LABELS["${LABEL}"]}"
        else
            gh issue edit "${ISSUE}" --remove-label "${COMMON_LABELS["${LABEL}"]}"
        fi
        continue
    fi

    SHORT_LABEL=$(awk -v path="${LABEL}" '$1 == path || $2 == path {print $2}' .github/component_labels.txt)

    if [[  -z "${SHORT_LABEL}" ]]; then
        echo "\"${SHORT_LABEL}\" doesn't correspond to a component, skipping."
        continue
    fi

    if [[ ${SHOULD_ADD} = true ]]; then
        gh issue edit "${ISSUE}" --add-label "${SHORT_LABEL}"

        # Labels added by a GitHub Actions workflow don't trigger other workflows
        # by design, so we have to manually ping code owners here.
        COMPONENT="${LABEL}" ISSUE=${ISSUE} SENDER="${SENDER}" bash "${CUR_DIRECTORY}/ping-codeowners-issues.sh"
    else
        gh issue edit "${ISSUE}" --remove-label "${LABEL}"
    fi
done


