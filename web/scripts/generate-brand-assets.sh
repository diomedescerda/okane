#!/usr/bin/env bash
# Deterministic exports of the approved artwork. Requires ImageMagick 7.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p public/brand public/icons
paper='#f8f3e7'
magick branding/seal-source.png -trim +repage -resize 320x320 -strip -define webp:lossless=true public/brand/seal.webp
# The approved 1254px source has the wordmark below y=850.
magick branding/logo-source.png -crop 1254x404+0+850 +repage -trim +repage -resize 480x180 -strip -define webp:lossless=true public/brand/wordmark.webp
for size in 192 512; do
  artwork=$((size * 80 / 100))
  magick branding/seal-source.png -trim +repage -resize "${artwork}x${artwork}" -gravity center -background "$paper" -extent "${size}x${size}" -alpha remove -alpha off -strip "public/icons/seal-${size}.png"
done
# A 56% square fits wholly inside the maskable icon's central 80% circle.
magick branding/seal-source.png -trim +repage -resize 286x286 -gravity center -background "$paper" -extent 512x512 -alpha remove -alpha off -strip public/icons/seal-maskable-512.png
magick branding/seal-source.png -trim +repage -resize 144x144 -gravity center -background "$paper" -extent 180x180 -alpha remove -alpha off -strip public/apple-touch-icon.png
magick branding/seal-source.png -trim +repage -resize 27x27 -gravity center -background "$paper" -extent 32x32 -alpha remove -alpha off -strip public/favicon-32.png
