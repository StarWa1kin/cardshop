#!/usr/bin/env python3
"""Hash-guarded source rollback. Dry run by default; does not touch database/builds."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import tarfile


def sha(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True)
    parser.add_argument('--apply', action='store_true', help='Restore original source after all hashes match')
    args = parser.parse_args()
    bundle = Path(__file__).resolve().parent
    manifest = json.loads((bundle / 'manifest.json').read_text())
    root = args.root.resolve(strict=True)
    backup = (bundle / 'baseline-sources.tar.gz').read_bytes()
    if sha(backup) != manifest['baseline_archive_sha256']:
        raise SystemExit('STOP: backup checksum mismatch')
    originals = {}
    with tarfile.open(fileobj=io.BytesIO(backup), mode='r:gz') as archive:
        for item in manifest['files']:
            rel = Path(item['path'])
            if rel.is_absolute() or '..' in rel.parts:
                raise SystemExit('STOP: unexpected source path')
            path = root / rel
            if not path.resolve().is_relative_to(root) or path.is_symlink():
                raise SystemExit(f'STOP: symlink or path boundary: {rel}')
            if not path.is_file() or sha(path.read_bytes()) != item['modified_sha256']:
                raise SystemExit(f'STOP: subsequent changes or missing source: {rel}')
            if item['baseline_sha256'] is not None:
                data = archive.extractfile(str(rel)).read()
                if sha(data) != item['baseline_sha256']:
                    raise SystemExit(f'STOP: baseline checksum: {rel}')
                originals[str(rel)] = data
    print(f"CHECK OK: {len(manifest['files'])} source files match delivery hashes")
    if not args.apply:
        print('DRY RUN: no files changed; use --apply to restore source')
        return
    for item in manifest['files']:
        path = root / item['path']
        if item['path'] in originals:
            path.write_bytes(originals[item['path']])
        else:
            path.unlink()
    print('ROLLBACK OK: original source restored; settings/database/builds unchanged')


if __name__ == '__main__':
    main()
