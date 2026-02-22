#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

API_KEY="${API_KEY:-sk-bench}"
IMAGE_NAME="${IMAGE_NAME:-python}"
OCI_REF="${OCI_REF:-python:3.12-slim}"
PORT="${PORT:-18080}"
HOST="http://127.0.0.1:${PORT}"

COLD_RUNS="${COLD_RUNS:-3}"
WARM_RUNS="${WARM_RUNS:-3}"
DOCKER_RUNS="${DOCKER_RUNS:-3}"
POOL_SIZE="${POOL_SIZE:-3}"
PARALLEL_USERS="${PARALLEL_USERS:-0}"

KEEP_BENCH_ENV="${KEEP_BENCH_ENV:-0}"
REPORT_ROOT="${REPORT_ROOT:-${REPO_ROOT}/bench-reports}"

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
REPORT_DIR="${REPORT_ROOT}/${STAMP}"

TMP_ROOT="$(mktemp -d /tmp/sandkasten-bench.XXXXXX)"
DATA_DIR="${TMP_ROOT}/data"
CONFIG_COLD="${TMP_ROOT}/sandkasten-cold.yaml"
CONFIG_WARM="${TMP_ROOT}/sandkasten-warm.yaml"

log() {
  printf '[bench] %s\n' "$*"
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'Missing required command: %s\n' "$1" >&2
    exit 1
  }
}

parse_parallel_levels() {
  local raw="$1"
  local token
  local cleaned

  IFS=',' read -ra tokens <<<"${raw}"
  for token in "${tokens[@]}"; do
    cleaned="${token//[[:space:]]/}"
    if [[ -z "${cleaned}" ]]; then
      continue
    fi
    if [[ "${cleaned}" =~ ^[0-9]+$ ]] && [[ "${cleaned}" -gt 0 ]]; then
      printf '%s\n' "${cleaned}"
    fi
  done
}

build_binaries() {
  if command -v task >/dev/null 2>&1; then
    log "Building binaries with task"
    task -d "${REPO_ROOT}" runner daemon sandbench >/dev/null
    return
  fi

  require_cmd go
  log "'task' not found; building binaries with go build"
  (
    cd "${REPO_ROOT}"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o bin/runner ./cmd/runner
    go build -o bin/sandkasten ./cmd/sandkasten
    go build -o bin/sandbench ./cmd/sandbench
  )
}

daemon_running=0

cleanup() {
  if [[ "${daemon_running}" == "1" ]]; then
    log "Stopping daemon"
    sudo "${REPO_ROOT}/bin/sandkasten" stop --data-dir "${DATA_DIR}" >/dev/null 2>&1 || true
  fi

  if [[ "${KEEP_BENCH_ENV}" == "1" ]]; then
    log "Keeping temporary benchmark environment: ${TMP_ROOT}"
  else
    if ! rm -rf "${TMP_ROOT}" 2>/dev/null; then
      sudo rm -rf "${TMP_ROOT}" >/dev/null 2>&1 || true
    fi
  fi
}

trap cleanup EXIT

write_config() {
  local cfg_path="$1"
  local pool_enabled="$2"
  local pool_size="$3"

  cat >"${cfg_path}" <<EOF
listen: "127.0.0.1:${PORT}"
api_key: "${API_KEY}"
data_dir: "${DATA_DIR}"
db_path: "${DATA_DIR}/sandkasten.db"
default_image: "${IMAGE_NAME}"
allowed_images:
  - "${IMAGE_NAME}"
session_ttl_seconds: 3600
defaults:
  cpu_limit: 2
  mem_limit_mb: 1024
  pids_limit: 256
  max_exec_timeout_ms: 600000
  network_mode: "bridge"
  readonly_rootfs: true
security:
  seccomp: "strict"
workspace:
  enabled: true
  persist_by_default: false
pool:
  enabled: ${pool_enabled}
  images:
    ${IMAGE_NAME}: ${pool_size}
EOF
}

start_daemon() {
  local cfg_path="$1"

  log "Starting daemon with ${cfg_path}"
  sudo "${REPO_ROOT}/bin/sandkasten" daemon -d --config "${cfg_path}"
  daemon_running=1

  for _ in {1..60}; do
    if curl -sf "${HOST}/healthz" >/dev/null; then
      return 0
    fi
    sleep 0.5
  done

  printf 'Daemon did not become healthy at %s\n' "${HOST}" >&2
  exit 1
}

