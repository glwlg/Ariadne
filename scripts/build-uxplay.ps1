param(
    [string]$OutputDir = (Join-Path $PSScriptRoot '..\bin\uxplay'),
    [string]$BuildDir = (Join-Path $PSScriptRoot '..\.codex-tmp\uxplay-build')
)

$ErrorActionPreference = 'Stop'
$sourceCommit = '4e95a9ec013c52d8555d88dbc55abe346e94d56e'
$bundleRevision = 9
function Get-SHA256([string]$Path) {
    $stream = [IO.File]::OpenRead($Path)
    $algorithm = [Security.Cryptography.SHA256]::Create()
    try {
        return [BitConverter]::ToString($algorithm.ComputeHash($stream)).Replace('-', '')
    } finally {
        $stream.Dispose()
        $algorithm.Dispose()
    }
}
$buildInputs = (@(
    $PSCommandPath,
    (Join-Path $PSScriptRoot 'uxplay\ariadne-window.c'),
    (Join-Path $PSScriptRoot 'uxplay\ariadne-window.h'),
    (Join-Path $PSScriptRoot 'uxplay\ariadne.rc'),
    (Join-Path $PSScriptRoot 'uxplay\test-window-branding.c'),
    (Join-Path $PSScriptRoot 'uxplay\test-mdns-windows.c'),
    (Join-Path $PSScriptRoot '..\assets\logo.ico')
) | ForEach-Object { Get-SHA256 $_ }) -join ':'
$OutputDir = [IO.Path]::GetFullPath($OutputDir)
$BuildDir = [IO.Path]::GetFullPath($BuildDir)
$manifestPath = Join-Path $OutputDir 'runtime.json'
if ((Test-Path -LiteralPath $manifestPath) -and (Test-Path -LiteralPath (Join-Path $OutputDir 'uxplay.exe'))) {
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    if ($manifest.sourceCommit -eq $sourceCommit -and $manifest.bundleRevision -eq $bundleRevision -and $manifest.buildInputs -eq $buildInputs) {
        Write-Output "UxPlay runtime ready: $OutputDir"
        return
    }
}

if ($env:PROCESSOR_ARCHITECTURE -ne 'AMD64') {
    throw 'The bundled UxPlay build requires Windows x64.'
}
New-Item -ItemType Directory -Force -Path $BuildDir, $OutputDir | Out-Null

function Get-VerifiedDownload([string]$Uri, [string]$Path, [string]$SHA256) {
    if (-not (Test-Path -LiteralPath $Path)) {
        & curl.exe --fail --location --retry 3 --output $Path $Uri
        if ($LASTEXITCODE -ne 0) { throw "Download failed: $Uri" }
    }
    $actualHash = Get-SHA256 $Path
    if ($actualHash -ne $SHA256) {
        throw "SHA256 mismatch: $Path"
    }
}

