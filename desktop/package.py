#!/usr/bin/env python3
"""Build and package Slate Desktop using the system WebView on each target."""

import argparse
import gzip
import os
from pathlib import Path
import plistlib
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import zipfile

ROOT = Path(__file__).resolve().parent.parent
ICON = ROOT / "desktop/icon.png"
TARGETS = {
    "windows_amd64": "zip",
    "darwin_amd64": "dmg",
    "darwin_arm64": "dmg",
    "linux_amd64": "tar.gz",
    "linux_arm64": "tar.gz",
}


def run(*args, **kwargs):
    subprocess.run(args, check=True, cwd=ROOT, **kwargs)


def git(*args):
    return subprocess.check_output(("git", *args), cwd=ROOT, text=True).strip()


def build(target, binary, version):
    goos, arch = target.split("_")
    env = os.environ | {"GOOS": goos, "GOARCH": arch, "CGO_ENABLED": "0"}
    ldflags = (
        f"-s -w -X main.buildVersion={version} -X main.gitBranch=desktop "
        f"-X main.gitHash={git('rev-parse', '--short=7', 'HEAD')} "
        f"-X main.buildTime={int(time.time())}"
    )
    if goos == "windows":
        ldflags += " -H windowsgui"
    run(
        "go", "build", "-tags", "slate_desktop,embed_web,nomsgpack,purego,production",
        "-trimpath", "-ldflags", ldflags, "-o", binary, "./cmd/desktop", env=env,
    )


def mac_app(binary, stage, version):
    volume = stage / "volume"
    app = volume / "Slate.app"
    contents = app / "Contents"
    executable = contents / "MacOS" / binary.name
    executable.parent.mkdir(parents=True)
    shutil.move(binary, executable)
    resources = contents / "Resources"
    resources.mkdir()
    version_number = re.match(r"v([0-9]+\.[0-9]+\.[0-9]+)", version)[1]
    with (contents / "Info.plist").open("wb") as plist:
        plistlib.dump({
            "CFBundleDisplayName": "Slate",
            "CFBundleExecutable": binary.name,
            "CFBundleIconFile": "Slate.icns",
            "CFBundleIdentifier": "com.ixugo.slate.desktop",
            "CFBundleName": "Slate",
            "CFBundlePackageType": "APPL",
            "CFBundleShortVersionString": version_number,
            "CFBundleVersion": version_number,
            "NSHighResolutionCapable": True,
        }, plist)

    iconset = stage / "Slate.iconset"
    iconset.mkdir()
    for size in (16, 32, 128, 256, 512):
        for scale, suffix in ((1, ""), (2, "@2x")):
            pixels = size * scale
            run("sips", "-z", str(pixels), str(pixels), ICON, "--out",
                iconset / f"icon_{size}x{size}{suffix}.png", stdout=subprocess.DEVNULL)
    run("iconutil", "--convert", "icns", "--output", resources / "Slate.icns", iconset)
    identity = os.environ.get("SLATE_DESKTOP_SIGN_IDENTITY", "-")
    sign_options = ["--options", "runtime", "--timestamp"] if identity != "-" else []
    run("codesign", "--force", "--sign", identity, *sign_options, app)
    run("codesign", "--verify", "--strict", app)
    (volume / "Applications").symlink_to("/Applications")
    return volume


def pack(binary, stage, output, target, version, upx):
    extension = TARGETS[target]
    if extension == "dmg":
        volume = mac_app(binary, stage, version)
        run("hdiutil", "create", "-quiet", "-fs", "HFS+", "-volname", "Slate",
            "-format", "UDZO", "-imagekey", "zlib-level=9", "-srcfolder", volume, output)
        return

    if upx and (compressor := shutil.which(upx)):
        env = {k: v for k, v in os.environ.items() if k != "UPX"}
        run(compressor, "--best", "--lzma", "-q", binary, env=env)
        run(compressor, "-t", "-q", binary, env=env)
    if extension == "zip":
        with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
            archive.write(binary, binary.name)
    else:
        with gzip.open(output, "wb", compresslevel=9) as compressed:
            with tarfile.open(fileobj=compressed, mode="w") as archive:
                archive.add(binary, binary.name)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", required=True, choices=TARGETS)
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--upx", default="upx")
    parser.add_argument("--build-only", action="store_true")
    args = parser.parse_args()
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:[-+.][0-9A-Za-z.-]+)?", args.version):
        parser.error("version must look like v1.2.3 or v1.2.3-rc.1")
    extension = TARGETS[args.target]
    if extension == "dmg" and not args.build_only and sys.platform != "darwin":
        parser.error("DMG packaging requires macOS")
    output_dir = args.output.resolve()
    output_dir.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".slate-desktop-", dir=output_dir) as tmp:
        stage = Path(tmp)
        binary_name = "slate_desktop.exe" if extension == "zip" else "slate_desktop"
        binary = stage / binary_name
        build(args.target, binary, args.version)
        artifact = binary
        if not args.build_only:
            artifact = stage / f"slate_desktop_{args.version}_{args.target}.{extension}"
            pack(binary, stage, artifact, args.target, args.version, args.upx)
        final = output_dir / artifact.name
        artifact.replace(final)
        print(f"{final}: {final.stat().st_size:,} bytes")


if __name__ == "__main__":
    main()
