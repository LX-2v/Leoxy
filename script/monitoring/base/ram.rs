use std::thread;
use std::time::Duration;

pub fn run() {
    println!("Monitoring Leoxy RAM usage. Press Ctrl+C to stop.");

    loop {
        let samples = read_samples();
        if samples.is_empty() {
            println!("Leoxy process not found");
        }
        for (pid, name, bytes) in samples {
            println!(
                "PID: {pid}, Process: {name}, RAM: {:.2} MB",
                bytes as f64 / 1_048_576.0
            );
        }
        thread::sleep(Duration::from_secs(1));
    }
}

#[cfg(target_os = "windows")]
fn read_samples() -> Vec<(u32, String, u64)> {
    use std::process::Command;

    let output = Command::new("powershell.exe")
        .args([
            "-NoProfile",
            "-NonInteractive",
            "-Command",
            "Get-Process -Name leoxy -ErrorAction SilentlyContinue | ForEach-Object { \"$($_.Id),$($_.ProcessName),$($_.WorkingSet64)\" }",
        ])
        .output();

    match output {
        Ok(output) if output.status.success() => String::from_utf8_lossy(&output.stdout)
            .lines()
            .filter_map(|line| {
                let mut fields = line.trim().splitn(3, ',');
                Some((
                    fields.next()?.parse().ok()?,
                    fields.next()?.to_owned(),
                    fields.next()?.parse().ok()?,
                ))
            })
            .collect(),
        Ok(output) => {
            eprintln!(
                "Failed to read Leoxy RAM: {}",
                String::from_utf8_lossy(&output.stderr).trim()
            );
            Vec::new()
        }
        Err(error) => {
            eprintln!("Failed to run PowerShell: {error}");
            Vec::new()
        }
    }
}

#[cfg(target_os = "linux")]
fn read_samples() -> Vec<(u32, String, u64)> {
    use std::fs;

    let mut samples = Vec::new();
    let Ok(entries) = fs::read_dir("/proc") else {
        return samples;
    };

    for entry in entries.flatten() {
        let Some(pid) = entry
            .file_name()
            .to_str()
            .and_then(|name| name.parse::<u32>().ok())
        else {
            continue;
        };
        let name = fs::read_to_string(entry.path().join("comm"))
            .unwrap_or_default()
            .trim()
            .to_owned();
        if !name.eq_ignore_ascii_case("leoxy") {
            continue;
        }
        let Ok(status) = fs::read_to_string(entry.path().join("status")) else {
            continue;
        };
        let Some(rss_kb) = status.lines().find_map(|line| {
            line.strip_prefix("VmRSS:")?
                .split_whitespace()
                .next()?
                .parse::<u64>()
                .ok()
        }) else {
            continue;
        };
        samples.push((pid, name, rss_kb * 1024));
    }

    samples
}
