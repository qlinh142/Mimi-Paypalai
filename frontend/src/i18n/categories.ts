import data from "./categories.json";

/**
 * Nhãn danh mục chi tiêu theo category_key của backend. Nguồn duy nhất là
 * backend/internal/taxonomy/categories.json; file categories.json cạnh đây là bản chép
 * (test Go TestCategoriesFrontendCopy bắt khi hai file lệch nhau).
 */
type Lang = "vi" | "en";
export type CategoryLabels = Record<string, string> & { khac: string };

export function categoryLabels(lang: Lang): CategoryLabels {
  const out: Record<string, string> = {};
  for (const c of [...data.categories, ...data.system]) out[c.key] = c.labels[lang];
  return out as CategoryLabels;
}
