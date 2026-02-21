import argparse
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed
from shutil import which
import subprocess


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        prog="install_all_solc.py",
        description=(
            "Install Solidity compiler versions via py-solc-x (solcx)."
        ),
    )
    parser.add_argument(
        "--min",
        dest="min_version",
        default="",
        help="Minimum version to install (inclusive), e.g. 0.4.11",
    )
    parser.add_argument(
        "--max",
        dest="max_version",
        default="",
        help="Maximum version to install (inclusive), e.g. 0.8.23",
    )
    parser.add_argument(
        "--only-major",
        dest="only_major",
        default="",
        help="Only install a major series, e.g. 0.8",
    )
    parser.add_argument(
        "--workers",
        type=int,
        default=4,
        help="Concurrent download workers (default: 4).",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print the plan without installing anything.",
    )
    parser.add_argument(
        "--no-solc-select-fallback",
        action="store_true",
        help="Do not fallback to solc-select on solcx failures.",
    )
    return parser.parse_args()


def to_version_str(v) -> str:
    s = str(v).strip()
    if s.startswith("v"):
        return s[1:]
    return s


def as_tuple(version_str: str) -> tuple[int, int, int]:
    parts = version_str.split(".")
    parts = (parts + ["0", "0", "0"])[:3]
    try:
        return (int(parts[0]), int(parts[1]), int(parts[2]))
    except ValueError:
        return (0, 0, 0)


def normalize_only_major(only_major: str) -> str:
    s = only_major.strip()
    if not s:
        return ""
    if s.startswith("v"):
        s = s[1:]
    if s.count(".") == 0:
        return f"{s}."
    if s.count(".") == 1:
        return f"{s}."
    return s


def has_solc_select() -> bool:
    return which("solc-select") is not None


def solc_select_installed() -> set[str]:
    if not has_solc_select():
        return set()
    try:
        res = subprocess.run(
            ["solc-select", "versions"],
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
    except Exception:
        return set()

    installed: set[str] = set()
    for line in res.stdout.splitlines():
        s = line.strip()
        if not s:
            continue
        if " " in s:
            s = s.split(" ", 1)[0].strip()
        if s and s[0].isdigit():
            installed.add(s)
    return installed


def solc_select_install(version_str: str) -> bool:
    if not has_solc_select():
        return False
    res = subprocess.run(
        ["solc-select", "install", version_str],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    return res.returncode == 0


def main() -> int:
    args = parse_args()
    try:
        import solcx  # type: ignore
    except Exception as e:
        sys.stderr.write("Missing dependency: py-solc-x (solcx)\n")
        sys.stderr.write("Install it first:\n")
        sys.stderr.write("  pip3 install py-solc-x --break-system-packages\n")
        sys.stderr.write(f"Details: {e}\n")
        return 2

    min_v = to_version_str(args.min_version)
    max_v = to_version_str(args.max_version)
    only_major_prefix = normalize_only_major(args.only_major)

    installable = [
        to_version_str(v)
        for v in solcx.get_installable_solc_versions()
    ]
    installed = {
        to_version_str(v)
        for v in solcx.get_installed_solc_versions()
    }
    solc_select_have = solc_select_installed()

    plan = []
    for v in installable:
        if only_major_prefix and not v.startswith(only_major_prefix):
            continue
        if min_v and as_tuple(v) < as_tuple(min_v):
            continue
        if max_v and as_tuple(v) > as_tuple(max_v):
            continue
        if v in installed or v in solc_select_have:
            continue
        plan.append(v)

    plan.sort(key=as_tuple)

    sys.stdout.write(f"Installable: {len(installable)}\n")
    sys.stdout.write(f"Already installed: {len(installed)}\n")
    sys.stdout.write(f"To install: {len(plan)}\n")

    if args.dry_run:
        if plan:
            sys.stdout.write("Planned versions:\n")
            for v in plan:
                sys.stdout.write(f"  {v}\n")
        return 0

    if not plan:
        return 0

    workers = max(1, int(args.workers))
    failures: list[tuple[str, str]] = []

    allow_fallback = (not args.no_solc_select_fallback) and has_solc_select()

    def install_one(version_str: str) -> None:
        try:
            solcx.install_solc(f"v{version_str}")
        except Exception:
            if not allow_fallback:
                raise
            ok = solc_select_install(version_str)
            if not ok:
                raise

    with ThreadPoolExecutor(max_workers=workers) as executor:
        futures = {executor.submit(install_one, v): v for v in plan}
        done = 0
        total = len(plan)
        for f in as_completed(futures):
            v = futures[f]
            try:
                f.result()
                done += 1
                sys.stdout.write(f"[{done}/{total}] installed {v}\n")
            except Exception as e:
                failures.append((v, str(e)))
                sys.stderr.write(f"[FAIL] {v}: {e}\n")

    if failures:
        sys.stderr.write(f"Failed: {len(failures)}\n")
        for v, msg in failures[:20]:
            sys.stderr.write(f"  {v}: {msg}\n")
        if len(failures) > 20:
            sys.stderr.write("  ...\n")
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
