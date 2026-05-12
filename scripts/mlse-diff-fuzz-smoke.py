#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import random
import re
import shutil
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_CASES_ROOT = REPO_ROOT / "test" / "SymbolicDiff" / "cases"
DEFAULT_ARTIFACT_DIR = REPO_ROOT / "artifacts" / "symbolic-diff-fuzz-smoke"
DEFAULT_WORK_DIR = REPO_ROOT / "tmp" / "symbolic-diff-fuzz-smoke"

INT_VALUES = [0, 1, -1, 2, -2, 7, 10, 999, 1000, 1001, 2147483647, -2147483648]
UINT_VALUES = [0, 1, 2, 7, 1000, 2147483647, 4294967295]
STRING_VALUES = ["", "a", "plugin", "psm", "/plugin_conf/x", "Zm9v", "!", "%%%", "1000"]


@dataclass(frozen=True)
class Param:
    name: str
    type: str


@dataclass(frozen=True)
class FuncSig:
    params: list[Param]
    returns: list[str]
    named_types: dict[str, str]


@dataclass(frozen=True)
class Seed:
    label: str
    flavor: int
    values: list[Any]


@dataclass(frozen=True)
class FuzzRunConfig:
    emit_root: Path
    work_root: Path
    go_bin: str | None
    iterations: int
    seed: int


@dataclass(frozen=True)
class FailedSeedContext:
    index: int
    seed: Seed
    executed: int
    selected: list[int]
    covered: set[str]
    total: int


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run concrete same-input fuzz/diff smoke for SymbolicDiff cases."
    )
    parser.add_argument("--cases-root", default=str(DEFAULT_CASES_ROOT))
    parser.add_argument("--case", action="append", default=[])
    parser.add_argument("--emit", default=str(DEFAULT_ARTIFACT_DIR))
    parser.add_argument("--work-dir", default=str(DEFAULT_WORK_DIR))
    parser.add_argument("--iterations", type=int, default=32)
    parser.add_argument("--seed", type=int, default=1)
    parser.add_argument("--go-bin", default="")
    parser.add_argument("--expect-status", default="")
    return parser.parse_args()


def resolve_path(text: str) -> Path:
    path = Path(text)
    if path.is_absolute():
        return path
    return (REPO_ROOT / path).resolve()


def load_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def discover_go(configured: str) -> str | None:
    if configured:
        path = Path(configured)
        if path.is_file() and os.access(path, os.X_OK):
            return str(path)
        return shutil.which(configured)
    return shutil.which("go")


def collect_case_dirs(cases_root: Path, selected: list[str]) -> list[Path]:
    if selected:
        return [cases_root / name for name in selected]
    return sorted(path for path in cases_root.iterdir() if path.is_dir())


def replace_package(source: str, package_name: str) -> str:
    return re.sub(r"(?m)^package\s+\w+", f"package {package_name}", source, count=1)


def find_matching(text: str, start: int, open_ch: str, close_ch: str) -> int:
    depth = 0
    for index in range(start, len(text)):
        char = text[index]
        if char == open_ch:
            depth += 1
        elif char == close_ch:
            depth -= 1
            if depth == 0:
                return index
    raise ValueError(f"unmatched {open_ch}")


def split_top_level(text: str) -> list[str]:
    out: list[str] = []
    start = 0
    depth = 0
    for index, char in enumerate(text):
        if char in "([{":
            depth += 1
        elif char in ")]}":
            depth -= 1
        elif char == "," and depth == 0:
            out.append(text[start:index].strip())
            start = index + 1
    tail = text[start:].strip()
    if tail:
        out.append(tail)
    return out


def parse_params(text: str) -> list[Param]:
    params: list[Param] = []
    pending_names: list[str] = []
    for chunk in split_top_level(text):
        fields = chunk.split()
        if len(fields) == 1:
            pending_names.append(fields[0])
            continue
        typ = fields[-1]
        names = [item.strip() for item in " ".join(fields[:-1]).split(",") if item.strip()]
        for name in [*pending_names, *names]:
            params.append(Param(name=name, type=typ))
        pending_names = []
    if pending_names:
        raise ValueError(f"parameter names without type: {', '.join(pending_names)}")
    return params


def parse_returns(text: str) -> list[str]:
    text = text.strip()
    if not text:
        return []
    returns = split_top_level(text)
    out: list[str] = []
    for item in returns:
        fields = item.split()
        out.append(fields[-1])
    return out


