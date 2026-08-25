pub const BACKGROUND: u32 = 0xf4f5f7;
pub const SURFACE: u32 = 0xffffff;
pub const SURFACE_SUBTLE: u32 = 0xf8f9fa;
pub const SURFACE_SELECTED: u32 = 0xeeeff2;
pub const FOREGROUND: u32 = 0x1f2933;
pub const MUTED: u32 = 0x6b7280;
pub const BORDER: u32 = 0xd3d8df;
pub const BORDER_STRONG: u32 = 0xaeb7c2;
pub const PRIMARY: u32 = 0x1f2933;
pub const SUCCESS: u32 = 0x2f6f4e;
pub const WARNING: u32 = 0x8b5e25;
pub const DANGER: u32 = 0x9a3b3b;

pub fn hex(value: u32) -> gpui::Rgba {
    gpui::rgb(value)
}
