use std::thread;
use std::time::Duration;
use sysinfo::{ProcessRefreshKind, ProcessesToUpdate, System};

pub fn run() {
    let Ok(mut system) = System::new() else {
        eprintln!("Failed to initialize system monitor");
        return;
    };
    let refresh_kind = ProcessRefreshKind::nothing().with_memory();

    println!("Monitoring Leoxy RAM usage. Press Ctrl+C to stop.");

    loop {
        system.refresh_processes_specifics(ProcessesToUpdate::All, true, refresh_kind);
        let mut found = false;

        for (pid, process) in system.processes() {
            let process_name = process.name().to_string_lossy();
            let process_name = process_name
                .strip_suffix(".exe")
                .unwrap_or(process_name.as_ref());
            if process_name.eq_ignore_ascii_case("leoxy") {
                found = true;
                println!(
                    "PID: {pid}, Process: {}, RAM: {:.2} MB",
                    process.name().to_string_lossy(),
                    process.memory() as f64 / 1_048_576.0
                );
            }
        }

        if !found {
            println!("Leoxy process not found");
        }

        thread::sleep(Duration::from_secs(1));
    }
}