$baseArchive = Join-Path $BuildDir 'msys2-base.tar.xz'
$bash = Join-Path $BuildDir 'msys64\usr\bin\bash.exe'
if (-not (Test-Path -LiteralPath $bash)) {
    Get-VerifiedDownload `
        'https://github.com/msys2/msys2-installer/releases/download/2026-09-27/msys2-base-x86_64-20260927.tar.xz' `
        $baseArchive 'ea2f31a0b6ade63914ce441ffb022f0f6aa96982bfefa2326460a26d5fb01322'
    & tar.exe -xf $baseArchive -C $BuildDir
    if ($LASTEXITCODE -ne 0) { throw 'Could not extract MSYS2.' }
}
$sourceArchive = Join-Path $BuildDir 'uxplay-source.zip'
Get-VerifiedDownload "https://codeload.github.com/FDH2/UxPlay/zip/$sourceCommit" `
    $sourceArchive '42b02e6456f2c16e0fb56082a16c9e867380aa611169b3de5898ecd9618c0581'
$sourceDir = Join-Path $BuildDir "UxPlay-$sourceCommit"
if (-not (Test-Path -LiteralPath $sourceDir)) {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [IO.Compression.ZipFile]::ExtractToDirectory($sourceArchive, $BuildDir)
}

# Hyper-V external switches can carry the actual LAN gateway.
$mdnsSourcePath = Join-Path $sourceDir 'lib\mdnsd\mdnsd.c'
$mdnsSource = [IO.File]::ReadAllText($mdnsSourcePath).Replace("`r`n", "`n")
$excludedVirtualAdapters = @'
            // Precise target filters to bypass the Hyper-V virtual adapter loopback capture
            if (pCurr->OperStatus == IfOperStatusUp && 
                pCurr->IfType != IF_TYPE_SOFTWARE_LOOPBACK &&
                pCurr->FirstGatewayAddress != NULL && 
                wcsstr(pCurr->Description, L"Hyper-V") == NULL &&
                wcsstr(pCurr->Description, L"Virtual") == NULL &&
                wcsstr(pCurr->Description, L"WSL") == NULL &&
                wcsstr(pCurr->Description, L"vEthernet") == NULL) {
'@
$gatewayAdapters = @'
            // A Hyper-V external switch can be the LAN interface with a gateway.
            if (pCurr->OperStatus == IfOperStatusUp &&
                pCurr->IfType != IF_TYPE_SOFTWARE_LOOPBACK &&
                pCurr->FirstGatewayAddress != NULL) {
'@
$excludedVirtualAdapters = $excludedVirtualAdapters.Replace("`r`n", "`n")
$gatewayAdapters = $gatewayAdapters.Replace("`r`n", "`n")
if ($mdnsSource.Contains($excludedVirtualAdapters)) {
    [IO.File]::WriteAllText($mdnsSourcePath, $mdnsSource.Replace($excludedVirtualAdapters, $gatewayAdapters), [Text.UTF8Encoding]::new($false))
} elseif (-not $mdnsSource.Contains($gatewayAdapters)) {
    throw 'UxPlay mDNS adapter source differs from the pinned version.'
}

# Brand every native video window when GStreamer publishes its handle.
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'uxplay\ariadne-window.h') -Destination (Join-Path $sourceDir 'renderers\ariadne-window.h') -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'uxplay\ariadne-window.c') -Destination (Join-Path $sourceDir 'renderers\ariadne-window.c') -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'uxplay\ariadne.rc') -Destination (Join-Path $sourceDir 'ariadne.rc') -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot '..\assets\logo.ico') -Destination (Join-Path $sourceDir 'ariadne.ico') -Force
$rendererSourcePath = Join-Path $sourceDir 'renderers\video_renderer.c'
$rendererSource = [IO.File]::ReadAllText($rendererSourcePath).Replace("`r`n", "`n")
$overlayInclude = '#include <gst/video/videooverlay.h>'
$brandedOverlayInclude = $overlayInclude + "`n#ifdef _WIN32`n#include `"ariadne-window.h`"`n#endif"
$windowHandler = 'window_handle_bus_sync_handler(GstBus *bus, GstMessage *message, gpointer user_data) {'
$brandedWindowHandler = $windowHandler + "`n#ifdef _WIN32`n    ariadne_brand_video_window(message);`n#endif"
if (-not $rendererSource.Contains($brandedOverlayInclude)) {
    if (-not $rendererSource.Contains($overlayInclude) -or -not $rendererSource.Contains($windowHandler)) {
        throw 'UxPlay window handler differs from the pinned version.'
    }
    $rendererSource = $rendererSource.Replace($overlayInclude, $brandedOverlayInclude).Replace($windowHandler, $brandedWindowHandler)
}
if (-not $rendererSource.Contains($brandedWindowHandler)) { throw 'UxPlay window branding handler is missing.' }
foreach ($patch in @(
    @('void video_renderer_stop() {', "void video_renderer_stop() {`n#ifdef _WIN32`n    ariadne_video_stopped();`n#endif"),
    @('void video_renderer_set_device_model(const char *model, const char *name) {', "void video_renderer_set_device_model(const char *model, const char *name) {`n#ifdef _WIN32`n    ariadne_video_client(name);`n#endif"),
    @('void video_renderer_set_track_metadata(const char *title, const char *artist, const char *album) {', "void video_renderer_set_track_metadata(const char *title, const char *artist, const char *album) {`n#ifdef _WIN32`n    ariadne_video_metadata(title, artist);`n#endif")
)) {
    if (-not $rendererSource.Contains($patch[1])) {
        if (-not $rendererSource.Contains($patch[0])) { throw 'UxPlay renderer source differs from the pinned version.' }
        $rendererSource = $rendererSource.Replace($patch[0], $patch[1])
    }
}
[IO.File]::WriteAllText($rendererSourcePath, $rendererSource, [Text.UTF8Encoding]::new($false))
$rendererCmakePath = Join-Path $sourceDir 'renderers\CMakeLists.txt'
$rendererCmake = [IO.File]::ReadAllText($rendererCmakePath).Replace("`r`n", "`n")
$rendererTarget = 'target_link_libraries ( renderers PUBLIC airplay )'
$nativeRendererTarget = $rendererTarget + "`nif (WIN32)`n  target_sources(renderers PRIVATE ariadne-window.c)`n  target_link_libraries(renderers PUBLIC user32)`nendif()"
if (-not $rendererCmake.Contains($nativeRendererTarget)) {
    if (-not $rendererCmake.Contains($rendererTarget)) { throw 'UxPlay renderers target differs from the pinned version.' }
    [IO.File]::WriteAllText($rendererCmakePath, $rendererCmake.Replace($rendererTarget, $nativeRendererTarget), [Text.UTF8Encoding]::new($false))
}
$clientSourcePath = Join-Path $sourceDir 'uxplay.cpp'
$clientSource = [IO.File]::ReadAllText($clientSourcePath).Replace("`r`n", "`n")
$metadataStart = 'static void process_metadata('
$metadataEnd = 'static int parse_dmap_header('
$start = $clientSource.IndexOf($metadataStart)
if ($start -lt 0) { throw 'UxPlay metadata parser differs from the pinned version.' }
$end = $clientSource.IndexOf($metadataEnd, $start)
if ($start -lt 0 -or $end -lt 0) { throw 'UxPlay metadata parser differs from the pinned version.' }
# Track actual client metadata even when cover-art rendering is disabled.
$metadataParser = $clientSource.Substring($start, $end - $start).Replace('if (render_coverart) {', '{')
$clientSource = $clientSource.Substring(0, $start) + $metadataParser + $clientSource.Substring($end)
$metadataCallback = '// Update video renderer with track metadata for cover art display'
$nativeMetadataCallback = '// Update video renderer and native title with received track metadata'
if ($clientSource.Contains($metadataCallback)) {
    $clientSource = $clientSource.Replace($metadataCallback + "`n    if (render_coverart) {", $nativeMetadataCallback + "`n    {")
}
if (-not $clientSource.Contains($nativeMetadataCallback)) { throw 'UxPlay metadata callback is missing.' }
$metadataText = '    std::string metadata_text = "";'
$metadataReset = "    track_title.clear();`n    artist.clear();`n" + $metadataText
if (-not $clientSource.Contains($metadataReset)) {
    if (-not $clientSource.Contains($metadataText)) { throw 'UxPlay metadata text initialization is missing.' }
    $clientSource = $clientSource.Replace($metadataText, $metadataReset)
}
$metadataProcess = '        process_metadata(count, (const char *) dmap_tag, metadata, datalen, &metadata_text);'
$checkedMetadataProcess = "        if (datalen < 0 || datalen > buflen) { LOGE(`"invalid metadata item length`"); return; }`n" + $metadataProcess
if (-not $clientSource.Contains($checkedMetadataProcess)) {
    if (-not $clientSource.Contains($metadataProcess)) { throw 'UxPlay metadata item parser is missing.' }
    $clientSource = $clientSource.Replace($metadataProcess, $checkedMetadataProcess)
}
[IO.File]::WriteAllText($clientSourcePath, $clientSource, [Text.UTF8Encoding]::new($false))
$cmakeSourcePath = Join-Path $sourceDir 'CMakeLists.txt'
$cmakeSource = [IO.File]::ReadAllText($cmakeSourcePath).Replace("`r`n", "`n")
$executableTarget = 'add_executable( uxplay uxplay.cpp )'
$brandedExecutableTarget = $executableTarget + @'