stop_daemon() {
  if [[ "${daemon_running}" == "1" ]]; then
    log "Stopping daemon"
    sudo "${REPO_ROOT}/bin/sandkasten" stop --data-dir "${DATA_DIR}" >/dev/null 2>&1 || true
    daemon_running=0
  fi
}

run_report() {
  local name="$1"
  shift
  log "Running report: ${name}"
  "${REPO_ROOT}/bin/sandbench" "$@" --json >"${REPORT_DIR}/${name}.json"
}

percentile_from_file() {
  local file="$1"
  local p="$2"
  awk -v p="${p}" 'BEGIN{n=0} {a[++n]=$1} END {if (n==0) {print "0.000"; exit} idx=int((p/100.0)*n); if (idx<1) idx=1; if (idx>n) idx=n; print a[idx]}' "${file}"
}

run_parallel_case() {
  local scenario="$1"
  local users="$2"
  local worker_func="$3"
  local out_csv="$4"

  local tmp_case="${REPORT_DIR}/.${scenario}.tmp"
  rm -rf "${tmp_case}"
  mkdir -p "${tmp_case}"

  log "Parallel case: ${scenario} (${users} users)"

  for i in $(seq 1 "${users}"); do
    "${worker_func}" "${scenario}" "${i}" "${tmp_case}" &
  done
  wait

  local ok_count fail_count
  ok_count="$(awk -F',' '$4=="1" {c++} END {print c+0}' "${tmp_case}"/*.csv)"
  fail_count="$(awk -F',' '$4=="0" {c++} END {print c+0}' "${tmp_case}"/*.csv)"

  local dur_file="${tmp_case}/durations.txt"
  awk -F',' '$4=="1" {print $3}' "${tmp_case}"/*.csv | sort -n >"${dur_file}"

  local p50 p95 p99 avg min max
  if [[ -s "${dur_file}" ]]; then
    p50="$(percentile_from_file "${dur_file}" 50)"
    p95="$(percentile_from_file "${dur_file}" 95)"
    p99="$(percentile_from_file "${dur_file}" 99)"
    avg="$(awk '{s+=$1; n++} END {if (n==0) print "0.000"; else printf "%.3f", s/n}' "${dur_file}")"
    min="$(awk 'NR==1 {print $1}' "${dur_file}")"
    max="$(awk 'END {print $1}' "${dur_file}")"
  else
    p50="0.000"; p95="0.000"; p99="0.000"; avg="0.000"; min="0.000"; max="0.000"
  fi

  printf '"%s",%s,%s,%s,%s,%s,%s,%s,%s,%s\n' \
    "${scenario}" "${users}" "${ok_count}" "${fail_count}" "${avg}" "${p50}" "${p95}" "${p99}" "${min}" "${max}" >>"${out_csv}"

  rm -rf "${tmp_case}"
}

worker_workspace_create() {
  local scenario="$1"
  local idx="$2"
  local out_dir="$3"
  local ws_id="bench-user-${idx}-${STAMP}"
  local t0 t1 dur_ms http
  t0="$(date +%s%N)"
  http="$(curl -sS -o /dev/null -w '%{http_code}' \
    -X POST "${HOST}/v1/workspaces/${ws_id}/fs/write" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"path":".bench/init.txt","text":"ok"}')"
  t1="$(date +%s%N)"
  dur_ms="$(awk -v a="${t0}" -v b="${t1}" 'BEGIN {printf "%.3f", (b-a)/1000000.0}')"
  if [[ "${http}" == "200" || "${http}" == "201" ]]; then
    printf '"%s",%s,%s,1,\n' "${scenario}" "${idx}" "${dur_ms}" >"${out_dir}/${idx}.csv"
  else
    printf '"%s",%s,%s,0,"http_%s"\n' "${scenario}" "${idx}" "${dur_ms}" "${http}" >"${out_dir}/${idx}.csv"
  fi
}

worker_session_unique_workspace() {
  local scenario="$1"
  local idx="$2"
  local out_dir="$3"
  local ws_id="bench-user-${idx}-${STAMP}"
  local t0 t1 dur_ms http body id
  t0="$(date +%s%N)"
  body="$(mktemp)"
  http="$(curl -sS -o "${body}" -w '%{http_code}' \
    -X POST "${HOST}/v1/sessions" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "{\"image\":\"${IMAGE_NAME}\",\"workspace_id\":\"${ws_id}\"}")"
  id=""
  if [[ "${http}" == "200" || "${http}" == "201" ]]; then
    id="$(jq -r '.id // empty' "${body}" 2>/dev/null || true)"
  fi
  if [[ -n "${id}" ]]; then
    curl -sS -o /dev/null -X DELETE "${HOST}/v1/sessions/${id}" -H "Authorization: Bearer ${API_KEY}" || true
  fi
  rm -f "${body}"
  t1="$(date +%s%N)"
  dur_ms="$(awk -v a="${t0}" -v b="${t1}" 'BEGIN {printf "%.3f", (b-a)/1000000.0}')"
  if [[ -n "${id}" ]]; then
    printf '"%s",%s,%s,1,\n' "${scenario}" "${idx}" "${dur_ms}" >"${out_dir}/${idx}.csv"
  else
    printf '"%s",%s,%s,0,"http_%s"\n' "${scenario}" "${idx}" "${dur_ms}" "${http}" >"${out_dir}/${idx}.csv"
  fi
}

worker_session_shared_workspace() {
  local scenario="$1"
  local idx="$2"
  local out_dir="$3"
  local ws_id="bench-shared-${STAMP}"
  local t0 t1 dur_ms http body id
  t0="$(date +%s%N)"
  body="$(mktemp)"
  http="$(curl -sS -o "${body}" -w '%{http_code}' \
    -X POST "${HOST}/v1/sessions" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "{\"image\":\"${IMAGE_NAME}\",\"workspace_id\":\"${ws_id}\"}")"
  id=""
  if [[ "${http}" == "200" || "${http}" == "201" ]]; then
    id="$(jq -r '.id // empty' "${body}" 2>/dev/null || true)"
  fi
  if [[ -n "${id}" ]]; then
    curl -sS -o /dev/null -X DELETE "${HOST}/v1/sessions/${id}" -H "Authorization: Bearer ${API_KEY}" || true
  fi
  rm -f "${body}"
  t1="$(date +%s%N)"
  dur_ms="$(awk -v a="${t0}" -v b="${t1}" 'BEGIN {printf "%.3f", (b-a)/1000000.0}')"
  if [[ -n "${id}" ]]; then
    printf '"%s",%s,%s,1,\n' "${scenario}" "${idx}" "${dur_ms}" >"${out_dir}/${idx}.csv"
  else
    printf '"%s",%s,%s,0,"http_%s"\n' "${scenario}" "${idx}" "${dur_ms}" "${http}" >"${out_dir}/${idx}.csv"
  fi
}

require_cmd sudo
require_cmd curl
require_cmd jq

mapfile -t PARALLEL_LEVELS < <(parse_parallel_levels "${PARALLEL_USERS}")
PARALLEL_ENABLED=0
if [[ "${#PARALLEL_LEVELS[@]}" -gt 0 ]]; then
  PARALLEL_ENABLED=1
fi

mkdir -p "${REPORT_DIR}"

build_binaries

log "Creating fresh environment via init"
sudo "${REPO_ROOT}/bin/sandkasten" init \
  --config "${CONFIG_COLD}" \
  --data-dir "${DATA_DIR}" \
  --default-image "${IMAGE_NAME}" \
  --api-key "${API_KEY}" \
  --skip-pull \
  --force >/dev/null

log "Pulling benchmark image ${OCI_REF} as ${IMAGE_NAME}"
sudo "${REPO_ROOT}/bin/sandkasten" image pull \
  --name "${IMAGE_NAME}" \
  --data-dir "${DATA_DIR}" \
  "${OCI_REF}" >/dev/null

write_config "${CONFIG_COLD}" false "${POOL_SIZE}"
write_config "${CONFIG_WARM}" true "${POOL_SIZE}"

log "Cold benchmark phase (pool disabled)"
start_daemon "${CONFIG_COLD}"

run_report "sandkasten_cold_none" \
  --target sandkasten \
  --host "${HOST}" \
  --api-key "${API_KEY}" \
  --image "${IMAGE_NAME}" \
  --cold-runs "${COLD_RUNS}" \
  --warm-runs 0 \
  --workspace-mode none

run_report "sandkasten_cold_per_run_workspace" \
  --target sandkasten \
  --host "${HOST}" \
  --api-key "${API_KEY}" \
  --image "${IMAGE_NAME}" \
  --cold-runs "${COLD_RUNS}" \
  --warm-runs 0 \
  --workspace-mode per-run

run_report "docker_baseline_cold_phase" \
  --target docker \
  --docker-image "${OCI_REF}" \
  --docker-runs "${DOCKER_RUNS}"

stop_daemon

log "Warm benchmark phase (pool enabled)"
start_daemon "${CONFIG_WARM}"
sleep 3

run_report "sandkasten_warm_none" \
  --target sandkasten \
  --host "${HOST}" \
  --api-key "${API_KEY}" \
  --image "${IMAGE_NAME}" \
  --cold-runs 0 \
  --warm-runs "${WARM_RUNS}" \
  --workspace-mode none

run_report "sandkasten_warm_shared_workspace" \
  --target sandkasten \
  --host "${HOST}" \
  --api-key "${API_KEY}" \
  --image "${IMAGE_NAME}" \
  --cold-runs 0 \
  --warm-runs "${WARM_RUNS}" \
  --workspace-mode shared \
  --workspace-prefix sandbench-shared

run_report "sandkasten_vs_docker_warm" \
  --target both \
  --host "${HOST}" \
  --api-key "${API_KEY}" \
  --image "${IMAGE_NAME}" \
  --docker-image "${OCI_REF}" \
  --cold-runs 0 \
  --warm-runs "${WARM_RUNS}" \
  --docker-runs "${DOCKER_RUNS}"

PARALLEL_CSV="${REPORT_DIR}/parallel_summary.csv"
PARALLEL_MD="${REPORT_DIR}/parallel_summary.md"
PARALLEL_SCALING_CSV="${REPORT_DIR}/parallel_scaling.csv"
PARALLEL_SCALING_MD="${REPORT_DIR}/parallel_scaling.md"

if [[ "${PARALLEL_ENABLED}" == "1" ]]; then
  log "Running parallel multi-user benchmark (levels: ${PARALLEL_LEVELS[*]})"
  # Ensure shared workspace exists before shared-session concurrency test.
  curl -sS -o /dev/null -X POST "${HOST}/v1/workspaces/bench-shared-${STAMP}/fs/write" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"path":".bench/init.txt","text":"ok"}'

  cat >"${PARALLEL_CSV}" <<EOF
scenario,users,ok,failed,avg_ms,p50_ms,p95_ms,p99_ms,min_ms,max_ms
EOF

  for users in "${PARALLEL_LEVELS[@]}"; do
    run_parallel_case "Workspace create (unique users)" "${users}" worker_workspace_create "${PARALLEL_CSV}"
    run_parallel_case "Session create+destroy (unique workspace per user)" "${users}" worker_session_unique_workspace "${PARALLEL_CSV}"
    run_parallel_case "Session create+destroy (shared workspace)" "${users}" worker_session_shared_workspace "${PARALLEL_CSV}"
  done

  cat >"${PARALLEL_MD}" <<EOF
| Scenario | Users | OK | Failed | Avg (ms) | P50 (ms) | P95 (ms) | P99 (ms) | Min (ms) | Max (ms) |
|---|---|---|---|---|---|---|---|---|---|
EOF
  tail -n +2 "${PARALLEL_CSV}" | while IFS=',' read -r scenario users ok failed avg p50 p95 p99 min max; do
    scenario="${scenario%\"}"; scenario="${scenario#\"}"
    printf '| %s | %s | %s | %s | %.3f | %.3f | %.3f | %.3f | %.3f | %.3f |\n' \
      "$scenario" "$users" "$ok" "$failed" "$avg" "$p50" "$p95" "$p99" "$min" "$max" >>"${PARALLEL_MD}"
  done

  cat >"${PARALLEL_SCALING_CSV}" <<EOF
users,scenario,avg_ms,p95_ms,p99_ms,failed,ok,fail_rate_pct
EOF
  awk -F',' 'NR>1 {
      scenario=$1; gsub(/^"|"$/, "", scenario);
      users=$2; ok=$3; failed=$4; avg=$5; p95=$7; p99=$8;
      total=ok+failed; fail_rate=(total>0)?(100.0*failed/total):0.0;
      printf "%s,%s,%s,%s,%s,%s,%s,%.2f\n", users, scenario, avg, p95, p99, failed, ok, fail_rate;
    }' "${PARALLEL_CSV}" >>"${PARALLEL_SCALING_CSV}"

  cat >"${PARALLEL_SCALING_MD}" <<EOF
| Users | Scenario | Avg (ms) | P95 (ms) | P99 (ms) | Failed | OK | Fail Rate (%) |
|---|---|---|---|---|---|---|---|
EOF
  tail -n +2 "${PARALLEL_SCALING_CSV}" | sort -t',' -k1,1n -k2,2 | while IFS=',' read -r users scenario avg p95 p99 failed ok fail_rate; do
    printf '| %s | %s | %.3f | %.3f | %.3f | %s | %s | %.2f |\n' \
      "$users" "$scenario" "$avg" "$p95" "$p99" "$failed" "$ok" "$fail_rate" >>"${PARALLEL_SCALING_MD}"
  done
fi

stop_daemon

log "Generating summary tables"

SUMMARY_CSV="${REPORT_DIR}/summary.csv"
SUMMARY_MD="${REPORT_DIR}/summary.md"
COMPARISON_MD="${REPORT_DIR}/comparison.md"

cat >"${SUMMARY_CSV}" <<EOF
scenario,target,phase,workspace,count,startup_avg_ms,startup_min_ms,startup_max_ms,pooled_hits
EOF

append_sand_row() {
  local file="$1"
  local scenario="$2"
  local phase="$3"
  local workspace="$4"
  jq -r --arg scenario "${scenario}" --arg phase "${phase}" --arg workspace "${workspace}" '
    .sandkasten as $s
    | (if $phase == "cold" then $s.cold_summary else $s.warm_summary end) as $m
    | select(($m.count // 0) > 0)
    | [
        $scenario,
        "sandkasten",
        $phase,
        $workspace,
        ($m.count|tostring),
        ($m.startup_avg_ms|tostring),
        ($m.startup_min_ms|tostring),
        ($m.startup_max_ms|tostring),
        ($m.pooled_hits|tostring)
      ]
    | @csv
  ' "${file}" >>"${SUMMARY_CSV}"
}

append_docker_row() {
  local file="$1"
  local scenario="$2"
  jq -r --arg scenario "${scenario}" '
    .docker.summary as $m
    | select(($m.count // 0) > 0)
    | [
        $scenario,
        "docker",
        "run",
        "n/a",
        ($m.count|tostring),
        ($m.startup_avg_ms|tostring),
        ($m.startup_min_ms|tostring),
        ($m.startup_max_ms|tostring),
        "0"
      ]
    | @csv
  ' "${file}" >>"${SUMMARY_CSV}"
}

append_sand_row "${REPORT_DIR}/sandkasten_cold_none.json" "Sandkasten cold (no workspace)" "cold" "none"
append_sand_row "${REPORT_DIR}/sandkasten_cold_per_run_workspace.json" "Sandkasten cold (per-run workspace)" "cold" "per-run"
append_sand_row "${REPORT_DIR}/sandkasten_warm_none.json" "Sandkasten warm pooled (no workspace)" "warm" "none"
append_sand_row "${REPORT_DIR}/sandkasten_warm_shared_workspace.json" "Sandkasten warm pooled (shared workspace)" "warm" "shared"
append_sand_row "${REPORT_DIR}/sandkasten_vs_docker_warm.json" "Sandkasten warm pooled (head-to-head)" "warm" "none"
append_docker_row "${REPORT_DIR}/docker_baseline_cold_phase.json" "Docker cold baseline"
append_docker_row "${REPORT_DIR}/sandkasten_vs_docker_warm.json" "Docker head-to-head"

cat >"${SUMMARY_MD}" <<EOF
| Scenario | Target | Phase | Workspace | Count | Startup Avg (ms) | Startup Min (ms) | Startup Max (ms) | Pooled Hits |
|---|---|---|---|---|---|---|---|---|
EOF

append_sand_md_row() {
  local file="$1"
  local scenario="$2"
  local phase="$3"
  local workspace="$4"
  jq -r --arg scenario "${scenario}" --arg phase "${phase}" --arg workspace "${workspace}" '
    .sandkasten as $s
    | (if $phase == "cold" then $s.cold_summary else $s.warm_summary end) as $m
    | select(($m.count // 0) > 0)
    | "| \($scenario) | sandkasten | \($phase) | \($workspace) | \($m.count) | \(($m.startup_avg_ms // 0)|tonumber|tostring) | \(($m.startup_min_ms // 0)|tonumber|tostring) | \(($m.startup_max_ms // 0)|tonumber|tostring) | \(($m.pooled_hits // 0)) |"
  ' "${file}" >>"${SUMMARY_MD}"
}

append_docker_md_row() {
  local file="$1"
  local scenario="$2"
  jq -r --arg scenario "${scenario}" '
    .docker.summary as $m
    | select(($m.count // 0) > 0)
    | "| \($scenario) | docker | run | n/a | \($m.count) | \(($m.startup_avg_ms // 0)|tonumber|tostring) | \(($m.startup_min_ms // 0)|tonumber|tostring) | \(($m.startup_max_ms // 0)|tonumber|tostring) | 0 |"
  ' "${file}" >>"${SUMMARY_MD}"
}

append_sand_md_row "${REPORT_DIR}/sandkasten_cold_none.json" "Sandkasten cold (no workspace)" "cold" "none"
append_sand_md_row "${REPORT_DIR}/sandkasten_cold_per_run_workspace.json" "Sandkasten cold (per-run workspace)" "cold" "per-run"
append_sand_md_row "${REPORT_DIR}/sandkasten_warm_none.json" "Sandkasten warm pooled (no workspace)" "warm" "none"
append_sand_md_row "${REPORT_DIR}/sandkasten_warm_shared_workspace.json" "Sandkasten warm pooled (shared workspace)" "warm" "shared"
append_sand_md_row "${REPORT_DIR}/sandkasten_vs_docker_warm.json" "Sandkasten warm pooled (head-to-head)" "warm" "none"
append_docker_md_row "${REPORT_DIR}/docker_baseline_cold_phase.json" "Docker cold baseline"
append_docker_md_row "${REPORT_DIR}/sandkasten_vs_docker_warm.json" "Docker head-to-head"

cold_none_avg="$(jq -r '.sandkasten.cold_summary.startup_avg_ms // empty' "${REPORT_DIR}/sandkasten_cold_none.json")"
cold_none_min="$(jq -r '.sandkasten.cold_summary.startup_min_ms // empty' "${REPORT_DIR}/sandkasten_cold_none.json")"
cold_none_max="$(jq -r '.sandkasten.cold_summary.startup_max_ms // empty' "${REPORT_DIR}/sandkasten_cold_none.json")"
warm_none_min="$(jq -r '.sandkasten.warm_summary.startup_min_ms // empty' "${REPORT_DIR}/sandkasten_warm_none.json")"
warm_none_max="$(jq -r '.sandkasten.warm_summary.startup_max_ms // empty' "${REPORT_DIR}/sandkasten_warm_none.json")"
warm_none_pooled="$(jq -r '.sandkasten.warm_summary.pooled_hits // 0' "${REPORT_DIR}/sandkasten_warm_none.json")"
warm_none_avg="$(jq -r '.sandkasten.warm_summary.startup_avg_ms // empty' "${REPORT_DIR}/sandkasten_warm_none.json")"
warm_shared_avg="$(jq -r '.sandkasten.warm_summary.startup_avg_ms // empty' "${REPORT_DIR}/sandkasten_warm_shared_workspace.json")"
warm_shared_min="$(jq -r '.sandkasten.warm_summary.startup_min_ms // empty' "${REPORT_DIR}/sandkasten_warm_shared_workspace.json")"
warm_shared_max="$(jq -r '.sandkasten.warm_summary.startup_max_ms // empty' "${REPORT_DIR}/sandkasten_warm_shared_workspace.json")"
warm_shared_pooled="$(jq -r '.sandkasten.warm_summary.pooled_hits // 0' "${REPORT_DIR}/sandkasten_warm_shared_workspace.json")"
docker_h2h_avg="$(jq -r '.docker.summary.startup_avg_ms // empty' "${REPORT_DIR}/sandkasten_vs_docker_warm.json")"
docker_h2h_min="$(jq -r '.docker.summary.startup_min_ms // empty' "${REPORT_DIR}/sandkasten_vs_docker_warm.json")"
docker_h2h_max="$(jq -r '.docker.summary.startup_max_ms // empty' "${REPORT_DIR}/sandkasten_vs_docker_warm.json")"

pool_speedup=""
if [[ -n "${cold_none_avg}" && -n "${warm_none_avg}" ]]; then
  pool_speedup="$(awk -v c="${cold_none_avg}" -v w="${warm_none_avg}" 'BEGIN { if (w>0) printf "%.1f", c/w; else printf "n/a" }')"
fi

shared_delta=""
if [[ -n "${warm_none_avg}" && -n "${warm_shared_avg}" ]]; then
  shared_delta="$(awk -v a="${warm_none_avg}" -v b="${warm_shared_avg}" 'BEGIN { if (a>0) printf "%+.1f", ((b-a)/a)*100; else printf "n/a" }')"
fi

sand_vs_docker=""
if [[ -n "${warm_none_avg}" && -n "${docker_h2h_avg}" ]]; then
  sand_vs_docker="$(awk -v s="${warm_none_avg}" -v d="${docker_h2h_avg}" 'BEGIN { if (s>0) printf "%.1f", d/s; else printf "n/a" }')"
fi

cat >"${COMPARISON_MD}" <<EOF
## Startup Benchmark Comparison

- Pooling speedup (Sandkasten cold -> warm none): **${pool_speedup}x** (${cold_none_avg} ms -> ${warm_none_avg} ms)
- Shared workspace warm delta vs warm none: **${shared_delta}%** (${warm_none_avg} ms -> ${warm_shared_avg} ms)
- Warm startup Sandkasten vs Docker: **${sand_vs_docker}x faster** (${warm_none_avg} ms vs ${docker_h2h_avg} ms)
EOF

TERMINAL_TABLE="${REPORT_DIR}/terminal_table.txt"
cat >"${TERMINAL_TABLE}" <<EOF
Scenario                                      Avg ms      Min ms      Max ms   Pooled
--------------------------------------------------------------------------------------
Sandkasten cold (no workspace)               $(printf "%8.3f" "${cold_none_avg:-0}")  $(printf "%10.3f" "${cold_none_min:-0}")  $(printf "%10.3f" "${cold_none_max:-0}")   0
Sandkasten warm pooled (no workspace)        $(printf "%8.3f" "${warm_none_avg:-0}")  $(printf "%10.3f" "${warm_none_min:-0}")  $(printf "%10.3f" "${warm_none_max:-0}")   ${warm_none_pooled}
Sandkasten warm pooled (shared workspace)    $(printf "%8.3f" "${warm_shared_avg:-0}")  $(printf "%10.3f" "${warm_shared_min:-0}")  $(printf "%10.3f" "${warm_shared_max:-0}")   ${warm_shared_pooled}
Docker head-to-head                          $(printf "%8.3f" "${docker_h2h_avg:-0}")  $(printf "%10.3f" "${docker_h2h_min:-0}")  $(printf "%10.3f" "${docker_h2h_max:-0}")   0
EOF

cat >"${REPORT_DIR}/README.txt" <<EOF
Fresh benchmark completed: ${STAMP}

Host: ${HOST}
Image name: ${IMAGE_NAME}
OCI ref: ${OCI_REF}
Cold runs: ${COLD_RUNS}
Warm runs: ${WARM_RUNS}
Docker runs: ${DOCKER_RUNS}
Pool size: ${POOL_SIZE}

Reports:
  sandkasten_cold_none.json
  sandkasten_cold_per_run_workspace.json
  docker_baseline_cold_phase.json
  sandkasten_warm_none.json
  sandkasten_warm_shared_workspace.json
  sandkasten_vs_docker_warm.json
  summary.csv
  summary.md
  comparison.md
$(if [[ "${PARALLEL_ENABLED}" == "1" ]]; then cat <<EOT
  parallel_summary.csv
  parallel_summary.md
  parallel_scaling.csv
  parallel_scaling.md
EOT
fi)

Temporary env:
  ${TMP_ROOT}

Set KEEP_BENCH_ENV=1 to keep temporary files for inspection.
EOF

log "Reports saved in ${REPORT_DIR}"
cat "${TERMINAL_TABLE}"
log "Quick comparison:"
if [[ -n "${pool_speedup}" ]]; then
  log "  Pooling speedup: ${pool_speedup}x"
fi
if [[ -n "${shared_delta}" ]]; then
  log "  Shared workspace warm delta: ${shared_delta}%"
fi
if [[ -n "${sand_vs_docker}" ]]; then
  log "  Warm Sandkasten vs Docker: ${sand_vs_docker}x faster"
fi
log "  Table: ${SUMMARY_MD}"
log "  Comparison: ${COMPARISON_MD}"
if [[ "${PARALLEL_ENABLED}" == "1" ]]; then
  log "  Parallel: ${PARALLEL_MD}"
  log "  Scaling: ${PARALLEL_SCALING_MD}"
fi
log "Done"