def parse_func_signature(source: str) -> FuncSig:
    named_types = parse_named_types(source)
    match = re.search(r"\bfunc\s+F\s*\(", source)
    if not match:
        raise ValueError("func F signature not found")
    param_open = source.index("(", match.start())
    param_close = find_matching(source, param_open, "(", ")")
    params = parse_params(source[param_open + 1 : param_close])
    index = param_close + 1
    while index < len(source) and source[index].isspace():
        index += 1
    if index >= len(source) or source[index] == "{":
        return FuncSig(params=params, returns=[], named_types=named_types)
    if source[index] == "(":
        returns_close = find_matching(source, index, "(", ")")
        return FuncSig(
            params=params,
            returns=parse_returns(source[index + 1 : returns_close]),
            named_types=named_types,
        )
    body_start = source.index("{", index)
    return FuncSig(params=params, returns=parse_returns(source[index:body_start]), named_types=named_types)


def parse_named_types(source: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for match in re.finditer(r"(?m)^type\s+([A-Za-z_][A-Za-z0-9_]*)\s+(.+)$", source):
        name = match.group(1)
        rhs = match.group(2).strip()
        if rhs.startswith("struct"):
            out[name] = "struct"
        elif rhs.startswith("[]"):
            out[name] = "slice"
        elif rhs.startswith("map["):
            out[name] = "map"
        elif rhs in {"int", "int64", "uint64", "bool", "string"}:
            out[name] = rhs
    return out


def type_kind(type_name: str) -> str:
    type_name = type_name.strip()
    if type_name in {"int", "int64", "uint64", "bool", "string", "context.Context"}:
        return type_name
    if type_name in {"[]int", "[]string", "[][]byte", "*[][]byte", "map[string]string"}:
        return type_name
    if type_name.startswith("*") and re.fullmatch(r"\*[A-Za-z_][A-Za-z0-9_]*", type_name):
        return "*named"
    if re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", type_name):
        return "named"
    return "unsupported"


def supported_signature(sig: FuncSig) -> tuple[bool, str]:
    unsupported = [param.type for param in sig.params if type_kind(param.type) == "unsupported"]
    if unsupported:
        return False, "unsupported parameter type: " + ", ".join(unsupported)
    unexported = []
    for param in sig.params:
        if type_kind(param.type) not in {"named", "*named"}:
            continue
        name = param.type[1:] if param.type.startswith("*") else param.type
        if re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", name) and name[:1].islower():
            unexported.append(param.type)
    if unexported:
        return False, "uncallable unexported parameter type: " + ", ".join(unexported)
    return True, ""


def deterministic_value(type_name: str, flavor: int, rng: random.Random) -> Any:
    kind = type_kind(type_name)
    if kind == "int":
        return INT_VALUES[flavor % len(INT_VALUES)]
    if kind == "int64":
        return INT_VALUES[flavor % len(INT_VALUES)]
    if kind == "uint64":
        return UINT_VALUES[flavor % len(UINT_VALUES)]
    if kind == "bool":
        return flavor % 2 == 1
    if kind == "string":
        return STRING_VALUES[flavor % len(STRING_VALUES)]
    if kind == "[]int":
        return list_value(INT_VALUES, flavor, rng)
    if kind == "[]string":
        return list_value(STRING_VALUES, flavor, rng)
    if kind == "[][]byte":
        return bytes_list_value(flavor)
    if kind == "*[][]byte":
        return bytes_list_value(flavor)
    if kind == "map[string]string":
        return None if flavor == 0 else {"k": STRING_VALUES[flavor % len(STRING_VALUES)]}
    if kind == "context.Context":
        return "context.Background()"
    if kind in {"*named", "named"}:
        return None if flavor == 0 else "zero"
    raise ValueError(f"unsupported type {type_name}")


def list_value(values: list[Any], flavor: int, rng: random.Random) -> Any:
    if flavor == 0:
        return None
    if flavor == 1:
        return []
    if flavor < len(values):
        return [values[flavor]]
    length = rng.randint(0, 3)
    return [values[rng.randrange(len(values))] for _ in range(length)]


def bytes_list_value(flavor: int) -> Any:
    if flavor == 0:
        return []
    if flavor == 1:
        return [b"ok"]
    if flavor == 2:
        return [b"!"]
    return [STRING_VALUES[flavor % len(STRING_VALUES)].encode("utf-8")]


def build_seeds(sig: FuncSig, iterations: int, seed: int) -> list[Seed]:
    rng = random.Random(seed)
    count = max(1, iterations)
    seeds: list[Seed] = []
    for index in range(count):
        if index < 12:
            flavor = index
        else:
            flavor = rng.randint(0, 10_000)
        values = [deterministic_value(param.type, flavor, rng) for param in sig.params]
        seeds.append(Seed(label=f"seed-{index}-flavor-{flavor}", flavor=flavor, values=values))
    return seeds


def go_string(text: str) -> str:
    return json.dumps(text)


def go_expr(type_name: str, value: Any, package_alias: str, named_types: dict[str, str]) -> str:
    kind = type_kind(type_name)
    if kind in {"int", "int64"}:
        return f"{kind}({int(value)})" if kind == "int64" else str(int(value))
    if kind == "uint64":
        return f"uint64({int(value)})"
    if kind == "bool":
        return "true" if value else "false"
    if kind == "string":
        return go_string(str(value))
    if kind == "[]int":
        return slice_expr("int", value)
    if kind == "[]string":
        return slice_expr("string", value)
    if kind == "[][]byte":
        return bytes_slice_expr(value)
    if kind == "*[][]byte":
        return "&" + bytes_slice_expr(value)
    if kind == "map[string]string":
        return map_string_expr(value)
    if kind == "context.Context":
        return "context.Background()"
    if kind == "*named":
        return named_pointer_expr(type_name, value, package_alias, named_types)
    if kind == "named":
        return named_value_expr(type_name, value, package_alias, named_types)
    raise ValueError(f"unsupported expression type {type_name}")


def slice_expr(elem_type: str, value: Any) -> str:
    if value is None:
        return f"[]{elem_type}(nil)"
    items = ", ".join(go_string(item) if elem_type == "string" else str(item) for item in value)
    return f"[]{elem_type}{{{items}}}"


def bytes_slice_expr(value: list[bytes]) -> str:
    items = ", ".join(f"[]byte({go_string(item.decode('utf-8'))})" for item in value)
    return f"[][]byte{{{items}}}"


def map_string_expr(value: Any) -> str:
    if value is None:
        return "map[string]string(nil)"
    items = ", ".join(f"{go_string(k)}: {go_string(v)}" for k, v in sorted(value.items()))
    return f"map[string]string{{{items}}}"


def named_pointer_expr(type_name: str, value: Any, package_alias: str, named_types: dict[str, str]) -> str:
    name = type_name[1:]
    if value is None:
        return f"(*{package_alias}.{name})(nil)"
    if named_types.get(name) == "struct":
        return f"&{package_alias}.{name}{{}}"
    return f"(*{package_alias}.{name})(nil)"


def named_value_expr(type_name: str, value: Any, package_alias: str, named_types: dict[str, str]) -> str:
    named_kind = named_types.get(type_name, "")
    if named_kind == "slice":
        return f"{package_alias}.{type_name}(nil)" if value is None else f"{package_alias}.{type_name}{{}}"
    if named_kind == "map":
        return f"{package_alias}.{type_name}(nil)" if value is None else f"{package_alias}.{type_name}{{}}"
    if named_kind == "string":
        return f"{package_alias}.{type_name}({go_string(str(value or ''))})"
    if named_kind in {"int", "int64", "uint64"}:
        numeric = value if isinstance(value, int) else 0
        return f"{package_alias}.{type_name}({numeric})"
    if named_kind == "bool":
        return f"{package_alias}.{type_name}({'true' if value else 'false'})"
    if named_kind == "struct":
        return f"{package_alias}.{type_name}{{}}"
    return f"{package_alias}.{type_name}(0)"


def observable_arg(type_name: str) -> bool:
    return type_kind(type_name) != "context.Context"


def generate_eval_func(package_alias: str, sig: FuncSig, seeds: list[Seed]) -> str:
    lines = [f"func eval{package_alias.title()}(seed int) (obs Observation) {{"]
    lines.append("  defer func() {")
    lines.append("    if r := recover(); r != nil {")
    lines.append("      obs.Panic = normalizeValue(r)")
    lines.append("    }")
    lines.append("  }()")
    lines.append("  switch seed {")
    for index, seed in enumerate(seeds):
        lines.append(f"  case {index}:")
        emit_seed_case(lines, package_alias, sig, seed)
    lines.append("  default:")
    lines.append('    obs.Panic = "invalid seed"')
    lines.append("  }")
    lines.append("  return obs")
    lines.append("}")
    return "\n".join(lines)


def emit_seed_case(lines: list[str], package_alias: str, sig: FuncSig, seed: Seed) -> None:
    arg_names = []
    observed_args = []
    for param_index, param in enumerate(sig.params):
        name = f"p{param_index}"
        expr = go_expr(param.type, seed.values[param_index], package_alias, sig.named_types)
        lines.append(f"    {name} := {expr}")
        arg_names.append(name)
        if observable_arg(param.type):
            observed_args.append(name)
    call = f"{package_alias}.F({', '.join(arg_names)})"
    result_names = [f"r{index}" for index in range(len(sig.returns))]
    if len(result_names) == 0:
        lines.append(f"    {call}")
    elif len(result_names) == 1:
        lines.append(f"    {result_names[0]} := {call}")
    else:
        lines.append(f"    {', '.join(result_names)} := {call}")
    lines.append(f"    obs.Results = []string{{{normalize_list(result_names)}}}")
    lines.append(f"    obs.Args = []string{{{normalize_list(observed_args)}}}")
    lines.append("    return obs")


def normalize_list(names: list[str]) -> str:
    return ", ".join(f"normalizeValue({name})" for name in names)


def generate_harness(sig: FuncSig, seeds: list[Seed]) -> str:
    labels = ", ".join(go_string(seed.label) for seed in seeds)
    imports = [
        '"encoding/json"',
        '"fmt"',
        '"os"',
        '"reflect"',
        '"strconv"',
        '"strings"',
        '"testing"',
        'oldcase "mlse_fuzz_case/oldcase"',
        'newcase "mlse_fuzz_case/newcase"',
    ]
    if any(type_kind(param.type) == "context.Context" for param in sig.params):
        imports.insert(2, '"context"')
    return f"""package harness

import (
{chr(10).join(f"  {item}" for item in imports)}
)

type Observation struct {{
  Panic string
  Results []string
  Args []string
}}

var seedLabels = []string{{{labels}}}

func normalizeValue(v any) string {{
  if v == nil {{
    return "null"
  }}
  rv := reflect.ValueOf(v)
  switch rv.Kind() {{
  case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
    if rv.IsNil() {{
      return "null"
    }}
  }}
  if err, ok := v.(error); ok {{
    return fmt.Sprintf("error:%s", err.Error())
  }}
  if data, err := json.Marshal(v); err == nil {{
    return string(data)
  }}
  return fmt.Sprintf("%#v", v)
}}

func selectedSeeds(t *testing.T) []int {{
  if raw := os.Getenv("MLSE_FUZZ_SEED_INDEX"); raw != "" {{
    index, err := strconv.Atoi(raw)
    if err != nil {{
      t.Fatalf("invalid MLSE_FUZZ_SEED_INDEX: %s", raw)
    }}
    return []int{{index}}
  }}
  raw := os.Getenv("MLSE_FUZZ_SELECTED")
  if raw == "" {{
    out := make([]int, len(seedLabels))
    for index := range seedLabels {{
      out[index] = index
    }}
    return out
  }}
  var out []int
  for _, part := range strings.Split(raw, ",") {{
    index, err := strconv.Atoi(strings.TrimSpace(part))
    if err != nil {{
      t.Fatalf("invalid MLSE_FUZZ_SELECTED item: %s", part)
    }}
    out = append(out, index)
  }}
  return out
}}

func TestDiffSeeds(t *testing.T) {{
  for _, seed := range selectedSeeds(t) {{
    oldObs := evalOldcase(seed)
    newObs := evalNewcase(seed)
    if !reflect.DeepEqual(oldObs, newObs) {{
      t.Fatalf("MLSE_FUZZ_MISMATCH seed=%d label=%s old=%#v new=%#v", seed, seedLabels[seed], oldObs, newObs)
    }}
  }}
}}

{generate_eval_func("oldcase", sig, seeds)}

{generate_eval_func("newcase", sig, seeds)}
"""


def run_go_test(go_bin: str, work_dir: Path, env: dict[str, str]) -> subprocess.CompletedProcess[str]:
    run_env = os.environ.copy()
    run_env.update(env)
    return subprocess.run(
        [
            go_bin,
            "test",
            "-count=1",
            "./harness",
            "-run",
            "TestDiffSeeds",
            "-coverpkg=./oldcase,./newcase",
            "-coverprofile",
            env["MLSE_COVERPROFILE"],
        ],
        cwd=str(work_dir),
        env=run_env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )


def parse_coverage(path: Path) -> tuple[set[str], int]:
    if not path.is_file():
        return set(), 0
    covered: set[str] = set()
    total = 0
    for line in path.read_text(encoding="utf-8").splitlines()[1:]:
        parts = line.split()
        if len(parts) != 3:
            continue
        total += 1
        if int(parts[2]) > 0:
            covered.add(parts[0])
    return covered, total


def prepare_work_dir(case_dir: Path, work_dir: Path, sig: FuncSig, seeds: list[Seed]) -> None:
    shutil.rmtree(work_dir, ignore_errors=True)
    (work_dir / "oldcase").mkdir(parents=True)
    (work_dir / "newcase").mkdir(parents=True)
    (work_dir / "harness").mkdir(parents=True)
    (work_dir / "go.mod").write_text("module mlse_fuzz_case\n\ngo 1.22.0\n", encoding="utf-8")
    old_source = replace_package((case_dir / "old.go").read_text(encoding="utf-8"), "oldcase")
    new_source = replace_package((case_dir / "new.go").read_text(encoding="utf-8"), "newcase")
    (work_dir / "oldcase" / "old.go").write_text(old_source, encoding="utf-8")
    (work_dir / "newcase" / "new.go").write_text(new_source, encoding="utf-8")
    (work_dir / "harness" / "harness_test.go").write_text(generate_harness(sig, seeds), encoding="utf-8")


def run_case(case_dir: Path, config: FuzzRunConfig) -> dict[str, Any]:
    metadata = load_json(case_dir / "case.json")
    out_dir = config.emit_root / metadata["name"]
    work_dir = config.work_root / metadata["name"]
    out_dir.mkdir(parents=True, exist_ok=True)
    shutil.copy2(case_dir / "case.json", out_dir / "case.json")
    shutil.copy2(case_dir / "old.go", out_dir / "old.go")
    shutil.copy2(case_dir / "new.go", out_dir / "new.go")
    result: dict[str, Any] = base_result(metadata, out_dir)
    if not config.go_bin:
        result.update({"status": "skipped", "reason": "go_not_found"})
        return result
    try:
        sig = parse_func_signature((case_dir / "old.go").read_text(encoding="utf-8"))
    except ValueError as exc:
        result.update({"status": "skipped", "reason": str(exc)})
        return result
    ok, reason = supported_signature(sig)
    if not ok:
        result.update({"status": "skipped", "reason": reason})
        return result
    seeds = build_seeds(sig, config.iterations, config.seed)
    prepare_work_dir(case_dir, work_dir, sig, seeds)
    result.update(run_seed_loop(config.go_bin, work_dir, out_dir, seeds))
    result["expectation_met"] = expectation_met(metadata["expected_status"], result["status"])
    return result


def base_result(metadata: dict[str, Any], out_dir: Path) -> dict[str, Any]:
    return {
        "case": metadata["name"],
        "function": metadata["function"],
        "expected_status": metadata["expected_status"],
        "artifact_dir": str(out_dir),
        "mode": "coverage-guided-concrete-fuzz",
    }


def run_seed_loop(go_bin: str, work_dir: Path, out_dir: Path, seeds: list[Seed]) -> dict[str, Any]:
    coverage_dir = out_dir / "coverage"
    coverage_dir.mkdir(parents=True, exist_ok=True)
    covered_union: set[str] = set()
    total_blocks = 0
    selected: list[int] = []
    executed = 0
    for index, seed in enumerate(seeds):
        profile = coverage_dir / f"seed-{index:03d}.out"
        proc = run_go_test(
            go_bin,
            work_dir,
            {
                "MLSE_FUZZ_SEED_INDEX": str(index),
                "MLSE_COVERPROFILE": str(profile),
            },
        )
        executed += 1
        write_run_output(out_dir, index, proc)
        if proc.returncode != 0:
            return classify_failed_seed(
                proc,
                FailedSeedContext(index, seed, executed, selected, covered_union, total_blocks),
            )
        covered, total = parse_coverage(profile)
        total_blocks = max(total_blocks, total)
        if not covered.issubset(covered_union):
            selected.append(index)
            covered_union |= covered
    return {
        "status": "fuzz-no-diff-found",
        "generated_inputs": len(seeds),
        "executed_inputs": executed,
        "selected_inputs": selected,
        "coverage_blocks": len(covered_union),
        "coverage_total_blocks": total_blocks,
        "coverage_ratio": coverage_ratio(len(covered_union), total_blocks),
        "conclusion": "no concrete diff found; this is not an equivalence proof",
    }


def write_run_output(out_dir: Path, index: int, proc: subprocess.CompletedProcess[str]) -> None:
    run_dir = out_dir / "runs"
    run_dir.mkdir(parents=True, exist_ok=True)
    (run_dir / f"seed-{index:03d}.stdout").write_text(proc.stdout, encoding="utf-8")
    (run_dir / f"seed-{index:03d}.stderr").write_text(proc.stderr, encoding="utf-8")


def classify_failed_seed(proc: subprocess.CompletedProcess[str], ctx: FailedSeedContext) -> dict[str, Any]:
    text = proc.stdout + proc.stderr
    if "no required module provides package" in text:
        return {
            "status": "skipped",
            "reason": "external_fixture_package_unavailable",
            "generated_inputs": ctx.index + 1,
            "executed_inputs": ctx.executed,
            "failure_seed": {"seed_index": ctx.index, "label": ctx.seed.label},
        }
    if "MLSE_FUZZ_MISMATCH" in text:
        return {
            "status": "fuzz-counterexample",
            "generated_inputs": ctx.index + 1,
            "executed_inputs": ctx.executed,
            "selected_inputs": ctx.selected,
            "coverage_blocks": len(ctx.covered),
            "coverage_total_blocks": ctx.total,
            "coverage_ratio": coverage_ratio(len(ctx.covered), ctx.total),
            "counterexample": {"seed_index": ctx.index, "label": ctx.seed.label},
        }
    return {
        "status": "blocked",
        "reason": "go_fuzz_harness_failed",
        "generated_inputs": ctx.index + 1,
        "executed_inputs": ctx.executed,
        "failure_seed": {"seed_index": ctx.index, "label": ctx.seed.label},
    }


def coverage_ratio(covered: int, total: int) -> float:
    if total == 0:
        return 0.0
    return round(covered / total, 4)


def expectation_met(expected_status: str, fuzz_status: str) -> bool:
    if expected_status == "counterexample":
        return fuzz_status == "fuzz-counterexample"
    if expected_status == "equivalent":
        return fuzz_status == "fuzz-no-diff-found"
    return False


def summarize(results: list[dict[str, Any]], started: float, tools: dict[str, str | None]) -> dict[str, Any]:
    blocked = [item["case"] for item in results if item["status"] == "blocked"]
    skipped = [item["case"] for item in results if item["status"] == "skipped"]
    unmet = [
        item["case"]
        for item in results
        if item["status"] not in {"blocked", "skipped"} and not item.get("expectation_met", False)
    ]
    status = "ok"
    if blocked:
        status = "blocked"
    elif unmet:
        status = "failure"
    elif skipped:
        status = "partial"
    return {
        "status": status,
        "elapsed_seconds": round(time.time() - started, 3),
        "tools": tools,
        "blocked": blocked,
        "skipped": skipped,
        "expectation_unmet": unmet,
        "results": results,
    }


def main() -> int:
    args = parse_args()
    started = time.time()
    emit_root = resolve_path(args.emit)
    work_root = resolve_path(args.work_dir)
    shutil.rmtree(emit_root, ignore_errors=True)
    shutil.rmtree(work_root, ignore_errors=True)
    emit_root.mkdir(parents=True, exist_ok=True)
    work_root.mkdir(parents=True, exist_ok=True)
    go_bin = discover_go(args.go_bin)
    cases = collect_case_dirs(resolve_path(args.cases_root), args.case)
    config = FuzzRunConfig(emit_root, work_root, go_bin, args.iterations, args.seed)
    results = [run_case(case_dir, config) for case_dir in cases]
    summary = summarize(results, started, {"go": go_bin})
    write_json(emit_root / "summary.json", summary)
    print(json.dumps(summary, indent=2, ensure_ascii=False))
    if args.expect_status and summary["status"] != args.expect_status:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
