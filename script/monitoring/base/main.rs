#[path = "cpu.rs"]
mod cpu;
#[path = "ram.rs"]
mod ram;

fn main() {
    let cpu_monitor = std::thread::spawn(cpu::run);
    let ram_monitor = std::thread::spawn(ram::run);

    let _ = cpu_monitor.join();
    let _ = ram_monitor.join();
}
