use std::{
    io::{self, BufRead, BufReader, Write},
    path::PathBuf,
    process::Command,
    sync::atomic::{AtomicU64, Ordering},
    thread,
    time::{Duration, Instant},
};

#[cfg(windows)]
static CAPTURE_SEQUENCE: AtomicU64 = AtomicU64::new(0);

/// Starts Ariadne's packaged capture host and opens its native selection overlay.
///
/// The host owns the overlay, clipboard, annotation, save, and pin behavior. The
/// GPUI shell only brokers the short request/response over its named pipe.
pub(crate) fn launch_screenshot() {
    #[cfg(windows)]
    {
        if let Err(error) = launch_native_capture() {
            eprintln!("Ariadne screenshot failed: {error}");
        }
    }

    #[cfg(not(windows))]
    {
        let _ = Command::new("gnome-screenshot").spawn();
    }
}

#[cfg(windows)]
fn launch_native_capture() -> io::Result<()> {
    let host_path = capture_host_path()?;
    let sequence = CAPTURE_SEQUENCE.fetch_add(1, Ordering::Relaxed);
    let pipe_name = format!("ariadne-gpui-capture-{}-{sequence}", std::process::id());
    let pipe_path = format!(r"\\.\pipe\{pipe_name}");

    let mut host = Command::new(host_path)
        .arg("--pipe")
        .arg(&pipe_name)
        .spawn()?;

    let capture_result = send_request(
        &pipe_path,
        r#"{"command":"capture","directClipboardCopy":true}"#,
        Duration::from_secs(4),
    );

    // A pinned result owns a WPF PinWindow, so keep that host process alive. All
    // other results can shut their one-shot CaptureHost instance down safely.
    let keep_host = capture_result
        .as_ref()
        .map(|response| response.contains(r#""pinned":true"#))
        .unwrap_or(false);
    if keep_host {
        std::mem::forget(host);
    } else {
        let _ = send_request(
            &pipe_path,
            r#"{"command":"shutdown"}"#,
            Duration::from_secs(2),
        );
        wait_for_exit(&mut host);
    }

    let response = capture_result?;
    if response.contains(r#""ok":false"#) {
        return Err(io::Error::new(
            io::ErrorKind::Other,
            extract_response_message(&response),
        ));
    }
    Ok(())
}

#[cfg(windows)]
fn capture_host_path() -> io::Result<PathBuf> {
    const HOST_NAME: &str = "Ariadne.CaptureHost.exe";

    let mut candidates = Vec::new();
    if let Ok(path) = std::env::var("ARIADNE_CAPTURE_HOST") {
        let path = PathBuf::from(path);
        if !path.as_os_str().is_empty() {
            candidates.push(path);
        }
    }
    if let Ok(executable) = std::env::current_exe() {
        if let Some(directory) = executable.parent() {
            candidates.push(directory.join("native-capture").join(HOST_NAME));
        }
    }

    // Development runs commonly start from the repository root while the
    // packaged executable lives beside its `native-capture` directory.
    if let Ok(current) = std::env::current_dir() {
        candidates.push(current.join("bin").join("native-capture").join(HOST_NAME));
        candidates.push(
            current
                .join("build")
                .join("native-capture-single-compressed")
                .join(HOST_NAME),
        );
        let mut ancestor = current.as_path();
        for _ in 0..6 {
            let Some(parent) = ancestor.parent() else {
                break;
            };
            candidates.push(parent.join("bin").join("native-capture").join(HOST_NAME));
            ancestor = parent;
        }
    }

    candidates.dedup();
    candidates
        .into_iter()
        .find(|candidate| candidate.is_file())
        .ok_or_else(|| {
            io::Error::new(
                io::ErrorKind::NotFound,
                "Ariadne.CaptureHost.exe was not found beside the executable or in the development native-capture directories",
            )
        })
}

#[cfg(windows)]
fn send_request(pipe_path: &str, request: &str, timeout: Duration) -> io::Result<String> {
    let deadline = Instant::now() + timeout;
    loop {
        match std::fs::OpenOptions::new()
            .read(true)
            .write(true)
            .open(pipe_path)
        {
            Ok(mut pipe) => {
                pipe.write_all(request.as_bytes())?;
                pipe.write_all(b"\n")?;
                pipe.flush()?;
                let mut response = String::new();
                BufReader::new(pipe).read_line(&mut response)?;
                return Ok(response);
            }
            Err(error) if Instant::now() >= deadline => return Err(error),
            Err(_) => thread::sleep(Duration::from_millis(25)),
        }
    }
}

#[cfg(windows)]
fn wait_for_exit(host: &mut std::process::Child) {
    let deadline = Instant::now() + Duration::from_secs(2);
    loop {
        match host.try_wait() {
            Ok(Some(_)) | Err(_) => return,
            Ok(None) if Instant::now() < deadline => thread::sleep(Duration::from_millis(25)),
            Ok(None) => {
                let _ = host.kill();
                let _ = host.wait();
                return;
            }
        }
    }
}

#[cfg(windows)]
fn extract_response_message(response: &str) -> String {
    response
        .split_once(r#""message":"#)
        .and_then(|(_, value)| value.split('"').next())
        .filter(|message| !message.is_empty())
        .unwrap_or("native capture host returned an error")
        .to_owned()
}
