//! A stand-in for gscouter-engine's WebAssembly module (its wasm/lib.rs),
//! with the same exports and calling convention, for testing how Gscouter
//! calls it (internal/engine). Build it with
//!
//!     cargo build --release --target wasm32-unknown-unknown
//!
//! and copy target/wasm32-unknown-unknown/release/fake.wasm to
//! ../fake.wasm.
//!
//! Iteration i of a config with members a, b, ... (its `<name> char`
//! lines) deals seed + i DPS, half by the first member and the rest split
//! among the others, over 6,000 frames. A config holding "refuse" is
//! refused, as one the engine lacks something for; one holding "fail at
//! N" fails at iteration N, as a fight that errors.

use std::fmt::Write;

#[unsafe(no_mangle)]
pub extern "C" fn gse_alloc(len: u32) -> *mut u8 {
    let mut v = Vec::<u8>::with_capacity(len as usize);
    let p = v.as_mut_ptr();
    std::mem::forget(v);
    p
}

/// # Safety
/// `ptr` and `len` are those of one `gse_alloc` call.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn gse_free(ptr: *mut u8, len: u32) {
    drop(unsafe { Vec::from_raw_parts(ptr, 0, len as usize) });
}

/// # Safety
/// `ptr` is a result this module returned.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn gse_free_result(ptr: *mut u8) {
    let len = u32::from_le_bytes(unsafe { [*ptr.add(4), *ptr.add(5), *ptr.add(6), *ptr.add(7)] }) as usize;
    drop(unsafe { Vec::from_raw_parts(ptr, 0, len + 8) });
}

/// # Safety
/// `cfg` and `len` are a buffer from `gse_alloc` holding UTF-8.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn gse_run_slice(cfg: *const u8, len: u32, _warmup: f64, seed: i64, first: u32, count: u32) -> *mut u8 {
    let src = unsafe { text(cfg, len) };
    if src.contains("refuse") {
        return result(1, "unsupported: refuse".into());
    }
    let fail = src.find("fail at ").map(|i| src[i + 8..].split_whitespace().next().unwrap_or("").parse::<u32>().unwrap_or(0));
    let names = names(src);
    let mut o = String::from("[");
    for j in 0..count {
        let i = first + j;
        if fail == Some(i) {
            return result(1, format!("iteration {i} (seed {}): failed", seed + i as i64));
        }
        if j > 0 {
            o.push(',');
        }
        let dps = (seed + i as i64) as f64;
        let _ = write!(o, "{{\"dps\":{dps},\"first\":{dps},\"second\":{dps},\"chars\":[");
        for (c, _) in names.iter().enumerate() {
            let d = if c == 0 { dps / 2.0 } else { dps / 2.0 / (names.len() - 1) as f64 };
            let _ = write!(o, "{}{d}", if c > 0 { "," } else { "" });
        }
        o.push_str("],\"waits\":[");
        for c in 0..names.len() {
            o.push_str(if c > 0 { ",{}" } else { "{}" });
        }
        o.push_str("],\"frames\":6000,\"names\":[");
        for (c, n) in names.iter().enumerate() {
            let _ = write!(o, "{}\"{n}\"", if c > 0 { "," } else { "" });
        }
        o.push_str("]}");
    }
    o.push(']');
    result(0, o)
}

/// # Safety
/// `cfg` and `len` are a buffer from `gse_alloc` holding UTF-8.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn gse_check(cfg: *const u8, len: u32) -> *mut u8 {
    let src = unsafe { text(cfg, len) };
    result(0, if src.contains("refuse") { "[\"unsupported: refuse\"]".into() } else { "[]".into() })
}

fn names(src: &str) -> Vec<&str> {
    src.lines().filter_map(|l| l.trim().split_once(" char ").map(|(n, _)| n)).collect()
}

unsafe fn text<'a>(ptr: *const u8, len: u32) -> &'a str {
    std::str::from_utf8(unsafe { std::slice::from_raw_parts(ptr, len as usize) }).unwrap_or("")
}

fn result(status: u32, body: String) -> *mut u8 {
    let mut v = Vec::with_capacity(8 + body.len());
    v.extend_from_slice(&status.to_le_bytes());
    v.extend_from_slice(&(body.len() as u32).to_le_bytes());
    v.extend_from_slice(body.as_bytes());
    let mut v = v.into_boxed_slice().into_vec();
    let p = v.as_mut_ptr();
    std::mem::forget(v);
    p
}
