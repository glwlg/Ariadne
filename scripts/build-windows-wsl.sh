#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
unset GOOS GOARCH

expected_wails=$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)
actual_wails=$(wails3 version 2>&1)
if [[ "$actual_wails" != "$expected_wails" ]]; then
  printf 'Wails CLI version %s does not match project version %s\n' "$actual_wails" "$expected_wails" >&2
  exit 1
fi

node --input-type=commonjs <<'NODE'
const fs = require('node:fs');
const crypto = require('node:crypto');
const path = require('node:path');
try {
  const script = fs.readFileSync('scripts/build-uxplay.ps1', 'utf8');
  const commit = script.match(/^\$sourceCommit = '([0-9a-f]{40})'/m)?.[1];
  const revision = script.match(/^\$bundleRevision = (\d+)/m)?.[1];
  if (!commit || !revision) throw new Error('Cannot read the UxPlay source version from scripts/build-uxplay.ps1');
  const inputs = [
    'scripts/build-uxplay.ps1', 'scripts/uxplay/fancun-window.c',
    'scripts/uxplay/fancun-window.h', 'scripts/uxplay/fancun.rc',
    'scripts/uxplay/test-window-branding.c', 'scripts/uxplay/test-mdns-windows.c',
    'assets/logo.ico',
  ];
  const fingerprint = inputs.map(file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex').toUpperCase()).join(':');
  const runtime = 'bin/uxplay';
  const manifest = JSON.parse(fs.readFileSync(path.join(runtime, 'runtime.json'), 'utf8').replace(/^\uFEFF/, ''));
  if (manifest.sourceCommit !== commit || Number(manifest.bundleRevision) !== Number(revision) || manifest.buildInputs !== fingerprint) {
    throw new Error('UxPlay runtime does not match the current source inputs');
  }
  for (const file of ['uxplay.exe', 'gst-inspect-1.0.exe', 'libexec/gstreamer-1.0/gst-plugin-scanner.exe']) {
    if (!fs.statSync(path.join(runtime, file)).isFile()) throw new Error(`Missing UxPlay runtime file: ${file}`);
  }
  console.log('UxPlay runtime matches the current source inputs.');
} catch (error) {
  console.error(`UxPlay runtime check failed: ${error.message}\nRebuild and copy the Windows UxPlay runtime as described in README.md.`);
  process.exit(1);
}
NODE

pnpm --dir frontend test
dotnet run --project native/Fancun.CaptureHost.Tests/Fancun.CaptureHost.Tests.csproj -c Release
dotnet run --project native/Fancun.iPhoneControl.Tests/Fancun.iPhoneControl.Tests.csproj -c Release
rm -rf -- bin/native-capture bin/iphone-control
dotnet publish native/Fancun.CaptureHost/Fancun.CaptureHost.csproj -c Release -r win-x64 --self-contained true \
  -p:EnableWindowsTargeting=true -p:PublishSingleFile=true -p:EnableCompressionInSingleFile=true \
  -p:IncludeNativeLibrariesForSelfExtract=true -p:DebugType=none -p:DebugSymbols=false -o bin/native-capture
dotnet publish native/Fancun.iPhoneControl/Fancun.iPhoneControl.csproj -c Release -r win-x64 --self-contained true \
  -p:EnableWindowsTargeting=true -p:PublishSingleFile=true -p:EnableCompressionInSingleFile=true \
  -p:IncludeNativeLibrariesForSelfExtract=true -p:DebugType=none -p:DebugSymbols=false -o bin/iphone-control
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 wails3 generate bindings -clean=false
pnpm --dir frontend build
go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --arch amd64 --out fancun_resource.syso --no-suffix
version=${FANCUN_VERSION:-dev}
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-H windowsgui -X main.appVersion=$version" -o bin/fancun.exe .
go run ./cmd/releasepack -version "$version" > /dev/null
printf 'Windows installer and checksum: dist/release/FancunSetup-%s-windows-x64.exe, dist/release/SHA256SUMS\n' "$version"
