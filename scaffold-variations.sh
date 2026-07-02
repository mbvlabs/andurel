#!/usr/bin/env bash
set -euo pipefail

ANDUREL_BIN="${ANDUREL_BIN:-andurel}"
PREFIX="${PREFIX:-test}"
DRY_RUN="${DRY_RUN:-0}"

extensions=(docker aws-ses css-components)

run_cmd() {
	local -a cmd=("$@")

	if [[ "$DRY_RUN" == "1" ]]; then
		printf '%q ' "${cmd[@]}"
		printf '\n'
		return
	fi

	printf 'Running: '
	printf '%q ' "${cmd[@]}"
	printf '\n'
	"${cmd[@]}"
}

extension_name_suffix() {
	local -a selected=("$@")

	if [[ "${#selected[@]}" -eq 0 ]]; then
		return
	fi

	if [[ "${#selected[@]}" -eq "${#extensions[@]}" ]]; then
		printf -- "-all-extensions"
		return
	fi

	local extension
	for extension in "${selected[@]}"; do
		printf -- "-%s" "$extension"
	done
}

scaffold_variation() {
	local css="$1"
	local inertia="$2"
	shift 2
	local -a selected_extensions=("$@")

	local name="${PREFIX}-${css}"
	local -a cmd=("$ANDUREL_BIN" new)

	if [[ "$inertia" != "" ]]; then
		name="${name}-inertia-${inertia}"
	fi

	name="${name}$(extension_name_suffix "${selected_extensions[@]}")"
	cmd+=("$name")

	if [[ "$inertia" != "" ]]; then
		cmd+=(--inertia "$inertia")
	fi

	local extension
	for extension in "${selected_extensions[@]}"; do
		cmd+=(-e "$extension")
	done

	run_cmd "${cmd[@]}"
}

for css in tailwind; do
	inertia_modes=("")
	if [[ "$css" == "tailwind" ]]; then
		inertia_modes=("" vue)
	fi

	for inertia in "${inertia_modes[@]}"; do
		for mask in {0..7}; do
			selected_extensions=()
			for i in "${!extensions[@]}"; do
				if (( mask & (1 << i) )); then
					selected_extensions+=("${extensions[$i]}")
				fi
			done

			scaffold_variation "$css" "$inertia" "${selected_extensions[@]}"
		done
	done
done
