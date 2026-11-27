fn main() {
    let q = "What is the diameter of Hubble's main mirror in meters?";
    let sent = "The main mirror has a diameter of 2.4 meters.";
    println!("requested: {:?}", ganesh_core::accept::requested_unit(q));
}
