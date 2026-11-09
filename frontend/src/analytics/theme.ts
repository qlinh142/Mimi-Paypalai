/**
 * Theme AG Studio theo bộ màu Mimi (src/styles/tokens.css): mọi giá trị trỏ về biến CSS của app,
 * nên Studio, lưới và biểu đồ trông như một phần của Mimi chứ không phải một khung nhúng.
 * Bảng màu biểu đồ: xanh Mimi đi trước, cam chỉ dùng làm điểm nhấn (không dùng cho chữ).
 */
import { studioTheme } from "ag-studio";

const v = (name: string) => `var(--${name})`;
const line = (color: string) => ({ width: 1, color: v(color) });

/** 10 màu biểu đồ, phân biệt được cả khi in đen trắng nhờ độ sáng khác nhau. */
// Năm màu đầu là màu danh mục của bản thiết kế mới (Household, Groceries, Baby, Personal care, Bills).
export const MIMI_CHART = ["#2a5bd7", "#2f9e6e", "#e8873a", "#8b5cf6", "#d4538a", "#1595a8", "#b4530a", "#56618a", "#8bb2ff", "#f2c14e"];

const palette = Object.fromEntries(
  MIMI_CHART.flatMap((c, i) => [
    [`chartPaletteFills${i + 1}Color`, c],
    [`chartPaletteStrokes${i + 1}Color`, c],
  ]),
);

export const mimiStudioTheme = studioTheme.withParams({
  fontFamily: v("font"),
  fontSize: 14,
  textColor: "var(--ink, #1b2559)",
  foregroundColor: v("ink"),
  subtleTextColor: "var(--mute, #5d678f)",
  backgroundColor: v("paper"),
  borderColor: v("sky-line"),
  accentColor: v("blue"),
  iconColor: v("ink"),
  rowHoverColor: v("sky"),

  // Canvas là nền xanh nhạt của app; mỗi widget là một thẻ trắng bo 20, không viền, có bóng
  // (bóng đặt bằng CSS trong analytics.css vì Studio không có tham số bóng cho widget).
  studioWrapperBorder: false,
  studioWrapperBorderRadius: 0,
  studioWrapperSpacing: 0,
  studioWrapperBackgroundColor: "var(--bg, #f4f6ff)",
  studioCanvasBorder: false,
  studioCanvasBorderRadius: 0,
  studioCanvasShadow: false,
  studioCanvasBackgroundColor: "var(--bg, #f4f6ff)",
  studioCanvasPadding: 12,
  studioCanvasRowHeight: 18,
  studioCanvasFontFamily: v("font"),

  studioWidgetBackgroundColor: "var(--card, #ffffff)",
  studioWidgetBorder: false,
  studioWidgetBorderRadius: 20,
  studioWidgetPadding: 14,
  studioWidgetTitleFontFamily: v("font"),
  studioWidgetTitleFontSize: 15,
  studioWidgetTitleFontWeight: 700,
  studioWidgetTitleTextColor: "var(--ink, #1b2559)",
  studioWidgetSubtitleFontFamily: v("font"),
  studioWidgetSubtitleFontSize: 13,
  studioWidgetSubtitleTextColor: "var(--mute, #5d678f)",
  studioWidgetCaptionFontFamily: v("font"),
  studioWidgetCaptionFontSize: 13,
  studioWidgetCaptionTextColor: v("ink-soft"),
  studioWidgetResizeBorderColor: v("orange"),

  studioPanelContainerBackgroundColor: v("paper"),
  studioPanelContainerBorder: line("sky-line"),
  studioPanelContainerBorderRadius: 0,
  studioPanelDividerColor: v("sky-line"),
  studioPanelDividerActiveColor: v("blue"),
  studioPanelSectionBorderColor: v("sky-line"),
  studioPanelHeaderFontFamily: v("font"),
  studioPanelHeaderFontWeight: 700,
  studioPanelChevronButtonHoverBackgroundColor: v("sky"),
  studioPanelChevronButtonHoverColor: v("blue"),
  studioToggleButtonActiveBackgroundColor: v("sky"),
  studioToggleButtonActiveBorderColor: v("blue"),
  studioToggleButtonActiveColor: v("blue"),
  studioValueWidgetFontSize: 30,

  gridFontFamily: v("font"),
  gridFontSize: 14,
  gridCellTextColor: v("ink"),
  gridHeaderBackgroundColor: v("sky"),
  gridHeaderTextColor: v("ink"),
  gridHeaderFontWeight: 700,
  gridHeaderFontSize: 13,
  gridRowHeight: 44,
  gridRowBorder: line("sky-line"),
  gridHeaderRowBorder: line("sky-line"),
  gridPinnedRowBorder: line("sky-line"),
  gridRowHoverColor: v("sky"),
  gridWrapperBorder: false,

  chartFontFamily: v("font"),
  chartTextColor: v("ink"),
  chartSubtleTextColor: v("ink-soft"),
  chartAxisLineColor: v("sky-line"),
  chartGridLineColor: v("sky"),
  ...palette,
});
