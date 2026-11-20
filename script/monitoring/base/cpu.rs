use std::collections::HashMap;
use std::thread;
use std::time::{Duration, Instant};

pub fn run() {
    let mut previous = HashMap::<u32, (f64, Instant)>::new();
    let cores = thread::available_parallelism()
        .map(|count| count.get() as f64)
        .unwrap_or(1.0);

    println!("Monitoring Leoxy CPU usage. Press Ctrl+C to stop.");

    loop {
        let samples = read_samples();
        let mut found = false;

        for (pid, name, cpu_seconds) in samples {
            found = true;
            let now = Instant::now();
            if let Some((old_cpu, old_time)) = previous.insert(pid, (cpu_seconds, now)) {
                let elapsed = now.duration_since(old_time).as_secs_f64();
                if elapsed > 0.0 {
                    let percent = ((cpu_seconds - old_cpu).max(0.0) / elapsed / cores) * 100.0;
                    println!("PID: {pid}, Process: {name}, CPU: {percent:.2}%");
                }
            } else {
                println!("PID: {pid}, Process: {name}, CPU: sampling...");
            }
        }

        if !found {
            previous.clear();
            println!("Leoxy process not found");
        }

        thread::sleep(Duration::from_secs(1));
    }
}

#[cfg(target_os = "windows")]
fn read_samples() -> Vec<(u32, String, f64)> {
    use std::process::Command;

    let output = Command::new("powershell.exe")
        .args([
            "-NoProfile",
            "-NonInteractive",
            "-Command",
            "Get-Process -Name leoxy -ErrorAction SilentlyContinue | ForEach-Object { \"$($_.Id),$($_.ProcessName),$($_.TotalProcessorTime.TotalSeconds)\" }",
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
                "Failed to read Leoxy CPU: {}",
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
fn read_samples() -> Vec<(u32, String, f64)> {
    use std::fs;

    let ticks_per_second = 100.0;
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
        let Ok(stat) = fs::read_to_string(entry.path().join("stat")) else {
            continue;
        };
        let Some((_, fields)) = stat.rsplit_once(") ") else {
            continue;
        };
        let fields: Vec<&str> = fields.split_whitespace().collect();
        if fields.len() <= 12 {
            continue;
        }
        let name = fs::read_to_string(entry.path().join("comm"))
            .unwrap_or_default()
            .trim()
            .to_owned();
        if !name.eq_ignore_ascii_case("leoxy") {
            continue;
        }
        let (Ok(user_ticks), Ok(system_ticks)) =
            (fields[11].parse::<f64>(), fields[12].parse::<f64>())
        else {
            continue;
        };
        samples.push((pid, name, (user_ticks + system_ticks) / ticks_per_second));
    }

    samples
}
