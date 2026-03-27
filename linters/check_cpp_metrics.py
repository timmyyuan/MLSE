#!/usr/bin/env python3
from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

CPP_EXTENSIONS = {".c", ".cc", ".cpp", ".cxx", ".h", ".hh", ".hpp"}
BLOCK_KEYWORDS = {"if", "for", "while", "switch", "catch", "else", "do", "try"}
TYPE_KEYWORDS = {"class", "struct", "namespace", "enum", "union"}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check C/C++ file and function size limits.")
    parser.add_argument("--root", default=".", help="repository root")
    parser.add_argument("--include", default="include,lib,tools", help="comma-separated include directories")
    parser.add_argument("--exclude", default="tmp,artifacts,.git", help="comma-separated directory names to skip")
    parser.add_argument("--max-params", type=int, default=5, help="maximum function parameters")
    parser.add_argument("--max-function-lines", type=int, default=200, help="maximum function length")
    parser.add_argument("--max-file-lines", type=int, default=2000, help="maximum file length")
    return parser.parse_args()


def split_csv(text: str) -> list[str]:
    return [item.strip() for item in text.split(",") if item.strip()]


def iter_cpp_files(root: Path, includes: list[str], excludes: set[str]) -> list[Path]:
    files: list[Path] = []
    for include in includes:
        base = root / include
        if not base.is_dir():
            continue
        for path in sorted(base.rglob("*")):
            if not path.is_file() or path.suffix not in CPP_EXTENSIONS:
                continue
            if any(part in excludes for part in path.relative_to(root).parts):
                continue
            files.append(path)
    return files


def strip_comments_and_strings(text: str) -> str:
    out: list[str] = []
    i = 0
    in_line_comment = False
    in_block_comment = False
    in_string = False
    string_quote = ""
    while i < len(text):
        ch = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ""
        if in_line_comment:
            out.append("\n" if ch == "\n" else " ")
            if ch == "\n":
                in_line_comment = False
            i += 1
            continue
        if in_block_comment:
            if ch == "*" and nxt == "/":
                out.extend([" ", " "])
                in_block_comment = False
                i += 2
            else:
                out.append("\n" if ch == "\n" else " ")
                i += 1
            continue
        if in_string:
            if ch == "\\" and i + 1 < len(text):
                out.extend([" ", " "])
                i += 2
                continue
            out.append("\n" if ch == "\n" else " ")
            if ch == string_quote:
                in_string = False
            i += 1
            continue
        if ch == "/" and nxt == "/":
            out.extend([" ", " "])
            in_line_comment = True
            i += 2
            continue
        if ch == "/" and nxt == "*":
            out.extend([" ", " "])
            in_block_comment = True
            i += 2
            continue
        if ch in {'"', "'"}:
            out.append(" ")
            in_string = True
            string_quote = ch
            i += 1
            continue
        out.append(ch)
        i += 1
    return "".join(out)


def extract_signature(lines: list[str], line_index: int) -> tuple[int, str]:
    start = line_index
    while start > 0:
        prev = lines[start - 1].strip()
        if not prev:
            start -= 1
            continue
        if prev.startswith("#") or prev.endswith(";") or prev.endswith("}"):
            break
        start -= 1
    signature = " ".join(line.strip() for line in lines[start : line_index + 1])
    return start, signature


def looks_like_function(signature: str) -> bool:
    if "{" not in signature or "(" not in signature or ")" not in signature:
        return False
    head = signature.split("{", 1)[0].strip()
    if any(re.match(rf"^{kw}\b", head) for kw in BLOCK_KEYWORDS | TYPE_KEYWORDS):
        return False
    if head.startswith("[") or re.search(r"\]\s*\(", head):
        return False
    if re.search(r"\btemplate\s*<", head) and "(" not in head.split(">", 1)[-1]:
        return False
    token = function_name_token(head)
    if token in BLOCK_KEYWORDS or token in TYPE_KEYWORDS or token == "":
        return False
    return True


def function_name_token(head: str) -> str:
    param_start, _ = find_parameter_span(head)
    prefix = head[:param_start].rstrip()
    match = re.search(r"([~\w:]+|operator\s*[^\s(]+)$", prefix)
    return match.group(1) if match else ""


def find_parameter_span(head: str) -> tuple[int, int]:
    close_index = head.rfind(")")
    if close_index == -1:
        return -1, -1
    depth = 0
    for idx in range(close_index, -1, -1):
        ch = head[idx]
        if ch == ")":
            depth += 1
        elif ch == "(":
            depth -= 1
            if depth == 0:
                return idx, close_index
    return -1, -1


def count_params(param_text: str) -> int:
    text = param_text.strip()
    if not text or text == "void":
        return 0
    depth_angle = depth_paren = depth_bracket = depth_brace = 0
    count = 1
    for ch in text:
        if ch == "<":
            depth_angle += 1
        elif ch == ">":
            depth_angle = max(0, depth_angle - 1)
        elif ch == "(":
            depth_paren += 1
        elif ch == ")":
            depth_paren = max(0, depth_paren - 1)
        elif ch == "[":
            depth_bracket += 1
        elif ch == "]":
            depth_bracket = max(0, depth_bracket - 1)
        elif ch == "{":
            depth_brace += 1
        elif ch == "}":
            depth_brace = max(0, depth_brace - 1)
        elif ch == "," and depth_angle == depth_paren == depth_bracket == depth_brace == 0:
            count += 1
    return count


def find_function_end(lines: list[str], start_line: int) -> int:
    depth = 0
    seen_open = False
    for idx in range(start_line, len(lines)):
        for ch in lines[idx]:
            if ch == "{":
                depth += 1
                seen_open = True
            elif ch == "}":
                depth -= 1
                if seen_open and depth == 0:
                    return idx
    return start_line


def check_cpp_file(path: Path, root: Path, args: argparse.Namespace) -> list[str]:
    rel = path.relative_to(root).as_posix()
    text = path.read_text(encoding="utf-8")
    violations: list[str] = []

    line_count = len(text.splitlines())
    if line_count > args.max_file_lines:
        violations.append(f"{rel}:1: file has {line_count} lines, exceeds limit {args.max_file_lines}")

    cleaned = strip_comments_and_strings(text)
    lines = cleaned.splitlines()
    for idx, line in enumerate(lines):
        if "{" not in line:
            continue
        start_idx, signature = extract_signature(lines, idx)
        if not looks_like_function(signature):
            continue

        param_start, param_end = find_parameter_span(signature)
        if param_start == -1:
            continue
        params = count_params(signature[param_start + 1 : param_end])
        name = function_name_token(signature)
        line_no = start_idx + 1
        if params > args.max_params:
            violations.append(
                f"{rel}:{line_no}: function {name!r} has {params} parameters, exceeds limit {args.max_params}"
            )

        end_idx = find_function_end(lines, idx)
        length = end_idx - start_idx + 1
        if length > args.max_function_lines:
            violations.append(
                f"{rel}:{line_no}: function {name!r} has {length} lines, exceeds limit {args.max_function_lines}"
            )
    return violations


def main() -> int:
    args = parse_args()
    root = Path(args.root).resolve()
    includes = split_csv(args.include)
    excludes = set(split_csv(args.exclude))
    violations: list[str] = []

    for path in iter_cpp_files(root, includes, excludes):
        violations.extend(check_cpp_file(path, root, args))

    if violations:
        for violation in sorted(violations):
            print(violation, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
