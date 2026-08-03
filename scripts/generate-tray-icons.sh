#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)
temporary_dir=$(mktemp -d "${TMPDIR:-/tmp}/dkst-tray-icons.XXXXXX")
trap 'rm -rf "$temporary_dir"' EXIT HUP INT TERM
target=${1:-all}

case "$target" in
  all|darwin|linux|windows) ;;
  *)
    echo "usage: $0 [all|darwin|linux|windows]" >&2
    exit 2
    ;;
esac

if ! command -v rsvg-convert >/dev/null 2>&1; then
  echo "rsvg-convert is required (librsvg)." >&2
  exit 1
fi

if [ "$target" = "all" ] || [ "$target" = "darwin" ]; then
  rsvg-convert --width 36 --height 36 \
    --output "$project_dir/build/darwin/trayicon.png" \
    "$project_dir/build/darwin/trayicon.svg"
fi

if [ "$target" = "all" ] || [ "$target" = "linux" ]; then
  rsvg-convert --width 64 --height 64 \
    --output "$project_dir/build/linux/trayicon.png" \
    "$project_dir/build/linux/trayicon.svg"
fi

if [ "$target" = "all" ] || [ "$target" = "windows" ]; then
  png_files=""
  for size in 16 20 24 32 48 64 128 256; do
    png_file="$temporary_dir/trayicon-${size}.png"
    rsvg-convert --width "$size" --height "$size" \
      --output "$png_file" \
      "$project_dir/build/windows/trayicon.svg"
    png_files="$png_files $png_file"
  done

  # shellcheck disable=SC2086
  go run "$project_dir/scripts/trayiconpack" \
    -output "$project_dir/build/windows/trayicon.ico" \
    $png_files
fi

echo "Generated tray icon target: $target"