if (WIN32)
  enable_language(RC)
  target_sources(uxplay PRIVATE ariadne.rc)
  target_include_directories(uxplay PRIVATE "${CMAKE_CURRENT_SOURCE_DIR}")
endif()
'@
$brandedExecutableTarget = $brandedExecutableTarget.Replace("`r`n", "`n")
if (-not $cmakeSource.Contains($brandedExecutableTarget)) {
    if (-not $cmakeSource.Contains($executableTarget)) { throw 'UxPlay executable target differs from the pinned version.' }
    [IO.File]::WriteAllText($cmakeSourcePath, $cmakeSource.Replace($executableTarget, $brandedExecutableTarget), [Text.UTF8Encoding]::new($false))
}

$savedEnvironment = @{}
foreach ($name in @('MSYSTEM', 'CHERE_INVOKING', 'ARIADNE_UXPLAY_SOURCE', 'ARIADNE_UXPLAY_OUTPUT', 'ARIADNE_UXPLAY_TEST', 'ARIADNE_UXPLAY_WINDOW_TEST')) {
    $savedEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
try {
    $env:MSYSTEM = 'UCRT64'
    $env:CHERE_INVOKING = '1'
    $env:ARIADNE_UXPLAY_SOURCE = $sourceDir
    $env:ARIADNE_UXPLAY_OUTPUT = $OutputDir
    $env:ARIADNE_UXPLAY_TEST = Join-Path $PSScriptRoot 'uxplay\test-mdns-windows.c'
    $env:ARIADNE_UXPLAY_WINDOW_TEST = Join-Path $PSScriptRoot 'uxplay\test-window-branding.c'
    $buildScript = @'
set -euo pipefail
pacman -Sy --noconfirm
pacman -S --needed --noconfirm mingw-w64-ucrt-x86_64-cmake mingw-w64-ucrt-x86_64-ninja mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-pkgconf mingw-w64-ucrt-x86_64-openssl mingw-w64-ucrt-x86_64-libplist mingw-w64-ucrt-x86_64-gstreamer mingw-w64-ucrt-x86_64-gst-plugins-base mingw-w64-ucrt-x86_64-gst-plugins-good mingw-w64-ucrt-x86_64-gst-plugins-bad mingw-w64-ucrt-x86_64-gst-libav
source_dir=$(cygpath -u "$ARIADNE_UXPLAY_SOURCE")
output_dir=$(cygpath -u "$ARIADNE_UXPLAY_OUTPUT")
cmake -S "$source_dir" -B "$source_dir/build" -G Ninja -DCMAKE_BUILD_TYPE=Release -DUSE_DNS_SD=OFF -DNO_MARCH_NATIVE=ON
cmake --build "$source_dir/build" --parallel 4
gcc -I "$source_dir" "$(cygpath -u "$ARIADNE_UXPLAY_TEST")" -o "$source_dir/build/test-mdns-windows.exe" -liphlpapi -lws2_32 -lpthread
"$source_dir/build/test-mdns-windows.exe"
windres -I "$source_dir" "$source_dir/ariadne.rc" -o "$source_dir/build/test-window-icon.o"
gcc '-DARIADNE_CONTROL_TITLE=L"Ariadne.ScreenInput.Test"' -I "$source_dir/renderers" $(pkg-config --cflags gstreamer-app-1.0 gstreamer-video-1.0) "$(cygpath -u "$ARIADNE_UXPLAY_WINDOW_TEST")" "$source_dir/renderers/ariadne-window.c" "$source_dir/build/test-window-icon.o" -o "$source_dir/build/test-window-branding.exe" $(pkg-config --libs gstreamer-app-1.0 gstreamer-video-1.0) -luser32
"$source_dir/build/test-window-branding.exe"
mkdir -p "$output_dir/lib/gstreamer-1.0" "$output_dir/libexec" "$output_dir/share"
cp "$source_dir/build/uxplay.exe" "$output_dir/"
cp /ucrt64/bin/gst-inspect-1.0.exe "$output_dir/"
cp -R /ucrt64/libexec/gstreamer-1.0 "$output_dir/libexec/"
for plugin in app coreelements typefindfunctions playback autodetect libav videoparsersbad videoconvertscale videofilter d3d11 audioconvert audioresample audioparsers volume level wasapi; do
    cp "/ucrt64/lib/gstreamer-1.0/libgst$plugin.dll" "$output_dir/lib/gstreamer-1.0/"
done
# Copy the imported DLL closure, rather than shipping unrelated compiler/AI runtimes.
declare -A dependencies_copied=()
copy_dependencies() {
    local binary="$1" dll
    while read -r dll; do
        if [[ -f "/ucrt64/bin/$dll" && ! ${dependencies_copied["$dll"]+present} ]]; then
            dependencies_copied["$dll"]=1
            cp "/ucrt64/bin/$dll" "$output_dir/"
            copy_dependencies "/ucrt64/bin/$dll"
        fi
    done < <(objdump -p "$binary" | awk '$1 == "DLL" && $2 == "Name:" {print $3}')
}
for binary in "$output_dir/"*.exe "$output_dir/lib/gstreamer-1.0/"*.dll "$output_dir/libexec/gstreamer-1.0/"*.exe; do
    copy_dependencies "$binary"
done
cp -R /ucrt64/share/licenses "$output_dir/share/"
pacman -Q > "$output_dir/share/msys2-packages.txt"
cp "$source_dir/LICENSE" "$output_dir/LICENSE-UxPlay"
cp "$source_dir/README.md" "$output_dir/README-UxPlay.md"
export PATH="$output_dir:/usr/bin:$(cygpath -u "$SYSTEMROOT")/System32"
export GST_PLUGIN_SYSTEM_PATH_1_0="$output_dir/lib/gstreamer-1.0"
export GST_PLUGIN_SCANNER_1_0="$output_dir/libexec/gstreamer-1.0/gst-plugin-scanner.exe"
export GST_REGISTRY_1_0="$output_dir/build-registry.bin"
"$output_dir/uxplay.exe" -h >/dev/null
for element in appsrc queue decodebin autoaudiosink autovideosink d3d11videosink wasapisink avdec_h264 avdec_aac avdec_alac h264parse videoconvert videoscale audioconvert audioresample volume level; do
    "$output_dir/gst-inspect-1.0.exe" "$element" >/dev/null
done
'@
    $buildScriptPath = Join-Path $BuildDir 'build-runtime.sh'
    [IO.File]::WriteAllText($buildScriptPath, $buildScript.Replace("`r`n", "`n"), [Text.UTF8Encoding]::new($false))
    $bashScriptPath = $buildScriptPath.Replace('\', '/')
    & $bash --login $bashScriptPath
    if ($LASTEXITCODE -ne 0) { throw 'UxPlay build or GStreamer runtime verification failed.' }
} finally {
    foreach ($name in $savedEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($name, $savedEnvironment[$name], 'Process')
    }
}

Remove-Item -LiteralPath (Join-Path $OutputDir 'build-registry.bin') -Force
# Keep the exact upstream source with the GPL runtime, including its notices.
Copy-Item -LiteralPath $sourceArchive -Destination (Join-Path $OutputDir 'UxPlay-source.zip') -Force
# Ship every modified source alongside the exact upstream archive.
foreach ($relativePath in @('lib\mdnsd\mdnsd.c', 'renderers\video_renderer.c', 'renderers\ariadne-window.h', 'renderers\ariadne-window.c', 'renderers\CMakeLists.txt', 'uxplay.cpp', 'CMakeLists.txt', 'ariadne.rc', 'ariadne.ico')) {
    $modifiedTarget = Join-Path (Join-Path $OutputDir 'modified-source') $relativePath
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $modifiedTarget) | Out-Null
    Copy-Item -LiteralPath (Join-Path $sourceDir $relativePath) -Destination $modifiedTarget -Force
}
@{
    sourceCommit = $sourceCommit
    source = "https://github.com/FDH2/UxPlay/tree/$sourceCommit"
    version = '1.74-dev'
    bundleRevision = $bundleRevision
    buildInputs = $buildInputs
    discovery = 'built-in mDNS'
    sourceChanges = 'modified-source/: Hyper-V LAN discovery; Ariadne adaptive video window, native controls, display power, stream events and client metadata'
} | ConvertTo-Json | Set-Content -LiteralPath $manifestPath -Encoding utf8
Write-Output "UxPlay runtime built and verified: $OutputDir"
