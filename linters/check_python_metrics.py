#!/usr/bin/env python3
from __future__ import annotations

import argparse
import ast
import sys
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check Python file and function size limits.")
    parser.add_argument("--root", default=".", help="repository root")
    parser.add_argument("--include", default="scripts,linters", help="comma-separated include directories")
    parser.add_argument("--exclude", default="tmp,artifacts,.git", help="comma-separated directory names to skip")
    parser.add_argument("--max-params", type=int, default=5, help="maximum function parameters")
    parser.add_argument("--max-function-lines", type=int, default=200, help="maximum function length")
    parser.add_argument("--max-file-lines", type=int, default=2000, help="maximum file length")
    return parser.parse_args()


def split_csv(text: str) -> list[str]:
    return [item.strip() for item in text.split(",") if item.strip()]


def iter_python_files(root: Path, includes: list[str], excludes: set[str]) -> list[Path]:
    files: list[Path] = []
    for include in includes:
        base = root / include
        if not base.is_dir():
            continue
        for path in sorted(base.rglob("*.py")):
            if any(part in excludes for part in path.relative_to(root).parts):
                continue
            files.append(path)
    return files


class MetricsVisitor(ast.NodeVisitor):
    def __init__(self, path: Path, max_params: int, max_function_lines: int) -> None:
        self.path = path
        self.max_params = max_params
        self.max_function_lines = max_function_lines
        self.class_depth = 0
        self.violations: list[str] = []

    def visit_ClassDef(self, node: ast.ClassDef) -> None:
        self.class_depth += 1
        self.generic_visit(node)
        self.class_depth -= 1

    def visit_FunctionDef(self, node: ast.FunctionDef) -> None:
        self._check_function(node)
        self.generic_visit(node)

    def visit_AsyncFunctionDef(self, node: ast.AsyncFunctionDef) -> None:
        self._check_function(node)
        self.generic_visit(node)

    def _check_function(self, node: ast.FunctionDef | ast.AsyncFunctionDef) -> None:
        param_count = count_python_params(node, self.class_depth > 0)
        if param_count > self.max_params:
            self.violations.append(
                f"{self.path}:{node.lineno}: function {node.name!r} has {param_count} parameters, exceeds limit {self.max_params}"
            )

        end_lineno = getattr(node, "end_lineno", node.lineno)
        length = end_lineno - node.lineno + 1
        if length > self.max_function_lines:
            self.violations.append(
                f"{self.path}:{node.lineno}: function {node.name!r} has {length} lines, exceeds limit {self.max_function_lines}"
            )


def count_python_params(node: ast.FunctionDef | ast.AsyncFunctionDef, drop_self: bool) -> int:
    args = node.args
    count = len(args.posonlyargs) + len(args.args) + len(args.kwonlyargs)
    if args.vararg is not None:
        count += 1
    if args.kwarg is not None:
        count += 1

    if drop_self and args.args:
        first = args.args[0].arg
        if first in {"self", "cls"}:
            count -= 1
    return count


def main() -> int:
    args = parse_args()
    root = Path(args.root).resolve()
    includes = split_csv(args.include)
    excludes = set(split_csv(args.exclude))
    violations: list[str] = []

    for path in iter_python_files(root, includes, excludes):
        rel = path.relative_to(root).as_posix()
        text = path.read_text(encoding="utf-8")
        line_count = len(text.splitlines())
        if line_count > args.max_file_lines:
            violations.append(
                f"{rel}:1: file has {line_count} lines, exceeds limit {args.max_file_lines}"
            )

        try:
            tree = ast.parse(text, filename=rel)
        except SyntaxError as exc:
            violations.append(f"{rel}:{exc.lineno or 1}: syntax error: {exc.msg}")
            continue

        visitor = MetricsVisitor(Path(rel), args.max_params, args.max_function_lines)
        visitor.visit(tree)
        violations.extend(visitor.violations)

    if violations:
        for violation in sorted(violations):
            print(violation, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
