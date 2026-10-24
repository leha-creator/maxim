#!/usr/bin/env bash
# archive_command PostgreSQL: wal-archive.sh %p %f — сегмент WAL в локальный архив (том
# wal-archive). Выгрузку в S3 делает контейнер backup (wal-upload.sh). Правила
# archive_command: не перезаписывать существующий файл; повтор того же сегмента — успех;
# код, отличный от 0, — PostgreSQL повторит позже и не удалит сегмент.
set -Eeuo pipefail

src=$1
name=$2
dir=${WAL_ARCHIVE_DIR:-/var/lib/postgresql/wal-archive}
dst="$dir/$name"

if [[ -e $dst ]]; then
  if cmp -s "$src" "$dst"; then
    exit 0
  fi
  printf '{"level":"error","component":"wal-archive","event":"wal_archive_conflict","segment":"%s"}\n' "$name" >&2
  exit 1
fi
if ! cp "$src" "$dst.tmp" || ! sync "$dst.tmp" || ! mv "$dst.tmp" "$dst"; then
  rm -f "$dst.tmp"
  printf '{"level":"error","component":"wal-archive","event":"wal_archive_failed","segment":"%s"}\n' "$name" >&2
  exit 1
fi
